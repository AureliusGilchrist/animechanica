package manga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"seanime/internal/api/anilist"
	"seanime/internal/hook"
	"seanime/internal/platforms/anilist_platform"
	"seanime/internal/platforms/platform"
	"seanime/internal/util/filecache"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

type (
	// Entry is fetched when the user goes to the manga entry page.
	Entry struct {
		MediaId       int                `json:"mediaId"`
		Media         *anilist.BaseManga `json:"media"`
		EntryListData *EntryListData     `json:"listData,omitempty"`
	}

	EntryListData struct {
		Progress    int                      `json:"progress,omitempty"`
		Score       float64                  `json:"score,omitempty"`
		Status      *anilist.MediaListStatus `json:"status,omitempty"`
		Repeat      int                      `json:"repeat,omitempty"`
		StartedAt   string                   `json:"startedAt,omitempty"`
		CompletedAt string                   `json:"completedAt,omitempty"`
	}
)

type (
	// NewEntryOptions is the options for creating a new manga entry.
	NewEntryOptions struct {
		MediaId         int
		Logger          *zerolog.Logger
		FileCacher      *filecache.Cacher
		MangaCollection *anilist.MangaCollection
		Platform        platform.Platform
	}
)

// NewEntry creates a new manga entry.
func NewEntry(ctx context.Context, opts *NewEntryOptions) (entry *Entry, err error) {
	entry = &Entry{
		MediaId: opts.MediaId,
	}

	reqEvent := new(MangaEntryRequestedEvent)
	reqEvent.MediaId = opts.MediaId
	reqEvent.MangaCollection = opts.MangaCollection
	reqEvent.Entry = entry

	err = hook.GlobalHookManager.OnMangaEntryRequested().Trigger(reqEvent)
	if err != nil {
		return nil, err
	}
	opts.MediaId = reqEvent.MediaId                 // Override the media ID
	opts.MangaCollection = reqEvent.MangaCollection // Override the manga collection
	entry = reqEvent.Entry                          // Override the entry

	if reqEvent.DefaultPrevented {
		mangaEvent := new(MangaEntryEvent)
		mangaEvent.Entry = reqEvent.Entry
		err = hook.GlobalHookManager.OnMangaEntry().Trigger(mangaEvent)
		if err != nil {
			return nil, err
		}

		if mangaEvent.Entry == nil {
			return nil, errors.New("no entry was returned")
		}
		return mangaEvent.Entry, nil
	}

	anilistEntry, found := opts.MangaCollection.GetListEntryFromMangaId(opts.MediaId)

	// If the entry is not found, we fetch the manga metadata.
	if !found {
		// Handle negative media IDs (En Masse downloads) by fetching from Kitsu
		if opts.MediaId < 0 {
			media, err := fetchKitsuMangaMetadata(ctx, opts.MediaId, opts.Logger)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch Kitsu metadata for media ID %d: %w", opts.MediaId, err)
			}
			entry.Media = media
		} else {
			// Positive media IDs use AniList
			media, err := opts.Platform.GetManga(ctx, opts.MediaId)
			if err != nil {
				return nil, err
			}
			entry.Media = media
		}

	} else {
		// If the entry is found, we use the entry from the collection.
		mangaEvent := new(anilist_platform.GetMangaEvent)
		mangaEvent.Manga = anilistEntry.GetMedia()
		err := hook.GlobalHookManager.OnGetManga().Trigger(mangaEvent)
		if err != nil {
			return nil, err
		}
		entry.Media = mangaEvent.Manga
		entry.EntryListData = &EntryListData{
			Progress:    *anilistEntry.Progress,
			Score:       *anilistEntry.Score,
			Status:      anilistEntry.Status,
			Repeat:      anilistEntry.GetRepeatSafe(),
			StartedAt:   anilist.FuzzyDateToString(anilistEntry.StartedAt),
			CompletedAt: anilist.FuzzyDateToString(anilistEntry.CompletedAt),
		}
	}

	mangaEvent := new(MangaEntryEvent)
	mangaEvent.Entry = entry
	err = hook.GlobalHookManager.OnMangaEntry().Trigger(mangaEvent)
	if err != nil {
		return nil, err
	}

	return mangaEvent.Entry, nil
}

// KitsuMangaData represents the structure of Kitsu API manga response
type KitsuMangaData struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			CanonicalTitle string `json:"canonicalTitle"`
			Titles         struct {
				En   string `json:"en"`
				EnJp string `json:"en_jp"`
				JaJp string `json:"ja_jp"`
			} `json:"titles"`
			Synopsis     string `json:"synopsis"`
			Description  string `json:"description"`
			ChapterCount *int   `json:"chapterCount"`
			VolumeCount  *int   `json:"volumeCount"`
			Status       string `json:"status"`
			StartDate    string `json:"startDate"`
			EndDate      string `json:"endDate"`
			PosterImage  struct {
				Tiny     string `json:"tiny"`
				Small    string `json:"small"`
				Medium   string `json:"medium"`
				Large    string `json:"large"`
				Original string `json:"original"`
			} `json:"posterImage"`
			CoverImage *struct {
				Tiny     string `json:"tiny"`
				Small    string `json:"small"`
				Large    string `json:"large"`
				Original string `json:"original"`
			} `json:"coverImage"`
			AverageRating string   `json:"averageRating"`
			PopularityRank *int    `json:"popularityRank"`
			RatingRank     *int    `json:"ratingRank"`
			AgeRating      string   `json:"ageRating"`
			AgeRatingGuide string   `json:"ageRatingGuide"`
		} `json:"attributes"`
	} `json:"data"`
}

// fetchKitsuMangaMetadata fetches manga metadata from Kitsu API for negative media IDs
func fetchKitsuMangaMetadata(ctx context.Context, mediaId int, logger *zerolog.Logger) (*anilist.BaseManga, error) {
	if mediaId >= 0 {
		return nil, fmt.Errorf("mediaId must be negative for Kitsu lookup, got %d", mediaId)
	}

	// Convert negative media ID back to positive Kitsu ID
	kitsuID := -mediaId

	logger.Debug().Int("mediaId", mediaId).Int("kitsuID", kitsuID).Msg("Fetching manga metadata from Kitsu")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Kitsu API endpoint for specific manga
	kitsuURL := fmt.Sprintf("https://kitsu.io/api/edge/manga/%d", kitsuID)

	req, err := http.NewRequestWithContext(ctx, "GET", kitsuURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Content-Type", "application/vnd.api+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Kitsu API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Kitsu API returned status %d for manga ID %d", resp.StatusCode, kitsuID)
	}

	var kitsuData KitsuMangaData
	if err := json.NewDecoder(resp.Body).Decode(&kitsuData); err != nil {
		return nil, fmt.Errorf("failed to decode Kitsu response: %w", err)
	}

	// Convert Kitsu data to AniList BaseManga format
	baseManga := &anilist.BaseManga{
		ID: mediaId, // Use the negative media ID
		Title: &anilist.BaseManga_Title{
			UserPreferred: &kitsuData.Data.Attributes.CanonicalTitle,
			Romaji:        &kitsuData.Data.Attributes.CanonicalTitle,
			English:       &kitsuData.Data.Attributes.Titles.En,
			Native:        &kitsuData.Data.Attributes.Titles.JaJp,
		},
		Description: &kitsuData.Data.Attributes.Description,
		Status:      convertKitsuStatus(kitsuData.Data.Attributes.Status),
		Format:      func() *anilist.MediaFormat { f := anilist.MediaFormatManga; return &f }(),
		CoverImage: &anilist.BaseManga_CoverImage{
			Large:  &kitsuData.Data.Attributes.PosterImage.Large,
			Medium: &kitsuData.Data.Attributes.PosterImage.Medium,
		},
		BannerImage: nil, // Kitsu doesn't have banner images
		Genres:      []*string{}, // Would need additional API call to get genres
	}

	// Set chapter count if available
	if kitsuData.Data.Attributes.ChapterCount != nil {
		baseManga.Chapters = kitsuData.Data.Attributes.ChapterCount
	}

	// Set volume count if available
	if kitsuData.Data.Attributes.VolumeCount != nil {
		baseManga.Volumes = kitsuData.Data.Attributes.VolumeCount
	}

	// Parse average rating
	if kitsuData.Data.Attributes.AverageRating != "" {
		if rating, err := strconv.ParseFloat(kitsuData.Data.Attributes.AverageRating, 64); err == nil {
			// Convert from 0-100 scale to 0-10 scale
			anilistRating := int(rating / 10)
			baseManga.MeanScore = &anilistRating
		}
	}

	// Note: BaseManga doesn't have a Popularity field, skipping popularity rank

	logger.Debug().Int("mediaId", mediaId).Str("title", kitsuData.Data.Attributes.CanonicalTitle).Msg("Successfully fetched Kitsu metadata")

	return baseManga, nil
}

// convertKitsuStatus converts Kitsu status to AniList MediaStatus
func convertKitsuStatus(kitsuStatus string) *anilist.MediaStatus {
	switch kitsuStatus {
	case "current":
		s := anilist.MediaStatusReleasing
		return &s
	case "finished":
		s := anilist.MediaStatusFinished
		return &s
	case "tba":
		s := anilist.MediaStatusNotYetReleased
		return &s
	case "unreleased":
		s := anilist.MediaStatusNotYetReleased
		return &s
	case "upcoming":
		s := anilist.MediaStatusNotYetReleased
		return &s
	default:
		s := anilist.MediaStatusReleasing // Default fallback
		return &s
	}
}

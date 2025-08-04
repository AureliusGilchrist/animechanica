package handlers

import (
	"errors"
	"fmt"
	"seanime/internal/events"
	"seanime/internal/manga"
	"sort"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// HandleSaveMangaLocally
//
//	@summary downloads all chapters of a manga to local storage in order.
//	@desc This downloads all chapters from lowest to highest chapter number, queued 3 at a time with provider-aware rate limiting.
//	@desc Downloads are saved to /aeternae/library/manga/seanime/{MANGANAME}
//	@route /api/v1/manga/save-locally [POST]
//	@returns bool
func (h *Handler) HandleSaveMangaLocally(c echo.Context) error {
	// Start the manga downloader immediately on button press
	h.App.MangaDownloader.Start()
	// Extract session ID from context
	sessionID := c.Get("Seanime-Client-Id").(string)

	type body struct {
		MediaId  int    `json:"mediaId"`
		Provider string `json:"provider"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	if b.MediaId == 0 {
		return h.RespondWithError(c, errors.New("mediaId is required"))
	}

	if b.Provider == "" {
		return h.RespondWithError(c, errors.New("provider is required"))
	}

	h.App.WSEventManager.SendEvent(events.InfoToast, "Starting manga download...")

	// Get manga titles from AniList platform
	var titles []*string
	baseManga, found := baseMangaCache.Get(b.MediaId)
	if !found {
		var err error
		baseManga, err = h.App.AnilistPlatform.GetManga(c.Request().Context(), b.MediaId)
		if err != nil {
			return h.RespondWithError(c, err)
		}
		titles = baseManga.GetAllTitles()
		baseMangaCache.SetT(b.MediaId, baseManga, 24*time.Hour)
	} else {
		titles = baseManga.GetAllTitles()
	}

	// Get the manga title for directory naming
	mangaTitle, err := h.getMangaTitleForDownload(c, b.MediaId)
	if err != nil {
		return h.RespondWithError(c, fmt.Errorf("failed to get manga title: %w", err))
	}

	// Get all chapters for this manga from the provider
	chapterContainer, err := h.App.MangaRepository.GetMangaChapterContainer(&manga.GetMangaChapterContainerOptions{
		Provider: b.Provider,
		MediaId:  b.MediaId,
		Titles:   titles,
		Year:     baseManga.GetStartYearSafe(),
	})
	if err != nil {
		return h.RespondWithError(c, fmt.Errorf("failed to get chapters: %w", err))
	}

	if len(chapterContainer.Chapters) == 0 {
		return h.RespondWithError(c, errors.New("no chapters found for this manga"))
	}

	// Sort chapters by chapter number (lowest to highest)
	chapters := make([]*manga.ChapterForDownload, 0, len(chapterContainer.Chapters))
	for _, chapter := range chapterContainer.Chapters {
		chapterNum, err := strconv.ParseFloat(chapter.Chapter, 64)
		if err != nil {
			// If we can't parse the chapter number, use the index
			chapterNum = float64(chapter.Index)
		}
		chapters = append(chapters, &manga.ChapterForDownload{
			ChapterDetails: chapter,
			ChapterNumber:  chapterNum,
		})
	}

	// Sort by chapter number
	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].ChapterNumber < chapters[j].ChapterNumber
	})

	h.App.WSEventManager.SendEvent(events.InfoToast, fmt.Sprintf("Found %d chapters to download", len(chapters)))

	// Add chapters to download queue in bulk
	go h.downloadMangaChaptersBulk(sessionID, b.MediaId, b.Provider, chapters, mangaTitle)

	return h.RespondWithData(c, true)
}

// downloadMangaChaptersBulk downloads all chapters by queuing them with minimal delays to prevent rate limiting
func (h *Handler) downloadMangaChaptersBulk(sessionID string, mediaId int, provider string, chapters []*manga.ChapterForDownload, mangaTitle string) {
	h.App.WSEventManager.SendEvent(events.InfoToast, fmt.Sprintf("Starting bulk download of %d chapters for %s", len(chapters), mangaTitle))

	// Get provider-specific rate limit for chapter queuing (to prevent 429 errors)
	queueRateLimit := h.getProviderChapterQueueRateLimit(provider)

	// Queue all chapters with minimal delays to prevent rate limiting
	for i, chapter := range chapters {
		// Add chapter to download queue with retry mechanism
		// Always queue chapters instead of starting immediately to prevent multithreading issues
		err := h.retryChapterQueue(manga.DownloadChapterOptions{
			Provider:   provider,
			MediaId:    mediaId,
			ChapterId:  chapter.ChapterDetails.ID,
			StartNow:   false, // Always queue, never start immediately to avoid glitches
			MangaTitle: mangaTitle,
		}, chapter.ChapterDetails.Chapter)

		if err != nil {
			h.App.WSEventManager.SendEvent(events.ErrorToast, fmt.Sprintf("Failed to queue chapter %s after retries: %v", chapter.ChapterDetails.Chapter, err))
			continue
		}

		// Apply minimal rate limiting between chapter queuing to prevent 429 errors
		if i < len(chapters)-1 {
			time.Sleep(queueRateLimit)
		}

		// Send progress update every 10 chapters or at the end
		if (i+1)%10 == 0 || i == len(chapters)-1 {
			h.App.WSEventManager.SendEvent(events.InfoToast, fmt.Sprintf("Queued %d/%d chapters for %s", i+1, len(chapters), mangaTitle))
		}
	}

	h.App.WSEventManager.SendEvent(events.SuccessToast, fmt.Sprintf("All %d chapters queued for download: %s", len(chapters), mangaTitle))

	// Always start the download queue to trigger downloads immediately
	h.App.MangaDownloader.RunChapterDownloadQueue()
}

// getProviderChapterQueueRateLimit returns the rate limiting delay for queuing chapters from different providers
// This prevents 429 "Too Many Requests" errors when fetching chapter page lists
func (h *Handler) getProviderChapterQueueRateLimit(provider string) time.Duration {
	switch provider {
	case "weebcentral":
		return 500 * time.Millisecond // WeebCentral has strict rate limits
	case "mangadex":
		return 300 * time.Millisecond // MangaDex has moderate rate limits
	case "comick":
		return 200 * time.Millisecond
	case "mangafire":
		return 250 * time.Millisecond
	case "manganato":
		return 150 * time.Millisecond
	case "mangapill":
		return 100 * time.Millisecond
	default:
		return 200 * time.Millisecond // Default rate limit
	}
}

// getMangaTitleForDownload gets the manga title for directory naming
func (h *Handler) getMangaTitleForDownload(c echo.Context, mediaId int) (string, error) {
	// Get session ID from context
	sessionID, ok := c.Get("Seanime-Client-Id").(string)
	if !ok {
		return fmt.Sprintf("Manga_%d", mediaId), nil
	}

	// Get manga collection to find the title
	collection, err := h.App.GetMangaCollectionForSession(sessionID, false)
	if err == nil && collection != nil {
		for _, list := range collection.MediaListCollection.Lists {
			for _, entry := range list.Entries {
				if entry.Media.ID == mediaId {
					if entry.Media.Title.English != nil && *entry.Media.Title.English != "" {
						return *entry.Media.Title.English, nil
					}
					if entry.Media.Title.Romaji != nil {
						return *entry.Media.Title.Romaji, nil
					}
				}
			}
		}
	}

	// Fallback to using media ID
	return fmt.Sprintf("Manga_%d", mediaId), nil
}

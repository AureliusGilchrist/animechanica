package anime

import (
	"seanime/internal/database/models"
	"seanime/internal/library/anime"
	"time"
)

type (
	// Entry represents an anime entry with metadata and download information
	Entry struct {
		ID                int                    `json:"id"`
		MediaID           int                    `json:"mediaId"`
		Title             string                 `json:"title"`
		EnglishTitle      string                 `json:"englishTitle"`
		RomajiTitle       string                 `json:"romajiTitle"`
		Status            string                 `json:"status"`
		Format            string                 `json:"format"`
		Episodes          int                    `json:"episodes"`
		Duration          int                    `json:"duration"`
		StartDate         *time.Time             `json:"startDate"`
		EndDate           *time.Time             `json:"endDate"`
		Season            string                 `json:"season"`
		SeasonYear        int                    `json:"seasonYear"`
		Genres            []string               `json:"genres"`
		Tags              []string               `json:"tags"`
		Studios           []string               `json:"studios"`
		CoverImage        string                 `json:"coverImage"`
		BannerImage       string                 `json:"bannerImage"`
		Description       string                 `json:"description"`
		AverageScore      int                    `json:"averageScore"`
		Popularity        int                    `json:"popularity"`
		IsAdult           bool                   `json:"isAdult"`
		CountryOfOrigin   string                 `json:"countryOfOrigin"`
		Source            string                 `json:"source"`
		Synonyms          []string               `json:"synonyms"`
		Relations         []*anime.BaseAnime     `json:"relations"`
		Recommendations   []*anime.BaseAnime     `json:"recommendations"`
		NextAiringEpisode *anime.NextAiringEpisode `json:"nextAiringEpisode"`
		
		// Download-specific fields
		DownloadInfo      *DownloadInfo          `json:"downloadInfo"`
		LibraryData       *LibraryData           `json:"libraryData"`
		
		CreatedAt         time.Time              `json:"createdAt"`
		UpdatedAt         time.Time              `json:"updatedAt"`
	}

	// DownloadInfo contains information about anime downloads
	DownloadInfo struct {
		Provider          string                 `json:"provider"`
		Quality           string                 `json:"quality"`
		Language          string                 `json:"language"`
		SubtitleLanguage  string                 `json:"subtitleLanguage"`
		TotalEpisodes     int                    `json:"totalEpisodes"`
		DownloadedEpisodes int                   `json:"downloadedEpisodes"`
		DownloadStatus    string                 `json:"downloadStatus"`
		DownloadProgress  float64                `json:"downloadProgress"`
		LastDownloadDate  *time.Time             `json:"lastDownloadDate"`
		DownloadPath      string                 `json:"downloadPath"`
		FileSize          int64                  `json:"fileSize"`
		Episodes          []*EpisodeDownloadInfo `json:"episodes"`
	}

	// EpisodeDownloadInfo contains information about individual episode downloads
	EpisodeDownloadInfo struct {
		EpisodeNumber     int                    `json:"episodeNumber"`
		Title             string                 `json:"title"`
		Duration          int                    `json:"duration"`
		AirDate           *time.Time             `json:"airDate"`
		Downloaded        bool                   `json:"downloaded"`
		DownloadDate      *time.Time             `json:"downloadDate"`
		FilePath          string                 `json:"filePath"`
		FileSize          int64                  `json:"fileSize"`
		Quality           string                 `json:"quality"`
		SubtitlePath      string                 `json:"subtitlePath"`
		ThumbnailPath     string                 `json:"thumbnailPath"`
		WatchProgress     float64                `json:"watchProgress"`
		Watched           bool                   `json:"watched"`
		WatchedDate       *time.Time             `json:"watchedDate"`
	}

	// LibraryData contains library-specific information
	LibraryData struct {
		Progress          int                    `json:"progress"`
		Score             float64                `json:"score"`
		Status            string                 `json:"status"`
		StartedAt         *time.Time             `json:"startedAt"`
		CompletedAt       *time.Time             `json:"completedAt"`
		Repeat            int                    `json:"repeat"`
		Priority          int                    `json:"priority"`
		Private           bool                   `json:"private"`
		Notes             string                 `json:"notes"`
		HiddenFromStatusLists bool               `json:"hiddenFromStatusLists"`
		CustomLists       []string               `json:"customLists"`
		AdvancedScores    map[string]float64     `json:"advancedScores"`
	}
)

// NewEntry creates a new anime entry from base anime data
func NewEntry(baseAnime *anime.BaseAnime) *Entry {
	entry := &Entry{
		MediaID:           baseAnime.ID,
		Title:             baseAnime.GetTitleSafe(),
		EnglishTitle:      baseAnime.GetTitle().GetEnglish(),
		RomajiTitle:       baseAnime.GetTitle().GetRomaji(),
		Status:            string(baseAnime.GetStatus()),
		Format:            string(baseAnime.GetFormat()),
		Episodes:          baseAnime.GetEpisodes(),
		Duration:          baseAnime.GetDuration(),
		Season:            string(baseAnime.GetSeason()),
		SeasonYear:        baseAnime.GetSeasonYear(),
		Genres:            baseAnime.GetGenres(),
		Studios:           extractStudioNames(baseAnime.GetStudios()),
		CoverImage:        baseAnime.GetCoverImage().GetLarge(),
		BannerImage:       baseAnime.GetBannerImage(),
		Description:       baseAnime.GetDescription(),
		AverageScore:      baseAnime.GetAverageScore(),
		Popularity:        baseAnime.GetPopularity(),
		IsAdult:           baseAnime.GetIsAdult(),
		CountryOfOrigin:   string(baseAnime.GetCountryOfOrigin()),
		Source:            string(baseAnime.GetSource()),
		Synonyms:          baseAnime.GetSynonyms(),
		NextAiringEpisode: baseAnime.GetNextAiringEpisode(),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if baseAnime.GetStartDate() != nil {
		startDate := baseAnime.GetStartDate().ToTime()
		entry.StartDate = &startDate
	}

	if baseAnime.GetEndDate() != nil {
		endDate := baseAnime.GetEndDate().ToTime()
		entry.EndDate = &endDate
	}

	// Extract tags
	if baseAnime.GetTags() != nil {
		tags := make([]string, 0)
		for _, tag := range baseAnime.GetTags() {
			if tag.GetName() != "" {
				tags = append(tags, tag.GetName())
			}
		}
		entry.Tags = tags
	}

	return entry
}

// extractStudioNames extracts studio names from studio objects
func extractStudioNames(studios []*anime.BaseStudio) []string {
	names := make([]string, 0, len(studios))
	for _, studio := range studios {
		if studio != nil && studio.GetName() != "" {
			names = append(names, studio.GetName())
		}
	}
	return names
}

// UpdateFromModel updates the entry from a database model
func (e *Entry) UpdateFromModel(model *models.AnimeEntry) {
	if model == nil {
		return
	}

	e.ID = model.ID
	e.CreatedAt = model.CreatedAt
	e.UpdatedAt = model.UpdatedAt

	// Update library data if present
	if model.LibraryData != nil {
		e.LibraryData = &LibraryData{
			Progress:              model.LibraryData.Progress,
			Score:                 model.LibraryData.Score,
			Status:                model.LibraryData.Status,
			StartedAt:             model.LibraryData.StartedAt,
			CompletedAt:           model.LibraryData.CompletedAt,
			Repeat:                model.LibraryData.Repeat,
			Priority:              model.LibraryData.Priority,
			Private:               model.LibraryData.Private,
			Notes:                 model.LibraryData.Notes,
			HiddenFromStatusLists: model.LibraryData.HiddenFromStatusLists,
		}
	}
}

// ToModel converts the entry to a database model
func (e *Entry) ToModel() *models.AnimeEntry {
	model := &models.AnimeEntry{
		ID:        e.ID,
		MediaID:   e.MediaID,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}

	if e.LibraryData != nil {
		model.LibraryData = &models.AnimeLibraryData{
			Progress:              e.LibraryData.Progress,
			Score:                 e.LibraryData.Score,
			Status:                e.LibraryData.Status,
			StartedAt:             e.LibraryData.StartedAt,
			CompletedAt:           e.LibraryData.CompletedAt,
			Repeat:                e.LibraryData.Repeat,
			Priority:              e.LibraryData.Priority,
			Private:               e.LibraryData.Private,
			Notes:                 e.LibraryData.Notes,
			HiddenFromStatusLists: e.LibraryData.HiddenFromStatusLists,
		}
	}

	return model
}

// GetDownloadProgress returns the download progress as a percentage
func (e *Entry) GetDownloadProgress() float64 {
	if e.DownloadInfo == nil || e.DownloadInfo.TotalEpisodes == 0 {
		return 0.0
	}
	return float64(e.DownloadInfo.DownloadedEpisodes) / float64(e.DownloadInfo.TotalEpisodes) * 100.0
}

// IsCompletelyDownloaded returns true if all episodes are downloaded
func (e *Entry) IsCompletelyDownloaded() bool {
	if e.DownloadInfo == nil {
		return false
	}
	return e.DownloadInfo.DownloadedEpisodes >= e.DownloadInfo.TotalEpisodes
}

// GetNextEpisodeToDownload returns the next episode number that needs to be downloaded
func (e *Entry) GetNextEpisodeToDownload() int {
	if e.DownloadInfo == nil || len(e.DownloadInfo.Episodes) == 0 {
		return 1
	}

	for _, episode := range e.DownloadInfo.Episodes {
		if !episode.Downloaded {
			return episode.EpisodeNumber
		}
	}

	return e.DownloadInfo.TotalEpisodes + 1
}

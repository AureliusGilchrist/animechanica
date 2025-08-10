package anime

import (
	"seanime/internal/events"
	"time"
)

// Hook events for anime system

type (
	// AnimeDownloadStartedEvent is emitted when an anime download starts
	AnimeDownloadStartedEvent struct {
		MediaID       int    `json:"mediaId"`
		Title         string `json:"title"`
		EpisodeNumber int    `json:"episodeNumber"`
		Provider      string `json:"provider"`
		Quality       string `json:"quality"`
		Language      string `json:"language"`
		StartTime     time.Time `json:"startTime"`
	}

	// AnimeDownloadProgressEvent is emitted during anime downloads
	AnimeDownloadProgressEvent struct {
		MediaID        int     `json:"mediaId"`
		Title          string  `json:"title"`
		EpisodeNumber  int     `json:"episodeNumber"`
		Progress       float64 `json:"progress"`
		Speed          int64   `json:"speed"`
		ETA            string  `json:"eta"`
		DownloadedSize int64   `json:"downloadedSize"`
		TotalSize      int64   `json:"totalSize"`
	}

	// AnimeDownloadCompletedEvent is emitted when an anime download completes
	AnimeDownloadCompletedEvent struct {
		MediaID       int       `json:"mediaId"`
		Title         string    `json:"title"`
		EpisodeNumber int       `json:"episodeNumber"`
		FilePath      string    `json:"filePath"`
		FileSize      int64     `json:"fileSize"`
		Quality       string    `json:"quality"`
		Language      string    `json:"language"`
		Success       bool      `json:"success"`
		Error         string    `json:"error"`
		CompletedAt   time.Time `json:"completedAt"`
	}

	// AnimeBatchDownloadStartedEvent is emitted when a batch download starts
	AnimeBatchDownloadStartedEvent struct {
		JobID      string            `json:"jobId"`
		Type       BatchDownloadType `json:"type"`
		TotalItems int               `json:"totalItems"`
		Settings   *BatchDownloadSettings `json:"settings"`
		StartTime  time.Time         `json:"startTime"`
	}

	// AnimeBatchDownloadProgressEvent is emitted during batch downloads
	AnimeBatchDownloadProgressEvent struct {
		JobID          string  `json:"jobId"`
		Progress       float64 `json:"progress"`
		CompletedItems int     `json:"completedItems"`
		FailedItems    int     `json:"failedItems"`
		TotalItems     int     `json:"totalItems"`
		CurrentItem    string  `json:"currentItem"`
	}

	// AnimeBatchDownloadCompletedEvent is emitted when a batch download completes
	AnimeBatchDownloadCompletedEvent struct {
		JobID          string    `json:"jobId"`
		Success        bool      `json:"success"`
		CompletedItems int       `json:"completedItems"`
		FailedItems    int       `json:"failedItems"`
		TotalItems     int       `json:"totalItems"`
		Error          string    `json:"error"`
		CompletedAt    time.Time `json:"completedAt"`
		Duration       string    `json:"duration"`
	}

	// AnimeLibraryUpdatedEvent is emitted when anime library is updated
	AnimeLibraryUpdatedEvent struct {
		MediaID     int       `json:"mediaId"`
		Title       string    `json:"title"`
		Action      string    `json:"action"` // "added", "updated", "removed"
		Episodes    int       `json:"episodes"`
		UpdatedAt   time.Time `json:"updatedAt"`
	}

	// AnimeEpisodeWatchedEvent is emitted when an episode is watched
	AnimeEpisodeWatchedEvent struct {
		MediaID       int       `json:"mediaId"`
		Title         string    `json:"title"`
		EpisodeNumber int       `json:"episodeNumber"`
		Progress      float64   `json:"progress"`
		WatchedAt     time.Time `json:"watchedAt"`
	}

	// AnimeProviderStatusEvent is emitted when provider status changes
	AnimeProviderStatusEvent struct {
		Provider  string    `json:"provider"`
		Status    string    `json:"status"` // "online", "offline", "error"
		Message   string    `json:"message"`
		UpdatedAt time.Time `json:"updatedAt"`
	}

	// AnimeMetadataUpdatedEvent is emitted when anime metadata is updated
	AnimeMetadataUpdatedEvent struct {
		MediaID     int       `json:"mediaId"`
		Title       string    `json:"title"`
		Provider    string    `json:"provider"`
		UpdatedAt   time.Time `json:"updatedAt"`
	}
)

// Event type constants for anime system
const (
	EventAnimeDownloadStarted         = "anime_download_started"
	EventAnimeDownloadProgress        = "anime_download_progress"
	EventAnimeDownloadCompleted       = "anime_download_completed"
	EventAnimeBatchDownloadStarted    = "anime_batch_download_started"
	EventAnimeBatchDownloadProgress   = "anime_batch_download_progress"
	EventAnimeBatchDownloadCompleted  = "anime_batch_download_completed"
	EventAnimeLibraryUpdated          = "anime_library_updated"
	EventAnimeEpisodeWatched          = "anime_episode_watched"
	EventAnimeProviderStatus          = "anime_provider_status"
	EventAnimeMetadataUpdated         = "anime_metadata_updated"
)

// EmitAnimeDownloadStarted emits an anime download started event
func (r *Repository) EmitAnimeDownloadStarted(mediaID int, title string, episodeNumber int, provider, quality, language string) {
	if r.wsEventManager == nil {
		return
	}

	event := &AnimeDownloadStartedEvent{
		MediaID:       mediaID,
		Title:         title,
		EpisodeNumber: episodeNumber,
		Provider:      provider,
		Quality:       quality,
		Language:      language,
		StartTime:     time.Now(),
	}

	r.wsEventManager.SendEvent(events.AnimeDownloadStarted, event)
}

// EmitAnimeDownloadCompleted emits an anime download completed event
func (r *Repository) EmitAnimeDownloadCompleted(mediaID int, title string, episodeNumber int, filePath string, fileSize int64, quality, language string, success bool, errorMsg string) {
	if r.wsEventManager == nil {
		return
	}

	event := &AnimeDownloadCompletedEvent{
		MediaID:       mediaID,
		Title:         title,
		EpisodeNumber: episodeNumber,
		FilePath:      filePath,
		FileSize:      fileSize,
		Quality:       quality,
		Language:      language,
		Success:       success,
		Error:         errorMsg,
		CompletedAt:   time.Now(),
	}

	r.wsEventManager.SendEvent(events.AnimeDownloadComplete, event)
}

// EmitAnimeLibraryUpdated emits an anime library updated event
func (r *Repository) EmitAnimeLibraryUpdated(mediaID int, title string, action string, episodes int) {
	if r.wsEventManager == nil {
		return
	}

	event := &AnimeLibraryUpdatedEvent{
		MediaID:   mediaID,
		Title:     title,
		Action:    action,
		Episodes:  episodes,
		UpdatedAt: time.Now(),
	}

	r.wsEventManager.SendEvent(events.AnimeLibraryUpdated, event)
}

// EmitAnimeEpisodeWatched emits an anime episode watched event
func (r *Repository) EmitAnimeEpisodeWatched(mediaID int, title string, episodeNumber int, progress float64) {
	if r.wsEventManager == nil {
		return
	}

	event := &AnimeEpisodeWatchedEvent{
		MediaID:       mediaID,
		Title:         title,
		EpisodeNumber: episodeNumber,
		Progress:      progress,
		WatchedAt:     time.Now(),
	}

	r.wsEventManager.SendEvent(events.AnimeEpisodeWatched, event)
}

// EmitAnimeProviderStatus emits an anime provider status event
func (r *Repository) EmitAnimeProviderStatus(provider, status, message string) {
	if r.wsEventManager == nil {
		return
	}

	event := &AnimeProviderStatusEvent{
		Provider:  provider,
		Status:    status,
		Message:   message,
		UpdatedAt: time.Now(),
	}

	r.wsEventManager.SendEvent(events.AnimeProviderStatus, event)
}

// EmitAnimeMetadataUpdated emits an anime metadata updated event
func (r *Repository) EmitAnimeMetadataUpdated(mediaID int, title, provider string) {
	if r.wsEventManager == nil {
		return
	}

	event := &AnimeMetadataUpdatedEvent{
		MediaID:   mediaID,
		Title:     title,
		Provider:  provider,
		UpdatedAt: time.Now(),
	}

	r.wsEventManager.SendEvent(events.AnimeMetadataUpdated, event)
}

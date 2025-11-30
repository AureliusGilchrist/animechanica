//go:build disabled
// +build disabled

package anime

import (
	"context"
	"fmt"
	"seanime/internal/events"
	"seanime/internal/torrents"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type (
	// AllAnimeDownloader handles downloading ALL anime from the database
	AllAnimeDownloader struct {
		enMasseDownloader *EnMasseDownloader
		repository        *Repository
		torrentClient     torrents.TorrentClientInterface
		logger            *zerolog.Logger
		wsEventManager    events.WSEventManagerInterface
		activeJob         *AllAnimeDownloadJob
		mu                sync.RWMutex
	}

	// AllAnimeDownloadJob represents the massive download operation
	AllAnimeDownloadJob struct {
		ID             string                    `json:"id"`
		Status         AllAnimeDownloadStatus    `json:"status"`
		TotalAnime     int                       `json:"totalAnime"`
		CompletedAnime int                       `json:"completedAnime"`
		FailedAnime    int                       `json:"failedAnime"`
		ActiveBatches  int                       `json:"activeBatches"`
		Progress       float64                   `json:"progress"`
		StartTime      time.Time                 `json:"startTime"`
		EndTime        *time.Time                `json:"endTime"`
		Settings       *AllAnimeDownloadSettings `json:"settings"`
		Statistics     *AllAnimeDownloadStats    `json:"statistics"`
		Error          string                    `json:"error"`
		ctx            context.Context
		cancel         context.CancelFunc
	}

	// AllAnimeDownloadSettings contains settings for the all-anime download
	AllAnimeDownloadSettings struct {
		PreferDualAudio      bool     `json:"preferDualAudio"`
		PreferBluray         bool     `json:"preferBluray"`
		PreferHighestRes     bool     `json:"preferHighestRes"`
		MinSeeders           int      `json:"minSeeders"`
		MaxConcurrentBatches int      `json:"maxConcurrentBatches"`
		SkipOVA              bool     `json:"skipOva"`
		SkipSpecials         bool     `json:"skipSpecials"`
		MinYear              int      `json:"minYear"`
		MaxYear              int      `json:"maxYear"`
		IncludeGenres        []string `json:"includeGenres"`
		ExcludeGenres        []string `json:"excludeGenres"`
	}

	// AllAnimeDownloadStats contains download statistics
	AllAnimeDownloadStats struct {
		TotalSizeGB       float64 `json:"totalSizeGb"`
		DownloadedSizeGB  float64 `json:"downloadedSizeGb"`
		AverageSpeed      int64   `json:"averageSpeed"`
		EstimatedTimeLeft string  `json:"estimatedTimeLeft"`
		DualAudioCount    int     `json:"dualAudioCount"`
		BlurayCount       int     `json:"blurayCount"`
		HighResCount      int     `json:"highResCount"`
		TorrentsAdded     int     `json:"torrentsAdded"`
		QbittorrentActive int     `json:"qbittorrentActive"`
	}

	// AllAnimeDownloadStatus represents the status of the all-anime download
	AllAnimeDownloadStatus string
)

const (
	AllAnimeDownloadStatusPending   AllAnimeDownloadStatus = "pending"
	AllAnimeDownloadStatusRunning   AllAnimeDownloadStatus = "running"
	AllAnimeDownloadStatusCompleted AllAnimeDownloadStatus = "completed"
	AllAnimeDownloadStatusFailed    AllAnimeDownloadStatus = "failed"
	AllAnimeDownloadStatusCancelled AllAnimeDownloadStatus = "cancelled"
	AllAnimeDownloadStatusPaused    AllAnimeDownloadStatus = "paused"
)

// NewAllAnimeDownloader creates a new all-anime downloader
func NewAllAnimeDownloader(
	enMasseDownloader *EnMasseDownloader,
	repository *Repository,
	torrentClient torrents.TorrentClientInterface,
	logger *zerolog.Logger,
	wsEventManager events.WSEventManagerInterface,
) *AllAnimeDownloader {
	return &AllAnimeDownloader{
		enMasseDownloader: enMasseDownloader,
		repository:        repository,
		torrentClient:     torrentClient,
		logger:            logger,
		wsEventManager:    wsEventManager,
	}
}

// StartAllAnimeDownload starts downloading ALL anime from the database
func (aad *AllAnimeDownloader) StartAllAnimeDownload(ctx context.Context, settings *AllAnimeDownloadSettings) (*AllAnimeDownloadJob, error) {
	aad.mu.Lock()
	defer aad.mu.Unlock()

	if aad.activeJob != nil && aad.activeJob.Status == AllAnimeDownloadStatusRunning {
		return aad.activeJob, fmt.Errorf("all-anime download already in progress")
	}

	// Load anime database if not already loaded
	if aad.enMasseDownloader.animeDatabase == nil {
		return nil, fmt.Errorf("anime database not loaded - please load anime-offline-database-minified.json first")
	}

	// Apply default settings
	if settings == nil {
		settings = &AllAnimeDownloadSettings{
			PreferDualAudio:      true,
			PreferBluray:         true,
			PreferHighestRes:     true,
			MinSeeders:           5,
			MaxConcurrentBatches: 10, // 10 concurrent anime downloads
			SkipOVA:              false,
			SkipSpecials:         false,
			MinYear:              1990,
			MaxYear:              2024,
		}
	}

	// Filter anime entries based on settings
	filteredAnime := aad.filterAnimeEntries(aad.enMasseDownloader.animeDatabase.Data, settings)

	if len(filteredAnime) == 0 {
		return nil, fmt.Errorf("no anime found matching the specified criteria")
	}

	// Create job context
	jobCtx, cancel := context.WithCancel(ctx)

	// Create the massive download job
	jobID := fmt.Sprintf("all_anime_%d", time.Now().Unix())
	job := &AllAnimeDownloadJob{
		ID:             jobID,
		Status:         AllAnimeDownloadStatusPending,
		TotalAnime:     len(filteredAnime),
		CompletedAnime: 0,
		FailedAnime:    0,
		ActiveBatches:  0,
		Progress:       0.0,
		StartTime:      time.Now(),
		Settings:       settings,
		Statistics:     &AllAnimeDownloadStats{},
		ctx:            jobCtx,
		cancel:         cancel,
	}

	aad.activeJob = job

	// Start the massive download process
	go aad.processAllAnimeDownload(job, filteredAnime)

	aad.logger.Info().
		Str("jobId", jobID).
		Int("totalAnime", len(filteredAnime)).
		Bool("preferDualAudio", settings.PreferDualAudio).
		Bool("preferBluray", settings.PreferBluray).
		Bool("preferHighestRes", settings.PreferHighestRes).
		Int("maxConcurrent", settings.MaxConcurrentBatches).
		Msg("anime: Started ALL anime download - this will take a VERY long time!")

	return job, nil
}

// processAllAnimeDownload processes the massive download operation
func (aad *AllAnimeDownloader) processAllAnimeDownload(job *AllAnimeDownloadJob, animeList []AnimeOfflineEntry) {
	defer func() {
		aad.mu.Lock()
		aad.activeJob = nil
		aad.mu.Unlock()
	}()

	job.Status = AllAnimeDownloadStatusRunning
	aad.emitAllAnimeProgress(job)

	// Create semaphore for concurrent batch control
	semaphore := make(chan struct{}, job.Settings.MaxConcurrentBatches)
	var wg sync.WaitGroup

	// Process each anime as a separate batch
	for i, animeEntry := range animeList {
		select {
		case <-job.ctx.Done():
			job.Status = AllAnimeDownloadStatusCancelled
			aad.emitAllAnimeProgress(job)
			return
		default:
		}

		wg.Add(1)
		go func(entry AnimeOfflineEntry, index int) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			aad.processIndividualAnime(job, entry, index)
		}(animeEntry, i)

		// Small delay to prevent overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all anime to complete
	wg.Wait()

	// Finalize the job
	now := time.Now()
	job.EndTime = &now
	job.Progress = 100.0

	if job.FailedAnime == 0 {
		job.Status = AllAnimeDownloadStatusCompleted
	} else if job.CompletedAnime == 0 {
		job.Status = AllAnimeDownloadStatusFailed
		job.Error = "All anime downloads failed"
	} else {
		job.Status = AllAnimeDownloadStatusCompleted
		job.Error = fmt.Sprintf("%d anime failed to download", job.FailedAnime)
	}

	aad.emitAllAnimeProgress(job)
	aad.emitAllAnimeComplete(job)

	duration := time.Since(job.StartTime)
	aad.logger.Info().
		Str("jobId", job.ID).
		Int("completed", job.CompletedAnime).
		Int("failed", job.FailedAnime).
		Str("duration", duration.String()).
		Msg("anime: ALL anime download completed!")
}

// processIndividualAnime processes a single anime using the batch system
func (aad *AllAnimeDownloader) processIndividualAnime(job *AllAnimeDownloadJob, entry AnimeOfflineEntry, index int) {
	// Increment active batches
	aad.mu.Lock()
	job.ActiveBatches++
	aad.mu.Unlock()

	defer func() {
		aad.mu.Lock()
		job.ActiveBatches--
		aad.mu.Unlock()
	}()

	aad.logger.Debug().
		Str("title", entry.Title).
		Int("year", entry.Year).
		Int("episodes", entry.Episodes).
		Msg("anime: Starting individual anime download")

	// Create batch download settings optimized for this anime
	batchSettings := &BatchDownloadSettings{
		Quality:             aad.determineOptimalQuality(job.Settings),
		Language:            aad.determineOptimalLanguage(job.Settings),
		PreferredFormats:    aad.determineOptimalFormats(job.Settings),
		MinSeeders:          job.Settings.MinSeeders,
		MaxFileSize:         50 * 1024 * 1024 * 1024, // 50GB max per anime
		IncludeOVA:          !job.Settings.SkipOVA,
		IncludeSpecials:     !job.Settings.SkipSpecials,
		AutoSelectBest:      true,
		ConcurrentDownloads: 1,    // One anime at a time per batch
		AutoLink:            true, // Always auto-link
	}

	// Create criteria for this specific anime
	criteria := map[string]interface{}{
		"titles": []string{entry.Title},
	}

	// Start the batch download for this anime
	batchJob, err := aad.enMasseDownloader.StartBatchDownload(
		job.ctx,
		BatchDownloadTypeAnime,
		criteria,
		batchSettings,
	)

	if err != nil {
		aad.logger.Error().
			Err(err).
			Str("title", entry.Title).
			Msg("anime: Failed to start batch download for anime")

		aad.mu.Lock()
		job.FailedAnime++
		aad.mu.Unlock()
		return
	}

	// Monitor the batch job until completion
	aad.monitorBatchJob(job, batchJob, entry)
}

// monitorBatchJob monitors a single anime batch job
func (aad *AllAnimeDownloader) monitorBatchJob(allJob *AllAnimeDownloadJob, batchJob *BatchDownloadJob, entry AnimeOfflineEntry) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-allJob.ctx.Done():
			return
		case <-ticker.C:
			// Check if batch job is still active
			currentBatch, exists := aad.enMasseDownloader.GetBatchJob(batchJob.ID)
			if !exists {
				// Job completed or failed
				aad.mu.Lock()
				if batchJob.Status == BatchDownloadStatusCompleted {
					allJob.CompletedAnime++
					aad.updateStatistics(allJob, batchJob)
				} else {
					allJob.FailedAnime++
				}
				allJob.Progress = float64(allJob.CompletedAnime+allJob.FailedAnime) / float64(allJob.TotalAnime) * 100
				aad.mu.Unlock()

				aad.emitAllAnimeProgress(allJob)
				return
			}

			// Update progress
			batchJob = currentBatch
		}
	}
}

// updateStatistics updates download statistics
func (aad *AllAnimeDownloader) updateStatistics(allJob *AllAnimeDownloadJob, batchJob *BatchDownloadJob) {
	stats := allJob.Statistics

	for _, item := range batchJob.Items {
		if item.Status == BatchItemStatusCompleted && item.TorrentInfo != nil {
			stats.TotalSizeGB += float64(item.TorrentInfo.Size) / (1024 * 1024 * 1024)
			stats.TorrentsAdded++

			// Count quality preferences achieved
			if item.TorrentInfo.Language != "" &&
				(item.TorrentInfo.Language == "Dual Audio" ||
					item.TorrentInfo.Language == "dual") {
				stats.DualAudioCount++
			}

			if item.TorrentInfo.Format != "" &&
				(item.TorrentInfo.Format == "BD" ||
					item.TorrentInfo.Format == "Bluray" ||
					item.TorrentInfo.Format == "BDRip") {
				stats.BlurayCount++
			}

			if item.TorrentInfo.Quality != "" &&
				(item.TorrentInfo.Quality == "1080p" ||
					item.TorrentInfo.Quality == "2160p" ||
					item.TorrentInfo.Quality == "4K") {
				stats.HighResCount++
			}
		}
	}
}

// filterAnimeEntries filters anime based on settings
func (aad *AllAnimeDownloader) filterAnimeEntries(entries []AnimeOfflineEntry, settings *AllAnimeDownloadSettings) []AnimeOfflineEntry {
	var filtered []AnimeOfflineEntry

	for _, entry := range entries {
		// Skip if year is outside range
		if entry.Year < settings.MinYear || entry.Year > settings.MaxYear {
			continue
		}

		// Skip OVA if requested
		if settings.SkipOVA && entry.Type == "OVA" {
			continue
		}

		// Skip specials if requested
		if settings.SkipSpecials && (entry.Type == "Special" || entry.Type == "Music") {
			continue
		}

		// Check include genres
		if len(settings.IncludeGenres) > 0 {
			hasIncludedGenre := false
			for _, includeGenre := range settings.IncludeGenres {
				for _, tag := range entry.Tags {
					if tag == includeGenre {
						hasIncludedGenre = true
						break
					}
				}
				if hasIncludedGenre {
					break
				}
			}
			if !hasIncludedGenre {
				continue
			}
		}

		// Check exclude genres
		if len(settings.ExcludeGenres) > 0 {
			hasExcludedGenre := false
			for _, excludeGenre := range settings.ExcludeGenres {
				for _, tag := range entry.Tags {
					if tag == excludeGenre {
						hasExcludedGenre = true
						break
					}
				}
				if hasExcludedGenre {
					break
				}
			}
			if hasExcludedGenre {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	return filtered
}

// determineOptimalQuality determines the best quality based on settings
func (aad *AllAnimeDownloader) determineOptimalQuality(settings *AllAnimeDownloadSettings) string {
	if settings.PreferHighestRes {
		return "1080p" // Could be 4K/2160p if available
	}
	return "720p"
}

// determineOptimalLanguage determines the best language based on settings
func (aad *AllAnimeDownloader) determineOptimalLanguage(settings *AllAnimeDownloadSettings) string {
	if settings.PreferDualAudio {
		return "dual"
	}
	return "japanese"
}

// determineOptimalFormats determines the best formats based on settings
func (aad *AllAnimeDownloader) determineOptimalFormats(settings *AllAnimeDownloadSettings) []string {
	if settings.PreferBluray {
		return []string{"BD", "Bluray", "BDRip"}
	}
	return []string{"WEB", "TV"}
}

// emitAllAnimeProgress emits progress update
func (aad *AllAnimeDownloader) emitAllAnimeProgress(job *AllAnimeDownloadJob) {
	if aad.wsEventManager == nil {
		return
	}

	aad.wsEventManager.SendEvent(events.AllAnimeDownloadProgress, job)
}

// emitAllAnimeComplete emits completion event
func (aad *AllAnimeDownloader) emitAllAnimeComplete(job *AllAnimeDownloadJob) {
	if aad.wsEventManager == nil {
		return
	}

	aad.wsEventManager.SendEvent(events.AllAnimeDownloadComplete, job)
}

// GetActiveJob returns the current active all-anime job
func (aad *AllAnimeDownloader) GetActiveJob() *AllAnimeDownloadJob {
	aad.mu.RLock()
	defer aad.mu.RUnlock()
	return aad.activeJob
}

// CancelAllAnimeDownload cancels the all-anime download
func (aad *AllAnimeDownloader) CancelAllAnimeDownload() error {
	aad.mu.Lock()
	defer aad.mu.Unlock()

	if aad.activeJob == nil {
		return fmt.Errorf("no active all-anime download to cancel")
	}

	aad.activeJob.cancel()
	aad.activeJob.Status = AllAnimeDownloadStatusCancelled
	now := time.Now()
	aad.activeJob.EndTime = &now

	aad.emitAllAnimeProgress(aad.activeJob)

	aad.logger.Info().
		Str("jobId", aad.activeJob.ID).
		Msg("anime: ALL anime download cancelled")

	return nil
}

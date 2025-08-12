package manga

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"seanime/internal/api/anilist"
	"seanime/internal/database/db"
	"seanime/internal/events"
	hibikemanga "seanime/internal/extension/hibike/manga"
	manga_providers "seanime/internal/manga/providers"

	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type (
	// EnMasseDownloader handles bulk downloading of manga series from weebcentral catalogue
	EnMasseDownloader struct {
		logger         *zerolog.Logger
		wsEventManager events.WSEventManagerInterface
		database       *db.Database
		repository     *Repository
		downloader     *Downloader
		anilistClient  anilist.AnilistClient

		// State management
		isRunning bool
		mu        sync.RWMutex
		stopCh    chan struct{}

		// Progress tracking
		totalSeries     int
		processedSeries int
		currentSeries   string
		startTime       time.Time

		// Configuration
		cataloguePath        string
		delayBetweenChapters time.Duration
		delayBetweenSeries   time.Duration
		progressFilePath     string
		runningStateFilePath string
	}

	// WeebCentralCatalogueEntry represents a manga series in the weebcentral catalogue
	WeebCentralCatalogueEntry struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}

	// EnMasseDownloaderStatus represents the current status of the en masse downloader
	EnMasseDownloaderStatus struct {
		IsRunning              bool      `json:"isRunning"`
		TotalSeries            int       `json:"totalSeries"`
		ProcessedSeries        int       `json:"processedSeries"`
		CurrentSeries          string    `json:"currentSeries"`
		StartTime              time.Time `json:"startTime"`
		Progress               float64   `json:"progress"`
		EstimatedTimeRemaining string    `json:"estimatedTimeRemaining"`
	}

	// EnMasseDownloaderProgress represents the saved progress state
	EnMasseDownloaderProgress struct {
		ProcessedSeriesIDs []string  `json:"processedSeriesIds"`
		TotalSeries        int       `json:"totalSeries"`
		ProcessedSeries    int       `json:"processedSeries"`
		StartTime          time.Time `json:"startTime"`
		LastUpdated        time.Time `json:"lastUpdated"`
	}

	// enMasseDownloaderRunningState persists whether the downloader was running
	enMasseDownloaderRunningState struct {
		Running     bool      `json:"running"`
		LastUpdated time.Time `json:"lastUpdated"`
	}

	// EnMasseDownloaderOptions contains options for creating a new EnMasseDownloader
	EnMasseDownloaderOptions struct {
		Logger         *zerolog.Logger
		WSEventManager events.WSEventManagerInterface
		Database       *db.Database
		Repository     *Repository
		Downloader     *Downloader
		AnilistClient  anilist.AnilistClient
		CataloguePath  string
	}
)

// NewEnMasseDownloader creates a new EnMasseDownloader instance
func NewEnMasseDownloader(opts *EnMasseDownloaderOptions) *EnMasseDownloader {
	// Create progress file path in the same directory as the catalogue
	catalogueDir := filepath.Dir(opts.CataloguePath)
	progressFilePath := filepath.Join(catalogueDir, "en_masse_downloader_progress.json")
	runningStateFilePath := filepath.Join(catalogueDir, "en_masse_downloader_running.json")

	return &EnMasseDownloader{
		logger:               opts.Logger,
		wsEventManager:       opts.WSEventManager,
		database:             opts.Database,
		repository:           opts.Repository,
		anilistClient:        opts.AnilistClient,
		downloader:           opts.Downloader,
		cataloguePath:        opts.CataloguePath,
		progressFilePath:     progressFilePath,
		runningStateFilePath: runningStateFilePath,
		delayBetweenChapters: 2 * time.Second,  // Increased from 100ms to 2s to reduce rate limiting
		delayBetweenSeries:   10 * time.Second, // Increased from 3s to 10s to reduce rate limiting
		stopCh:               make(chan struct{}),
	}
}

// saveProgress saves the current progress to a file
func (emd *EnMasseDownloader) saveProgress(processedSeriesIDs []string) error {
	progress := &EnMasseDownloaderProgress{
		ProcessedSeriesIDs: processedSeriesIDs,
		TotalSeries:        emd.totalSeries,
		ProcessedSeries:    emd.processedSeries,
		StartTime:          emd.startTime,
		LastUpdated:        time.Now(),
	}

	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	if err := os.WriteFile(emd.progressFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}

	emd.logger.Debug().Str("progressFile", emd.progressFilePath).Int("processedSeries", emd.processedSeries).Msg("en_masse_downloader: Saved progress")
	return nil
}

// setRunningState persists the running state to disk
func (emd *EnMasseDownloader) setRunningState(running bool) {
	state := &enMasseDownloaderRunningState{
		Running:     running,
		LastUpdated: time.Now(),
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		emd.logger.Warn().Err(err).Msg("en_masse_downloader: Failed to marshal running state")
		return
	}
	if err := os.WriteFile(emd.runningStateFilePath, data, 0644); err != nil {
		emd.logger.Warn().Err(err).Msg("en_masse_downloader: Failed to write running state file")
		return
	}
	emd.logger.Debug().Str("stateFile", emd.runningStateFilePath).Bool("running", running).Msg("en_masse_downloader: Saved running state")
}

// WasRunning returns true if the previous process indicated it was running
func (emd *EnMasseDownloader) WasRunning() bool {
	data, err := os.ReadFile(emd.runningStateFilePath)
	if err != nil {
		return false
	}
	var state enMasseDownloaderRunningState
	if err := json.Unmarshal(data, &state); err != nil {
		return false
	}
	return state.Running
}

// loadProgress loads the progress from a file
func (emd *EnMasseDownloader) loadProgress() (*EnMasseDownloaderProgress, error) {
	if _, err := os.Stat(emd.progressFilePath); os.IsNotExist(err) {
		return nil, nil // No progress file exists
	}

	data, err := os.ReadFile(emd.progressFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read progress file: %w", err)
	}

	var progress EnMasseDownloaderProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal progress: %w", err)
	}

	emd.logger.Debug().Str("progressFile", emd.progressFilePath).Int("processedSeries", progress.ProcessedSeries).Msg("en_masse_downloader: Loaded progress")
	return &progress, nil
}

// clearProgress removes the progress file
func (emd *EnMasseDownloader) clearProgress() error {
	if _, err := os.Stat(emd.progressFilePath); os.IsNotExist(err) {
		return nil // No progress file exists
	}

	if err := os.Remove(emd.progressFilePath); err != nil {
		return fmt.Errorf("failed to remove progress file: %w", err)
	}

	emd.logger.Debug().Str("progressFile", emd.progressFilePath).Msg("en_masse_downloader: Cleared progress")
	return nil
}

// filterUnprocessedSeries filters out already processed series from the catalogue
func (emd *EnMasseDownloader) filterUnprocessedSeries(catalogue []WeebCentralCatalogueEntry, processedIDs []string) []WeebCentralCatalogueEntry {
	processedSet := make(map[string]bool)
	for _, id := range processedIDs {
		processedSet[id] = true
	}

	var unprocessed []WeebCentralCatalogueEntry
	for _, entry := range catalogue {
		if !processedSet[entry.ID] {
			unprocessed = append(unprocessed, entry)
		}
	}

	return unprocessed
}

// GetStatus returns the current status of the en masse downloader
func (emd *EnMasseDownloader) GetStatus() *EnMasseDownloaderStatus {
	emd.mu.RLock()
	defer emd.mu.RUnlock()

	progress := float64(0)
	if emd.totalSeries > 0 {
		progress = float64(emd.processedSeries) / float64(emd.totalSeries) * 100
	}

	estimatedTimeRemaining := "Unknown"
	if emd.isRunning && emd.processedSeries > 0 {
		elapsed := time.Since(emd.startTime)
		avgTimePerSeries := elapsed / time.Duration(emd.processedSeries)
		remaining := time.Duration(emd.totalSeries-emd.processedSeries) * avgTimePerSeries
		estimatedTimeRemaining = remaining.Round(time.Second).String()
	}

	return &EnMasseDownloaderStatus{
		IsRunning:              emd.isRunning,
		TotalSeries:            emd.totalSeries,
		ProcessedSeries:        emd.processedSeries,
		CurrentSeries:          emd.currentSeries,
		StartTime:              emd.startTime,
		Progress:               progress,
		EstimatedTimeRemaining: estimatedTimeRemaining,
	}
}

// Start begins the en masse download process
func (emd *EnMasseDownloader) Start() error {
	emd.mu.Lock()
	defer emd.mu.Unlock()

	if emd.isRunning {
		return fmt.Errorf("en masse downloader is already running")
	}

	// Load catalogue
	catalogue, err := emd.loadCatalogue()
	if err != nil {
		return fmt.Errorf("failed to load catalogue: %w", err)
	}

	// Load existing progress if available
	progress, err := emd.loadProgress()
	if err != nil {
		emd.logger.Warn().Err(err).Msg("en_masse_downloader: Failed to load progress, starting fresh")
		progress = nil
	}

	var catalogueToProcess []WeebCentralCatalogueEntry
	var resuming bool

	if progress != nil && len(progress.ProcessedSeriesIDs) > 0 {
		// Resume from existing progress
		catalogueToProcess = emd.filterUnprocessedSeries(catalogue, progress.ProcessedSeriesIDs)
		emd.processedSeries = progress.ProcessedSeries
		emd.startTime = progress.StartTime
		resuming = true
		emd.logger.Info().
			Int("totalSeries", len(catalogue)).
			Int("processedSeries", emd.processedSeries).
			Int("remainingSeries", len(catalogueToProcess)).
			Time("originalStartTime", emd.startTime).
			Msg("en_masse_downloader: Resuming bulk download process from saved progress")
	} else {
		// Start fresh
		catalogueToProcess = catalogue
		emd.processedSeries = 0
		emd.startTime = time.Now()
		resuming = false
		emd.logger.Info().
			Int("totalSeries", len(catalogue)).
			Msg("en_masse_downloader: Starting fresh bulk download process")
	}

	emd.isRunning = true
	emd.totalSeries = len(catalogue)
	emd.stopCh = make(chan struct{})

	// Persist running state for auto-resume on next startup
	emd.setRunningState(true)

	if resuming {
		emd.wsEventManager.SendEvent(events.InfoToast, fmt.Sprintf("En Masse Downloader resumed - %d/%d series remaining", len(catalogueToProcess), emd.totalSeries))
	} else {
		emd.wsEventManager.SendEvent(events.InfoToast, fmt.Sprintf("En Masse Downloader started - Processing %d series", emd.totalSeries))
	}

	// Start the manga download queue to process queued chapters
	emd.downloader.RunChapterDownloadQueue()
	emd.logger.Info().Msg("en_masse_downloader: Started manga download queue")

	// Start the download process in a goroutine
	go emd.processAllSeries(catalogueToProcess)

	return nil
}

// Stop stops the en masse download process
func (emd *EnMasseDownloader) Stop() error {
	emd.mu.Lock()
	defer emd.mu.Unlock()

	if !emd.isRunning {
		return fmt.Errorf("en masse downloader is not running")
	}

	close(emd.stopCh)
	emd.isRunning = false

	// Clear running state so we don't auto-resume after a manual stop
	emd.setRunningState(false)

	emd.logger.Info().Msg("en_masse_downloader: Stopping bulk download process")
	emd.wsEventManager.SendEvent(events.InfoToast, "En Masse Downloader stopped")

	return nil
}

// loadCatalogue loads the weebcentral catalogue from the JSON file
func (emd *EnMasseDownloader) loadCatalogue() ([]WeebCentralCatalogueEntry, error) {
	file, err := os.Open(emd.cataloguePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open catalogue file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read catalogue file: %w", err)
	}

	var catalogue []WeebCentralCatalogueEntry
	if err := json.Unmarshal(data, &catalogue); err != nil {
		return nil, fmt.Errorf("failed to parse catalogue JSON: %w", err)
	}

	emd.logger.Info().
		Int("seriesCount", len(catalogue)).
		Msg("en_masse_downloader: Loaded weebcentral catalogue")

	return catalogue, nil
}

// processAllSeries processes all series in the catalogue sequentially
func (emd *EnMasseDownloader) processAllSeries(catalogue []WeebCentralCatalogueEntry) {
	var processedSeriesIDs []string

	// Load existing processed series IDs if resuming
	if progress, err := emd.loadProgress(); err == nil && progress != nil {
		processedSeriesIDs = progress.ProcessedSeriesIDs
	}

	defer func() {
		emd.mu.Lock()
		emd.isRunning = false
		emd.mu.Unlock()

		// Ensure running state is cleared on completion
		emd.setRunningState(false)

		// Clear progress file on successful completion
		if err := emd.clearProgress(); err != nil {
			emd.logger.Warn().Err(err).Msg("en_masse_downloader: Failed to clear progress file")
		}

		// Refresh the metadata cache to ensure newly downloaded manga appear in the UI
		emd.logger.Info().Msg("en_masse_downloader: Refreshing metadata cache for newly downloaded manga")
		emd.downloader.RefreshDownloadedMangaCache()

		emd.logger.Info().
			Int("processedSeries", emd.processedSeries).
			Int("totalSeries", emd.totalSeries).
			Dur("totalTime", time.Since(emd.startTime)).
			Msg("en_masse_downloader: Bulk download process completed")

		emd.wsEventManager.SendEvent(events.SuccessToast, fmt.Sprintf("En Masse Downloader completed - Processed %d/%d series", emd.processedSeries, emd.totalSeries))
	}()

	for i, entry := range catalogue {
		select {
		case <-emd.stopCh:
			emd.logger.Info().Msg("en_masse_downloader: Process stopped by user")
			return
		default:
			// Update current series
			emd.mu.Lock()
			emd.currentSeries = entry.Title
			emd.mu.Unlock()

			emd.logger.Debug().
				Int("index", i+1).
				Int("total", len(catalogue)).
				Str("title", entry.Title).
				Str("id", entry.ID).
				Msg("en_masse_downloader: Processing series")

			// Process the series
			if err := emd.processSeries(entry); err != nil {
				emd.logger.Error().
					Err(err).
					Str("title", entry.Title).
					Str("id", entry.ID).
					Msg("en_masse_downloader: Failed to process series")
			} else {
				// Refresh metadata cache after successfully processing a series
				// so newly downloaded manga appear in the UI immediately
				emd.logger.Debug().Str("title", entry.Title).Msg("en_masse_downloader: Refreshing metadata cache for processed series")
				emd.downloader.RefreshDownloadedMangaCache()
			}

			// Update progress
			emd.mu.Lock()
			emd.processedSeries++
			// Add this series to the processed list
			processedSeriesIDs = append(processedSeriesIDs, entry.ID)
			// Save progress after each series
			if err := emd.saveProgress(processedSeriesIDs); err != nil {
				emd.logger.Warn().Err(err).Msg("en_masse_downloader: Failed to save progress")
			}
			emd.mu.Unlock()

			// Send progress update
			status := emd.GetStatus()
			emd.wsEventManager.SendEvent("en_masse_downloader_progress", status)

			// Sleep between series (except for the last one)
			if i < len(catalogue)-1 {
				select {
				case <-emd.stopCh:
					return
				case <-time.After(emd.delayBetweenSeries):
				}
			}
		}
	}
}

// processSeries processes a single manga series
func (emd *EnMasseDownloader) processSeries(entry WeebCentralCatalogueEntry) error {
	// Extract series ID from the weebcentral URL format
	// Format: /series/{id}/{title-slug}
	parts := strings.Split(entry.ID, "/")
	if len(parts) < 3 {
		return fmt.Errorf("invalid series ID format: %s", entry.ID)
	}
	seriesID := parts[2] // Get the actual ID part

	// Search for the manga in the database/AniList to get media ID
	mediaID, err := emd.findMangaMediaID(entry.Title, seriesID)
	if err != nil {
		emd.logger.Warn().
			Err(err).
			Str("title", entry.Title).
			Str("seriesID", seriesID).
			Msg("en_masse_downloader: Could not find media ID for series, skipping")
		return nil // Skip this series but don't fail the entire process
	}

	// Get manga chapters from weebcentral provider
	provider := manga_providers.NewWeebCentral(emd.logger)

	// Search for the manga on weebcentral
	searchResults, err := provider.Search(hibikemanga.SearchOptions{
		Query: entry.Title,
	})
	if err != nil {
		return fmt.Errorf("failed to search for manga on weebcentral: %w", err)
	}

	if len(searchResults) == 0 {
		emd.logger.Warn().
			Str("title", entry.Title).
			Msg("en_masse_downloader: No search results found on weebcentral")
		return nil
	}

	// Find the best match (first result for now, could be improved with fuzzy matching)
	bestMatch := searchResults[0]

	// Log cover image URL for debugging
	emd.logger.Debug().
		Str("title", entry.Title).
		Str("coverImageUrl", bestMatch.Image).
		Msg("en_masse_downloader: Found cover image URL from search results")

	// Get chapters for the manga
	chapters, err := provider.FindChapters(bestMatch.ID)
	if err != nil {
		return fmt.Errorf("failed to get chapters for manga: %w", err)
	}

	if len(chapters) == 0 {
		emd.logger.Info().
			Str("title", entry.Title).
			Msg("en_masse_downloader: No chapters found for series")
		return nil
	}

	emd.logger.Info().
		Str("title", entry.Title).
		Int("chapterCount", len(chapters)).
		Str("coverImageUrl", bestMatch.Image).
		Msg("en_masse_downloader: Found chapters, queuing for download")

	// Store the chapter container in filecache under the synthetic media ID
	// This is required for DownloadChapter to find the chapters
	chapterContainer := &ChapterContainer{
		Provider: "weebcentral",
		MediaId:  mediaID,
		Chapters: chapters,
	}

	// Store in repository filecache so DownloadChapter can find it
	// Use the same pattern as the existing code in chapter_container.go
	containerBucket := emd.repository.getFcProviderBucket("weebcentral", mediaID, bucketTypeChapter)
	chapterContainerKey := getMangaChapterContainerCacheKey("weebcentral", mediaID)
	err = emd.repository.fileCacher.Set(containerBucket, chapterContainerKey, chapterContainer)
	if err != nil {
		emd.logger.Error().
			Err(err).
			Int("mediaId", mediaID).
			Msg("en_masse_downloader: Failed to store chapter container in filecache")
		return fmt.Errorf("failed to store chapter container in filecache: %w", err)
	}

	emd.logger.Debug().
		Int("mediaId", mediaID).
		Int("chapterCount", len(chapters)).
		Msg("en_masse_downloader: Stored chapter container in filecache")

	// Process each chapter in the series
	for _, chapter := range chapters {
		select {
		case <-emd.stopCh:
			return fmt.Errorf("process stopped")
		default:
			// Extract chapter title for logging
			chapterTitle := chapter.Title
			if strings.Contains(chapterTitle, " - ") {
				parts := strings.Split(chapterTitle, " - ")
				if len(parts) > 1 {
					chapterTitle = strings.Join(parts[1:], " - ")
				}
			}

			// Use the exact same approach as the regular download handler with retry logic for rate limiting
			emd.logger.Info().
				Str("provider", "weebcentral").
				Int("mediaId", mediaID).
				Str("chapterId", chapter.ID).
				Str("chapterTitle", chapterTitle).
				Str("chapterTitle", chapter.Title).
				Str("seriesTitle", entry.Title).
				Msg("en_masse_downloader: About to call DownloadChapter")

			// Implement retry logic with exponential backoff for rate limiting
			var err error
			maxRetries := 3
			baseDelay := 5 * time.Second

			for attempt := 0; attempt <= maxRetries; attempt++ {
				err = emd.downloader.DownloadChapter(DownloadChapterOptions{
					Provider:      "weebcentral",
					MediaId:       mediaID,
					ChapterId:     chapter.ID,
					SeriesTitle:   entry.Title,     // Use the series title from the catalogue
					ChapterTitle:  chapterTitle,    // Use the extracted chapter title
					CoverImageUrl: bestMatch.Image, // Store the cover image URL from search results
					StartNow:      true,            // Use StartNow: true like regular downloads
				})

				if err == nil {
					break // Success, exit retry loop
				}

				// Check if it's a rate limiting error
				if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Too Many Requests") {
					if attempt < maxRetries {
						retryDelay := baseDelay * time.Duration(1<<uint(attempt)) // Exponential backoff: 5s, 10s, 20s
						emd.logger.Warn().
							Err(err).
							Str("chapterId", chapter.ID).
							Int("attempt", attempt+1).
							Int("maxRetries", maxRetries).
							Dur("retryDelay", retryDelay).
							Msg("en_masse_downloader: Rate limited, retrying after delay")

						// Wait for retry delay, but check for stop signal
						select {
						case <-emd.stopCh:
							return fmt.Errorf("process stopped during retry")
						case <-time.After(retryDelay):
							// Continue to next retry attempt
						}
					} else {
						emd.logger.Error().
							Err(err).
							Str("chapterId", chapter.ID).
							Msg("en_masse_downloader: Max retries exceeded for rate limited chapter")
					}
				} else {
					// Non-rate-limiting error, don't retry
					break
				}
			}

			if err == nil {
				emd.logger.Info().
					Str("chapterId", chapter.ID).
					Msg("en_masse_downloader: Successfully queued chapter")
			}
			if err != nil {
				emd.logger.Error().
					Err(err).
					Str("chapterID", chapter.ID).
					Str("chapterTitle", chapter.Title).
					Msg("en_masse_downloader: Failed to queue chapter")
				continue
			}

			// Add delay between chapters to avoid rate limiting (same as manual downloads)
			time.Sleep(2 * time.Second)
		}
	}

	emd.logger.Info().
		Str("title", entry.Title).
		Int("queuedChapters", len(chapters)).
		Msg("en_masse_downloader: Successfully queued all chapters")

	return nil
}

// findMangaMediaID generates a synthetic media ID for En Masse downloaded manga
// Always uses synthetic IDs to ensure compatibility with local provider system
func (emd *EnMasseDownloader) findMangaMediaID(title, seriesID string) (int, error) {
	// For En Masse downloads, always use synthetic IDs to ensure they work with the local provider
	// This prevents issues where manga exist in AniList collection but need synthetic ID handling
	syntheticID := emd.generateSyntheticMediaID(seriesID)
	emd.logger.Info().
		Str("title", title).
		Str("seriesID", seriesID).
		Int("syntheticID", syntheticID).
		Msg("en_masse_downloader: Using synthetic media ID for En Masse download")

	return syntheticID, nil
}

// titleMatches checks if two manga titles match using fuzzy matching
func (emd *EnMasseDownloader) titleMatches(searchTitle string, anilistTitle *anilist.BaseManga_Title) bool {
	if anilistTitle == nil {
		return false
	}

	// Normalize titles for comparison
	searchTitleLower := strings.ToLower(strings.TrimSpace(searchTitle))

	// Check against various title formats, handling pointer types
	titlesToCheck := []*string{
		anilistTitle.Romaji,
		anilistTitle.English,
		anilistTitle.Native,
		anilistTitle.UserPreferred,
	}

	for _, titlePtr := range titlesToCheck {
		if titlePtr != nil && *titlePtr != "" {
			titleLower := strings.ToLower(strings.TrimSpace(*titlePtr))
			// Simple exact match for now - could be enhanced with fuzzy matching
			if searchTitleLower == titleLower {
				return true
			}
			// Check if one title contains the other
			if strings.Contains(searchTitleLower, titleLower) || strings.Contains(titleLower, searchTitleLower) {
				return true
			}
		}
	}

	return false
}

// generateSyntheticMediaID creates a synthetic media ID from a series ID
func (emd *EnMasseDownloader) generateSyntheticMediaID(seriesID string) int {
	// Use FNV hash to generate a consistent synthetic ID
	h := fnv.New32a()
	h.Write([]byte(seriesID))
	hashValue := h.Sum32()

	// Ensure the ID is positive and within a reasonable range
	// Use modulo to keep it within a specific range (e.g., 1000000-9999999)
	syntheticID := int(hashValue%8999999) + 1000000

	return syntheticID
}

// GetMangaCollection retrieves the manga collection (placeholder method)
func (r *Repository) GetMangaCollection(cached bool) (*anilist.MangaCollection, error) {
	// This method should already exist in the Repository
	// If not, it needs to be implemented to get the user's manga collection from AniList
	return nil, fmt.Errorf("GetMangaCollection not implemented")
}

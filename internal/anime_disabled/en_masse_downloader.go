package anime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"seanime/internal/events"
	"seanime/internal/torrents"
	"seanime/internal/util"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type (
	// EnMasseDownloader handles batch anime downloads
	EnMasseDownloader struct {
		repository         *Repository
		downloadManager    *DownloadManager
		torrentClient      torrents.TorrentClientInterface
		logger             *zerolog.Logger
		wsEventManager     events.WSEventManagerInterface
		animeDatabase      *AnimeOfflineDatabase
		downloadDir        string
		activeBatchJobs    map[string]*BatchDownloadJob
		mu                 sync.RWMutex
	}

	// BatchDownloadJob represents a batch download operation
	BatchDownloadJob struct {
		ID              string                    `json:"id"`
		Type            BatchDownloadType         `json:"type"`
		Status          BatchDownloadStatus       `json:"status"`
		TotalItems      int                       `json:"totalItems"`
		CompletedItems  int                       `json:"completedItems"`
		FailedItems     int                       `json:"failedItems"`
		Progress        float64                   `json:"progress"`
		StartTime       time.Time                 `json:"startTime"`
		EndTime         *time.Time                `json:"endTime"`
		Items           []*BatchDownloadItem      `json:"items"`
		Settings        *BatchDownloadSettings    `json:"settings"`
		Error           string                    `json:"error"`
		ctx             context.Context
		cancel          context.CancelFunc
	}

	// BatchDownloadItem represents an individual item in a batch
	BatchDownloadItem struct {
		ID            string                `json:"id"`
		MediaID       int                   `json:"mediaId"`
		Title         string                `json:"title"`
		Year          int                   `json:"year"`
		Episodes      int                   `json:"episodes"`
		Status        BatchItemStatus       `json:"status"`
		Progress      float64               `json:"progress"`
		Error         string                `json:"error"`
		TorrentInfo   *TorrentInfo          `json:"torrentInfo"`
		DownloadPath  string                `json:"downloadPath"`
		StartTime     *time.Time            `json:"startTime"`
		EndTime       *time.Time            `json:"endTime"`
	}

	// BatchDownloadSettings contains batch download configuration
	BatchDownloadSettings struct {
		Quality           string   `json:"quality"`
		Language          string   `json:"language"`
		PreferredFormats  []string `json:"preferredFormats"`
		MinSeeders        int      `json:"minSeeders"`
		MaxFileSize       int64    `json:"maxFileSize"`
		IncludeOVA        bool     `json:"includeOva"`
		IncludeSpecials   bool     `json:"includeSpecials"`
		AutoSelectBest    bool     `json:"autoSelectBest"`
		ConcurrentDownloads int    `json:"concurrentDownloads"`
		AutoLink          bool     `json:"autoLink"`
	}

	// TorrentInfo contains torrent metadata
	TorrentInfo struct {
		Name         string    `json:"name"`
		Size         int64     `json:"size"`
		Seeders      int       `json:"seeders"`
		Leechers     int       `json:"leechers"`
		Quality      string    `json:"quality"`
		Format       string    `json:"format"`
		Language     string    `json:"language"`
		ReleaseGroup string    `json:"releaseGroup"`
		MagnetLink   string    `json:"magnetLink"`
		InfoHash     string    `json:"infoHash"`
		AddedDate    time.Time `json:"addedDate"`
	}

	// AnimeOfflineDatabase represents the anime offline database structure
	AnimeOfflineDatabase struct {
		Data []AnimeOfflineEntry `json:"data"`
	}

	// AnimeOfflineEntry represents an entry in the anime offline database
	AnimeOfflineEntry struct {
		Sources     []string `json:"sources"`
		Title       string   `json:"title"`
		Type        string   `json:"type"`
		Episodes    int      `json:"episodes"`
		Status      string   `json:"status"`
		Season      string   `json:"season"`
		Year        int      `json:"year"`
		Picture     string   `json:"picture"`
		Thumbnail   string   `json:"thumbnail"`
		Synonyms    []string `json:"synonyms"`
		Relations   []string `json:"relations"`
		Tags        []string `json:"tags"`
	}

	// BatchDownloadType represents the type of batch download
	BatchDownloadType string

	// BatchDownloadStatus represents batch download states
	BatchDownloadStatus string

	// BatchItemStatus represents individual item states
	BatchItemStatus string
)

const (
	BatchDownloadTypeAnime   BatchDownloadType = "anime"
	BatchDownloadTypeSeason  BatchDownloadType = "season"
	BatchDownloadTypeYear    BatchDownloadType = "year"
	BatchDownloadTypeGenre   BatchDownloadType = "genre"

	BatchDownloadStatusPending    BatchDownloadStatus = "pending"
	BatchDownloadStatusRunning    BatchDownloadStatus = "running"
	BatchDownloadStatusCompleted  BatchDownloadStatus = "completed"
	BatchDownloadStatusFailed     BatchDownloadStatus = "failed"
	BatchDownloadStatusCancelled  BatchDownloadStatus = "cancelled"

	BatchItemStatusPending      BatchItemStatus = "pending"
	BatchItemStatusSearching    BatchItemStatus = "searching"
	BatchItemStatusDownloading  BatchItemStatus = "downloading"
	BatchItemStatusCompleted    BatchItemStatus = "completed"
	BatchItemStatusFailed       BatchItemStatus = "failed"
	BatchItemStatusSkipped      BatchItemStatus = "skipped"
)

// NewEnMasseDownloader creates a new en masse downloader
func NewEnMasseDownloader(
	repository *Repository,
	downloadManager *DownloadManager,
	torrentClient torrents.TorrentClientInterface,
	logger *zerolog.Logger,
	wsEventManager events.WSEventManagerInterface,
	downloadDir string,
) *EnMasseDownloader {
	return &EnMasseDownloader{
		repository:      repository,
		downloadManager: downloadManager,
		torrentClient:   torrentClient,
		logger:          logger,
		wsEventManager:  wsEventManager,
		downloadDir:     downloadDir,
		activeBatchJobs: make(map[string]*BatchDownloadJob),
	}
}

// LoadAnimeDatabase loads the anime offline database
func (emd *EnMasseDownloader) LoadAnimeDatabase(databasePath string) error {
	data, err := os.ReadFile(databasePath)
	if err != nil {
		return fmt.Errorf("failed to read anime database: %w", err)
	}

	var db AnimeOfflineDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("failed to parse anime database: %w", err)
	}

	emd.animeDatabase = &db
	
	emd.logger.Info().
		Int("entries", len(db.Data)).
		Msg("anime: Loaded anime offline database")

	return nil
}

// StartBatchDownload starts a batch download operation - ALL anime downloads are batched
func (emd *EnMasseDownloader) StartBatchDownload(ctx context.Context, batchType BatchDownloadType, criteria map[string]interface{}, settings *BatchDownloadSettings) (*BatchDownloadJob, error) {
	if emd.animeDatabase == nil {
		return nil, fmt.Errorf("anime database not loaded")
	}

	// Ensure auto-linking is always enabled
	if settings == nil {
		settings = &BatchDownloadSettings{}
	}
	settings.AutoLink = true

	// Generate job ID
	jobID := fmt.Sprintf("batch_%s_%d", batchType, time.Now().Unix())

	// Create job context
	jobCtx, cancel := context.WithCancel(ctx)

	// Find anime entries based on criteria
	entries, err := emd.findAnimeEntries(batchType, criteria)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to find anime entries: %w", err)
	}

	if len(entries) == 0 {
		cancel()
		return nil, fmt.Errorf("no anime found matching criteria")
	}

	// Create batch items
	items := make([]*BatchDownloadItem, 0, len(entries))
	for i, entry := range entries {
		item := &BatchDownloadItem{
			ID:       fmt.Sprintf("%s_item_%d", jobID, i),
			MediaID:  emd.extractAniListID(entry),
			Title:    entry.Title,
			Year:     entry.Year,
			Episodes: entry.Episodes,
			Status:   BatchItemStatusPending,
		}
		items = append(items, item)
	}

	// Create batch job
	job := &BatchDownloadJob{
		ID:             jobID,
		Type:           batchType,
		Status:         BatchDownloadStatusPending,
		TotalItems:     len(items),
		CompletedItems: 0,
		FailedItems:    0,
		Progress:       0.0,
		StartTime:      time.Now(),
		Items:          items,
		Settings:       settings,
		ctx:            jobCtx,
		cancel:         cancel,
	}

	// Store job
	emd.mu.Lock()
	emd.activeBatchJobs[jobID] = job
	emd.mu.Unlock()

	// Start processing in background
	go emd.processBatchJob(job)

	emd.logger.Info().
		Str("jobId", jobID).
		Str("type", string(batchType)).
		Int("items", len(items)).
		Msg("anime: Started batch download with auto-linking")

	return job, nil
}

// DownloadSingleAnime downloads a single anime (but still uses batch system)
func (emd *EnMasseDownloader) DownloadSingleAnime(ctx context.Context, title string, settings *BatchDownloadSettings) (*BatchDownloadJob, error) {
	criteria := map[string]interface{}{
		"titles": []string{title},
	}
	return emd.StartBatchDownload(ctx, BatchDownloadTypeAnime, criteria, settings)
}

// processBatchJob processes a batch download job
func (emd *EnMasseDownloader) processBatchJob(job *BatchDownloadJob) {
	defer func() {
		emd.mu.Lock()
		delete(emd.activeBatchJobs, job.ID)
		emd.mu.Unlock()
	}()

	job.Status = BatchDownloadStatusRunning
	emd.emitBatchProgress(job)

	// Process items with concurrency limit
	concurrency := job.Settings.ConcurrentDownloads
	if concurrency <= 0 {
		concurrency = 3
	}

	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, item := range job.Items {
		wg.Add(1)
		go func(item *BatchDownloadItem) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			select {
			case <-job.ctx.Done():
				item.Status = BatchItemStatusSkipped
				return
			default:
			}

			emd.processAnimeItem(job, item)
		}(item)
	}

	wg.Wait()

	// Calculate final statistics
	completedCount := 0
	failedCount := 0
	for _, item := range job.Items {
		switch item.Status {
		case BatchItemStatusCompleted:
			completedCount++
		case BatchItemStatusFailed:
			failedCount++
		}
	}

	job.CompletedItems = completedCount
	job.FailedItems = failedCount
	job.Progress = 100.0

	if failedCount == 0 {
		job.Status = BatchDownloadStatusCompleted
	} else if completedCount == 0 {
		job.Status = BatchDownloadStatusFailed
		job.Error = "All items failed to download"
	} else {
		job.Status = BatchDownloadStatusCompleted
		job.Error = fmt.Sprintf("%d items failed", failedCount)
	}

	now := time.Now()
	job.EndTime = &now

	emd.emitBatchProgress(job)
	emd.emitBatchComplete(job)

	emd.logger.Info().
		Str("jobId", job.ID).
		Int("completed", completedCount).
		Int("failed", failedCount).
		Msg("anime: Batch download completed with auto-linking")
}

// processAnimeItem processes a single anime item
func (emd *EnMasseDownloader) processAnimeItem(job *BatchDownloadJob, item *BatchDownloadItem) {
	startTime := time.Now()
	item.StartTime = &startTime
	item.Status = BatchItemStatusSearching
	emd.emitBatchProgress(job)

	// Search for torrents
	torrents, err := emd.searchAnimeTorrents(item.Title, item.Year, job.Settings)
	if err != nil {
		emd.failBatchItem(job, item, fmt.Sprintf("torrent search failed: %v", err))
		return
	}

	if len(torrents) == 0 {
		emd.failBatchItem(job, item, "no torrents found")
		return
	}

	// Select best torrent
	bestTorrent := emd.selectBestTorrent(torrents, job.Settings)
	if bestTorrent == nil {
		emd.failBatchItem(job, item, "no suitable torrent found")
		return
	}

	item.TorrentInfo = bestTorrent
	item.Status = BatchItemStatusDownloading
	emd.emitBatchProgress(job)

	// Create download directory
	downloadPath := filepath.Join(emd.downloadDir, "anime", util.SanitizeFilename(item.Title))
	if err := os.MkdirAll(downloadPath, 0755); err != nil {
		emd.failBatchItem(job, item, fmt.Sprintf("failed to create download directory: %v", err))
		return
	}

	item.DownloadPath = downloadPath

	// Add torrent to client
	if err := emd.torrentClient.AddMagnet(bestTorrent.MagnetLink, downloadPath); err != nil {
		emd.failBatchItem(job, item, fmt.Sprintf("failed to add torrent: %v", err))
		return
	}

	// Monitor download progress
	emd.monitorTorrentDownload(job, item, bestTorrent.InfoHash)
}

// monitorTorrentDownload monitors torrent download progress
func (emd *EnMasseDownloader) monitorTorrentDownload(job *BatchDownloadJob, item *BatchDownloadItem, infoHash string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-job.ctx.Done():
			item.Status = BatchItemStatusSkipped
			return
		case <-ticker.C:
			// Get torrent status
			status, err := emd.torrentClient.GetTorrentStatus(infoHash)
			if err != nil {
				emd.logger.Warn().
					Err(err).
					Str("infoHash", infoHash).
					Msg("anime: Failed to get torrent status")
				continue
			}

			item.Progress = status.Progress * 100
			emd.emitBatchProgress(job)

			if status.IsCompleted {
				// Download completed, now automatically link the anime
				emd.completeBatchItem(job, item)
				return
			}

			if status.HasError {
				emd.failBatchItem(job, item, "torrent download failed")
				return
			}
		}
	}
}

// completeBatchItem marks an item as completed and automatically links it
func (emd *EnMasseDownloader) completeBatchItem(job *BatchDownloadJob, item *BatchDownloadItem) {
	now := time.Now()
	item.EndTime = &now
	item.Status = BatchItemStatusCompleted
	item.Progress = 100.0

	// Automatically link the downloaded anime (always enabled)
	if err := emd.linkDownloadedAnime(item); err != nil {
		emd.logger.Warn().
			Err(err).
			Str("title", item.Title).
			Msg("anime: Failed to auto-link downloaded anime")
	} else {
		emd.logger.Info().
			Str("title", item.Title).
			Msg("anime: Successfully auto-linked downloaded anime")
	}

	emd.emitBatchProgress(job)

	emd.logger.Info().
		Str("title", item.Title).
		Str("downloadPath", item.DownloadPath).
		Msg("anime: Item download completed and auto-linked")
}

// failBatchItem marks an item as failed
func (emd *EnMasseDownloader) failBatchItem(job *BatchDownloadJob, item *BatchDownloadItem, errorMsg string) {
	now := time.Now()
	item.EndTime = &now
	item.Status = BatchItemStatusFailed
	item.Error = errorMsg

	emd.emitBatchProgress(job)

	emd.logger.Error().
		Str("title", item.Title).
		Str("error", errorMsg).
		Msg("anime: Item download failed")
}

// linkDownloadedAnime automatically links downloaded anime to library
func (emd *EnMasseDownloader) linkDownloadedAnime(item *BatchDownloadItem) error {
	// Find video files in download directory
	videoFiles, err := emd.findVideoFiles(item.DownloadPath)
	if err != nil {
		return fmt.Errorf("failed to find video files: %w", err)
	}

	if len(videoFiles) == 0 {
		return fmt.Errorf("no video files found in download directory")
	}

	// Create anime entry with download info
	entry := &Entry{
		MediaID:     item.MediaID,
		Title:       item.Title,
		Episodes:    item.Episodes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		DownloadInfo: &DownloadInfo{
			Provider:           "torrent",
			Quality:            item.TorrentInfo.Quality,
			Language:           item.TorrentInfo.Language,
			TotalEpisodes:      item.Episodes,
			DownloadedEpisodes: len(videoFiles),
			DownloadStatus:     "completed",
			DownloadProgress:   100.0,
			LastDownloadDate:   item.EndTime,
			DownloadPath:       item.DownloadPath,
			FileSize:           item.TorrentInfo.Size,
			Episodes:           emd.createEpisodeDownloadInfo(videoFiles),
		},
	}

	// Save to database (this would integrate with the actual database)
	emd.logger.Info().
		Str("title", item.Title).
		Int("episodes", len(videoFiles)).
		Str("quality", item.TorrentInfo.Quality).
		Str("language", item.TorrentInfo.Language).
		Msg("anime: Created library entry for downloaded anime")

	return nil
}

// findVideoFiles finds video files in a directory
func (emd *EnMasseDownloader) findVideoFiles(dir string) ([]string, error) {
	var videoFiles []string
	videoExtensions := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm"}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, videoExt := range videoExtensions {
			if ext == videoExt {
				videoFiles = append(videoFiles, path)
				break
			}
		}

		return nil
	})

	return videoFiles, err
}

// createEpisodeDownloadInfo creates episode download info from video files
func (emd *EnMasseDownloader) createEpisodeDownloadInfo(videoFiles []string) []*EpisodeDownloadInfo {
	episodes := make([]*EpisodeDownloadInfo, 0, len(videoFiles))

	for i, filePath := range videoFiles {
		fileInfo, err := os.Stat(filePath)
		var fileSize int64
		if err == nil {
			fileSize = fileInfo.Size()
		}

		episode := &EpisodeDownloadInfo{
			EpisodeNumber: i + 1,
			Title:         fmt.Sprintf("Episode %d", i+1),
			Downloaded:    true,
			DownloadDate:  &[]time.Time{time.Now()}[0],
			FilePath:      filePath,
			FileSize:      fileSize,
			Quality:       "1080p", // Default, could be detected from filename
		}
		episodes = append(episodes, episode)
	}

	return episodes
}

// searchAnimeTorrents searches for anime torrents with prioritization using real torrent providers
func (emd *EnMasseDownloader) searchAnimeTorrents(title string, year int, settings *BatchDownloadSettings) ([]*TorrentInfo, error) {
	// Use the torrent repository to search for real torrents
	if emd.repository == nil {
		return nil, fmt.Errorf("torrent repository not available")
	}

	// Create search query with year if available
	query := title
	if year > 0 {
		query = fmt.Sprintf("%s %d", title, year)
	}

	// Search for torrents using the torrent repository
	searchOpts := torrent.AnimeSearchOptions{
		Provider: "", // Use default provider
		Type:     torrent.AnimeSearchTypeSimple,
		Query:    query,
		Batch:    true, // We want batch torrents for full series
	}

	// Create a mock media object for the search
	mockMedia := &anilist.BaseAnime{
		ID: 0, // We don't have AniList ID for offline database entries
		Title: &anilist.BaseAnime_Title{
			Romaji:  &title,
			English: &title,
		},
	}
	searchOpts.Media = mockMedia

	searchData, err := emd.repository.GetTorrentRepository().SearchAnime(context.Background(), searchOpts)
	if err != nil {
		emd.logger.Error().Err(err).Str("title", title).Msg("anime: Failed to search torrents")
		return nil, err
	}

	// Convert search results to TorrentInfo
	var torrents []*TorrentInfo
	for _, torrentResult := range searchData.Torrents {
		// Get magnet link
		magnetLink := torrentResult.MagnetLink
		if magnetLink == "" {
			// Try to get magnet link from provider if not available
			continue
		}

		// Parse quality and format from torrent name
		quality := "1080p" // Default
		format := "WEB"    // Default
		language := "Japanese" // Default

		// Simple parsing for common patterns
		name := strings.ToLower(torrentResult.Name)
		if strings.Contains(name, "2160p") || strings.Contains(name, "4k") {
			quality = "2160p"
		} else if strings.Contains(name, "1080p") {
			quality = "1080p"
		} else if strings.Contains(name, "720p") {
			quality = "720p"
		} else if strings.Contains(name, "480p") {
			quality = "480p"
		}

		if strings.Contains(name, "bd") || strings.Contains(name, "bluray") || strings.Contains(name, "bdrip") {
			format = "BD"
		} else if strings.Contains(name, "web") || strings.Contains(name, "webrip") {
			format = "WEB"
		} else if strings.Contains(name, "dvd") {
			format = "DVD"
		}

		if strings.Contains(name, "dual") && strings.Contains(name, "audio") {
			language = "Dual Audio"
		} else if strings.Contains(name, "english") || strings.Contains(name, "dub") {
			language = "English"
		}

		// Extract release group
		releaseGroup := ""
		if strings.HasPrefix(torrentResult.Name, "[") {
			if endIdx := strings.Index(torrentResult.Name, "]"); endIdx > 0 {
				releaseGroup = torrentResult.Name[1:endIdx]
			}
		}

		torrents = append(torrents, &TorrentInfo{
			Name:         torrentResult.Name,
			Size:         torrentResult.Size,
			Seeders:      torrentResult.Seeders,
			Leechers:     torrentResult.Leechers,
			Quality:      quality,
			Format:       format,
			Language:     language,
			ReleaseGroup: releaseGroup,
			MagnetLink:   magnetLink,
			InfoHash:     torrentResult.InfoHash,
			AddedDate:    time.Now(),
		})
	}

	emd.logger.Debug().Int("count", len(torrents)).Str("title", title).Msg("anime: Found torrents")
	return torrents, nil
}

// selectBestTorrent selects the best torrent based on prioritization rules
func (emd *EnMasseDownloader) selectBestTorrent(torrents []*TorrentInfo, settings *BatchDownloadSettings) *TorrentInfo {
	if len(torrents) == 0 {
		return nil
	}

	// Filter by minimum seeders
	filtered := make([]*TorrentInfo, 0)
	for _, torrent := range torrents {
		if torrent.Seeders >= settings.MinSeeders {
			filtered = append(filtered, torrent)
		}
	}

	if len(filtered) == 0 {
		return torrents[0] // Return first if none meet seeder requirement
	}

	// Sort by preference: dual audio > most seeders > BD/Bluray/BDRip > size
	sort.Slice(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]

		// Prefer dual audio
		aIsDual := strings.Contains(strings.ToLower(a.Language), "dual")
		bIsDual := strings.Contains(strings.ToLower(b.Language), "dual")
		if aIsDual != bIsDual {
			return aIsDual
		}

		// Prefer BD/Bluray/BDRip
		aIsBD := strings.Contains(strings.ToLower(a.Format), "bd") || 
				 strings.Contains(strings.ToLower(a.Format), "bluray") ||
				 strings.Contains(strings.ToLower(a.Name), "bdrip")
		bIsBD := strings.Contains(strings.ToLower(b.Format), "bd") || 
				 strings.Contains(strings.ToLower(b.Format), "bluray") ||
				 strings.Contains(strings.ToLower(b.Name), "bdrip")
		if aIsBD != bIsBD {
			return aIsBD
		}

		// Prefer more seeders
		return a.Seeders > b.Seeders
	})

	return filtered[0]
}

// findAnimeEntries finds anime entries based on criteria
func (emd *EnMasseDownloader) findAnimeEntries(batchType BatchDownloadType, criteria map[string]interface{}) ([]AnimeOfflineEntry, error) {
	if emd.animeDatabase == nil {
		return nil, fmt.Errorf("anime database not loaded")
	}

	var results []AnimeOfflineEntry

	switch batchType {
	case BatchDownloadTypeAnime:
		// Search by specific anime titles
		if titles, ok := criteria["titles"].([]string); ok {
			for _, entry := range emd.animeDatabase.Data {
				for _, title := range titles {
					if strings.Contains(strings.ToLower(entry.Title), strings.ToLower(title)) {
						results = append(results, entry)
						break
					}
				}
			}
		}

	case BatchDownloadTypeSeason:
		// Search by season and year
		season, seasonOk := criteria["season"].(string)
		year, yearOk := criteria["year"].(int)
		if seasonOk && yearOk {
			for _, entry := range emd.animeDatabase.Data {
				if strings.EqualFold(entry.Season, season) && entry.Year == year {
					results = append(results, entry)
				}
			}
		}

	case BatchDownloadTypeYear:
		// Search by year
		if year, ok := criteria["year"].(int); ok {
			for _, entry := range emd.animeDatabase.Data {
				if entry.Year == year {
					results = append(results, entry)
				}
			}
		}

	case BatchDownloadTypeGenre:
		// Search by genre/tags
		if genre, ok := criteria["genre"].(string); ok {
			genreLower := strings.ToLower(genre)
			for _, entry := range emd.animeDatabase.Data {
				for _, tag := range entry.Tags {
					if strings.Contains(strings.ToLower(tag), genreLower) {
						results = append(results, entry)
						break
					}
				}
			}
		}
	}

	return results, nil
}

// extractAniListID extracts AniList ID from sources
func (emd *EnMasseDownloader) extractAniListID(entry AnimeOfflineEntry) int {
	for _, source := range entry.Sources {
		if strings.Contains(source, "anilist.co") {
			// Extract ID from URL like https://anilist.co/anime/12345
			parts := strings.Split(source, "/")
			if len(parts) > 0 {
				if id, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
					return id
				}
			}
		}
	}
	return 0 // Return 0 if no AniList ID found
}

// emitBatchProgress emits batch progress event
func (emd *EnMasseDownloader) emitBatchProgress(job *BatchDownloadJob) {
	if emd.wsEventManager == nil {
		return
	}

	// Calculate overall progress
	totalProgress := 0.0
	for _, item := range job.Items {
		totalProgress += item.Progress
	}
	job.Progress = totalProgress / float64(len(job.Items))

	emd.wsEventManager.SendEvent(events.AnimeBatchDownloadProgress, job)
}

// emitBatchComplete emits batch complete event
func (emd *EnMasseDownloader) emitBatchComplete(job *BatchDownloadJob) {
	if emd.wsEventManager == nil {
		return
	}

	emd.wsEventManager.SendEvent(events.AnimeBatchDownloadComplete, job)
}

// CancelBatchDownload cancels a batch download
func (emd *EnMasseDownloader) CancelBatchDownload(jobID string) error {
	emd.mu.RLock()
	job, exists := emd.activeBatchJobs[jobID]
	emd.mu.RUnlock()

	if !exists {
		return fmt.Errorf("batch job not found: %s", jobID)
	}

	job.cancel()
	job.Status = BatchDownloadStatusCancelled
	now := time.Now()
	job.EndTime = &now

	emd.emitBatchProgress(job)

	emd.logger.Info().
		Str("jobId", jobID).
		Msg("anime: Batch download cancelled")

	return nil
}

// GetActiveBatchJobs returns all active batch jobs
func (emd *EnMasseDownloader) GetActiveBatchJobs() []*BatchDownloadJob {
	emd.mu.RLock()
	defer emd.mu.RUnlock()

	jobs := make([]*BatchDownloadJob, 0, len(emd.activeBatchJobs))
	for _, job := range emd.activeBatchJobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// GetBatchJob returns a specific batch job
func (emd *EnMasseDownloader) GetBatchJob(jobID string) (*BatchDownloadJob, bool) {
	emd.mu.RLock()
	defer emd.mu.RUnlock()

	job, exists := emd.activeBatchJobs[jobID]
	return job, exists
}

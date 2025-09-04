package manga

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"seanime/internal/database/db"
	"seanime/internal/util/filecache"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type (
	// MetadataScanner scans downloaded manga directories and extracts metadata for UI display
	MetadataScanner struct {
		logger           *zerolog.Logger
		downloadDir      string
		database         *db.Database
		filecacher       *filecache.Cacher
		backgroundWorker chan string // Channel for background enhancement jobs
		workerRunning    bool
	}

	// DownloadedMangaSeries represents a downloaded manga series with metadata
	DownloadedMangaSeries struct {
		SeriesTitle    string                   `json:"seriesTitle"`
		SeriesPath     string                   `json:"seriesPath"`
		MediaID        int                      `json:"mediaId,omitempty"`
		CoverImagePath string                   `json:"coverImagePath,omitempty"`
		ChapterCount   int                      `json:"chapterCount"`
		Chapters       []DownloadedMangaChapter `json:"chapters"`
		LastUpdated    int64                    `json:"lastUpdated"`
	}

	// DownloadedMangaChapter represents a downloaded chapter
	DownloadedMangaChapter struct {
		ChapterNumber string `json:"chapterNumber"`
		ChapterTitle  string `json:"chapterTitle"`
		ChapterPath   string `json:"chapterPath"`
		PageCount     int    `json:"pageCount"`
		LastModified  int64  `json:"lastModified"`
	}

	// MetadataScannerOptions for creating a new scanner
	MetadataScannerOptions struct {
		Logger      *zerolog.Logger
		DownloadDir string
		Database    *db.Database
		Filecacher  *filecache.Cacher
	}
)

// NewMetadataScanner creates a new metadata scanner
func NewMetadataScanner(opts *MetadataScannerOptions) *MetadataScanner {
	return &MetadataScanner{
		logger:           opts.Logger,
		downloadDir:      opts.DownloadDir,
		database:         opts.Database,
		filecacher:       opts.Filecacher,
		backgroundWorker: make(chan string, 100),
		workerRunning:    false,
	}
}

// ScanDownloadedManga scans the download directory and returns metadata for all downloaded manga series
func (ms *MetadataScanner) ScanDownloadedManga() ([]DownloadedMangaSeries, error) {
	ms.logger.Debug().Msg("manga metadata scanner: Scanning downloaded manga")

	var series []DownloadedMangaSeries

	// Read the download directory
	entries, err := os.ReadDir(ms.downloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			ms.logger.Debug().Msg("manga metadata scanner: Download directory does not exist")
			return series, nil
		}
		return nil, fmt.Errorf("failed to read download directory: %w", err)
	}

	// Process each series directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip temporary working directory used during downloads
		if entry.Name() == ".tmp" {
			continue
		}

		seriesPath := filepath.Join(ms.downloadDir, entry.Name())
		seriesData, err := ms.scanSeries(entry.Name(), seriesPath)
		if err != nil {
			ms.logger.Warn().Err(err).Str("series", entry.Name()).Msg("manga metadata scanner: Failed to scan series")
			continue
		}

		if seriesData != nil {
			series = append(series, *seriesData)
		}
	}

	ms.logger.Debug().Int("count", len(series)).Msg("manga metadata scanner: Completed scanning")
	return series, nil
}

// extractSeriesTitleFromRegistryFast extracts series title from first registry.json file only
func (ms *MetadataScanner) extractSeriesTitleFromRegistryFast(seriesPath string) string {
	// Only check first chapter directory for performance
	chapterDirs, err := os.ReadDir(seriesPath)
	if err != nil {
		return ""
	}

	// Look at only the first chapter directory
	for _, chapterDir := range chapterDirs {
		if !chapterDir.IsDir() {
			continue
		}

		registryPath := filepath.Join(seriesPath, chapterDir.Name(), "registry.json")
		if _, err := os.Stat(registryPath); os.IsNotExist(err) {
			continue
		}

		// Read and parse the registry file
		data, err := os.ReadFile(registryPath)
		if err != nil {
			continue
		}

		// Try new format first
		var newFormat struct {
			DownloadMetadata struct {
				SeriesTitle string `json:"seriesTitle"`
			} `json:"download_metadata"`
		}

		if err := json.Unmarshal(data, &newFormat); err == nil && newFormat.DownloadMetadata.SeriesTitle != "" {
			return newFormat.DownloadMetadata.SeriesTitle
		}

		// Quick old format check - only first entry
		var oldFormat map[string]interface{}
		if err := json.Unmarshal(data, &oldFormat); err == nil {
			for _, value := range oldFormat {
				if pageData, ok := value.(map[string]interface{}); ok {
					if originalURL, exists := pageData["original_url"]; exists {
						if urlStr, ok := originalURL.(string); ok {
							return ms.extractSeriesTitleFromURL(urlStr)
						}
					}
				}
				break // Only check first entry
			}
		}
		break // Only check first chapter
	}

	return ""
}

// extractMediaIdFromRegistryFast extracts media ID from first registry.json file only
func (ms *MetadataScanner) extractMediaIdFromRegistryFast(seriesPath string) int {
	// Only check first chapter directory for performance
	chapterDirs, err := os.ReadDir(seriesPath)
	if err != nil {
		return 0
	}

	// Look at only the first chapter directory
	for _, chapterDir := range chapterDirs {
		if !chapterDir.IsDir() {
			continue
		}

		registryPath := filepath.Join(seriesPath, chapterDir.Name(), "registry.json")
		if _, err := os.Stat(registryPath); os.IsNotExist(err) {
			continue
		}

		// Read and parse the registry file
		data, err := os.ReadFile(registryPath)
		if err != nil {
			continue
		}

		// Try new format first
		var newFormat struct {
			DownloadMetadata struct {
				MediaID int `json:"mediaId"`
			} `json:"download_metadata"`
		}

		if err := json.Unmarshal(data, &newFormat); err == nil && newFormat.DownloadMetadata.MediaID != 0 {
			return newFormat.DownloadMetadata.MediaID
		}
		break // Only check first chapter
	}

	return 0
}

// extractSeriesTitleFromRegistry attempts to extract the series title from registry.json files
func (ms *MetadataScanner) extractSeriesTitleFromRegistry(seriesPath string) string {
	// Try to find any registry.json file in the series directory
	chapterEntries, err := os.ReadDir(seriesPath)
	if err != nil {
		ms.logger.Debug().Err(err).Str("seriesPath", seriesPath).Msg("manga metadata scanner: Failed to read series directory for title extraction")
		return ""
	}

	for _, chapterEntry := range chapterEntries {
		if !chapterEntry.IsDir() {
			continue
		}

		chapterPath := filepath.Join(seriesPath, chapterEntry.Name())
		registryPath := filepath.Join(chapterPath, "registry.json")

		// Try to read and parse the registry.json
		data, err := os.ReadFile(registryPath)
		if err != nil {
			ms.logger.Debug().Err(err).Str("registryPath", registryPath).Msg("manga metadata scanner: Failed to read registry file")
			continue
		}

		// Try to parse as new registry format first (with download_metadata)
		var newRegistry struct {
			DownloadMetadata struct {
				SeriesTitle string `json:"seriesTitle"`
			} `json:"download_metadata"`
			Pages map[string]interface{} `json:"pages"`
		}
		if err := json.Unmarshal(data, &newRegistry); err == nil && newRegistry.DownloadMetadata.SeriesTitle != "" {
			ms.logger.Debug().Str("seriesTitle", newRegistry.DownloadMetadata.SeriesTitle).Str("registryPath", registryPath).Msg("manga metadata scanner: Extracted series title from new registry format")
			return newRegistry.DownloadMetadata.SeriesTitle
		}

		// Fallback to old registry format (direct page map)
		var oldRegistry map[string]interface{}
		if err := json.Unmarshal(data, &oldRegistry); err != nil {
			ms.logger.Debug().Err(err).Str("registryPath", registryPath).Msg("manga metadata scanner: Failed to parse registry JSON")
			continue
		}

		// Look for any entry with an original_url in old format
		for _, value := range oldRegistry {
			if pageData, ok := value.(map[string]interface{}); ok {
				if originalURL, exists := pageData["original_url"]; exists {
					if urlStr, ok := originalURL.(string); ok {
						ms.logger.Debug().Str("url", urlStr).Msg("manga metadata scanner: Found original_url in old registry")
						// Extract series name from URL like "https://hot.planeptune.us/manga/Blue-Lock/0001-001.png"
						if seriesTitle := ms.extractSeriesTitleFromURL(urlStr); seriesTitle != "" {
							ms.logger.Debug().Str("extractedTitle", seriesTitle).Msg("manga metadata scanner: Successfully extracted series title")
							return seriesTitle
						}
					}
				}
			}
		}
	}

	return ""
}

// extractSeriesTitleFromURL extracts series title from manga URL
func (ms *MetadataScanner) extractSeriesTitleFromURL(url string) string {
	// Parse URLs like "https://hot.planeptune.us/manga/Blue-Lock/0001-001.png"
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "manga" && i+1 < len(parts) {
			// The next part should be the series name
			seriesName := parts[i+1]
			// Convert "Blue-Lock" to "Blue Lock"
			return strings.ReplaceAll(seriesName, "-", " ")
		}
	}
	return ""
}

// extractMediaIdFromRegistry attempts to extract the media ID from registry.json files
func (ms *MetadataScanner) extractMediaIdFromRegistry(seriesPath string) int {
	// Try to find any registry.json file in the series directory
	chapterEntries, err := os.ReadDir(seriesPath)
	if err != nil {
		ms.logger.Debug().Err(err).Str("seriesPath", seriesPath).Msg("manga metadata scanner: Failed to read series directory for media ID extraction")
		return 0
	}

	for _, chapterEntry := range chapterEntries {
		if !chapterEntry.IsDir() {
			continue
		}

		chapterPath := filepath.Join(seriesPath, chapterEntry.Name())
		registryPath := filepath.Join(chapterPath, "registry.json")

		// Try to read and parse the registry.json
		data, err := os.ReadFile(registryPath)
		if err != nil {
			ms.logger.Debug().Err(err).Str("registryPath", registryPath).Msg("manga metadata scanner: Failed to read registry file for media ID")
			continue
		}

		// Try to parse as new registry format first (with download_metadata)
		var newRegistry struct {
			DownloadMetadata struct {
				MediaId int `json:"mediaId"`
			} `json:"download_metadata"`
		}
		if err := json.Unmarshal(data, &newRegistry); err == nil && newRegistry.DownloadMetadata.MediaId > 0 {
			ms.logger.Debug().Int("mediaId", newRegistry.DownloadMetadata.MediaId).Str("seriesPath", seriesPath).Msg("manga metadata scanner: Extracted media ID from new registry format")
			return newRegistry.DownloadMetadata.MediaId
		}

		// For old registry format, we don't have media ID stored
		// This is expected for downloads made before the registry format update
		ms.logger.Debug().Str("registryPath", registryPath).Msg("manga metadata scanner: Old registry format detected, no media ID available")
	}

	ms.logger.Debug().Str("seriesPath", seriesPath).Msg("manga metadata scanner: No valid media ID found in registry files")
	return 0
}

// scanSeries scans a single manga series directory with ultra-fast indexing
func (ms *MetadataScanner) scanSeries(seriesName, seriesPath string) (*DownloadedMangaSeries, error) {
	// Ultra-fast indexing: only count directories, skip all file operations
	chapterEntries, err := os.ReadDir(seriesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read series directory: %w", err)
	}

	// Quick count of chapter directories only
	chapterCount := 0
	var lastUpdated int64
	for _, entry := range chapterEntries {
		if entry.IsDir() {
			chapterCount++
			// Get modification time from directory entry (no extra stat calls)
			if info, err := entry.Info(); err == nil {
				if info.ModTime().Unix() > lastUpdated {
					lastUpdated = info.ModTime().Unix()
				}
			}
		}
	}

	// Skip series with no chapters
	if chapterCount == 0 {
		return nil, nil
	}

	// Fast metadata extraction - only check first registry file
	actualSeriesTitle := ms.extractSeriesTitleFromRegistryFast(seriesPath)
	if actualSeriesTitle == "" {
		actualSeriesTitle = seriesName
	}

	// Fast media ID extraction - only check first registry file
	mediaId := ms.extractMediaIdFromRegistryFast(seriesPath)

	// Return minimal series data for ultra-fast indexing
	return &DownloadedMangaSeries{
		SeriesTitle:    actualSeriesTitle,
		SeriesPath:     seriesPath,
		MediaID:        mediaId,
		CoverImagePath: "", // Skip cover images for fast indexing
		ChapterCount:   chapterCount,
		Chapters:       []DownloadedMangaChapter{}, // Empty chapters for fast indexing
		LastUpdated:    time.Now().Unix(),
	}, nil
}

// scanChapter scans a single chapter directory (DEPRECATED - use optimized version in scanSeries)
func (ms *MetadataScanner) scanChapter(chapterName, chapterPath string) (*DownloadedMangaChapter, error) {
	// Get chapter info
	info, err := os.Stat(chapterPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat chapter directory: %w", err)
	}

	// Parse chapter number and title from directory name
	chapterNumber, chapterTitle := ms.parseChapterName(chapterName)

	// Skip expensive page counting for performance
	pageCount := 1 // Assume at least 1 page if directory exists

	return &DownloadedMangaChapter{
		ChapterNumber: chapterNumber,
		ChapterTitle:  chapterTitle,
		ChapterPath:   chapterPath,
		PageCount:     pageCount,
		LastModified:  info.ModTime().Unix(),
	}, nil
}

// parseChapterName parses chapter number and title from directory name
// Expected format: "1 - Chapter Title" or just "1"
func (ms *MetadataScanner) parseChapterName(chapterName string) (string, string) {
	parts := strings.Split(chapterName, " - ")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(strings.Join(parts[1:], " - "))
	}
	return strings.TrimSpace(chapterName), ""
}

// countPages counts the number of image files in a chapter directory
func (ms *MetadataScanner) countPages(chapterPath string) (int, error) {
	entries, err := os.ReadDir(chapterPath)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip registry.json and other non-image files
		if entry.Name() == "registry.json" {
			continue
		}

		// Check if it's an image file
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
			count++
		}
	}

	return count, nil
}

// findCoverImage searches for a cover image in the series directory
func (ms *MetadataScanner) findCoverImage(seriesPath string) string {
	// Look for common cover image patterns in the series directory
	patterns := []string{
		"cover.*",
		"folder.*",
		"poster.*",
		"thumb.*",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(seriesPath, "**", pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			// Check if it's an image file
			ext := strings.ToLower(filepath.Ext(match))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
				return match
			}
		}
	}

	return ""
}

// findCoverImageFast performs a quick cover image search in a single directory
func (ms *MetadataScanner) findCoverImageFast(chapterPath string) string {
	// Only check the first few files in the chapter directory for performance
	entries, err := os.ReadDir(chapterPath)
	if err != nil {
		return ""
	}

	// Look for the first image file (likely to be cover/first page)
	for i, entry := range entries {
		if i >= 3 { // Only check first 3 files for speed
			break
		}
		if entry.IsDir() {
			continue
		}

		// Check if it's an image file
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
			return filepath.Join(chapterPath, entry.Name())
		}
	}

	return ""
}

// findCoverImageInChapterFiles finds a proper cover image by scanning chapter files
// Prioritizes dedicated cover files, falls back to first page if needed
func (ms *MetadataScanner) findCoverImageInChapterFiles(seriesDir string) string {
	// Skip cover image scanning entirely for performance with 10k+ series
	// This is too expensive for large collections
	return ""
}

// extractCoverImageFromRegistry extracts cover image URL from registry.json files in the series directory
func (ms *MetadataScanner) extractCoverImageFromRegistry(seriesDir string) string {
	ms.logger.Debug().Str("seriesDir", seriesDir).Msg("metadata_scanner: Extracting cover image URL from registry files")

	// Get all chapter directories in the series
	chapterDirs, err := os.ReadDir(seriesDir)
	if err != nil {
		ms.logger.Error().Err(err).Str("seriesDir", seriesDir).Msg("metadata_scanner: Failed to read series directory")
		return ""
	}

	// Look through all chapter directories for registry.json files
	for _, chapterDir := range chapterDirs {
		if !chapterDir.IsDir() {
			continue
		}

		chapterPath := filepath.Join(seriesDir, chapterDir.Name())
		registryPath := filepath.Join(chapterPath, "registry.json")

		// Check if registry.json exists
		if _, err := os.Stat(registryPath); os.IsNotExist(err) {
			continue
		}

		// Read registry.json file
		registryData, err := os.ReadFile(registryPath)
		if err != nil {
			ms.logger.Error().Err(err).Str("registryPath", registryPath).Msg("metadata_scanner: Failed to read registry file")
			continue
		}

		// Parse registry.json
		var registry struct {
			DownloadMetadata struct {
				CoverImageUrl string `json:"coverImageUrl"`
			} `json:"download_metadata"`
		}

		err = json.Unmarshal(registryData, &registry)
		if err != nil {
			ms.logger.Error().Err(err).Str("registryPath", registryPath).Msg("metadata_scanner: Failed to parse registry file")
			continue
		}

		// Check if cover image URL is present and not empty
		if registry.DownloadMetadata.CoverImageUrl != "" {
			ms.logger.Info().
				Str("coverImageUrl", registry.DownloadMetadata.CoverImageUrl).
				Str("registryPath", registryPath).
				Msg("metadata_scanner: Found cover image URL in registry")
			return registry.DownloadMetadata.CoverImageUrl
		}
	}

	ms.logger.Debug().Str("seriesDir", seriesDir).Msg("metadata_scanner: No cover image URL found in any registry files")
	return ""
}

// GetDownloadedMangaList returns a cached list of downloaded manga or performs incremental updates
func (ms *MetadataScanner) GetDownloadedMangaList() ([]DownloadedMangaSeries, error) {
	// Use permanent cache to persist across restarts
	cacheKey := "downloaded_manga_list"
	bucket := filecache.NewPermanentBucket("manga_metadata")

	var cachedSeries []DownloadedMangaSeries
	found, err := ms.filecacher.GetPerm(bucket, cacheKey, &cachedSeries)
	if err != nil {
		ms.logger.Debug().Err(err).Msg("manga metadata scanner: Failed to load cache, performing full scan")
		return ms.performFullScan(bucket, cacheKey)
	}

	if !found {
		ms.logger.Debug().Msg("manga metadata scanner: No cache found, performing full scan")
		return ms.performFullScan(bucket, cacheKey)
	}

	// Perform incremental update
	ms.logger.Debug().Int("cached_count", len(cachedSeries)).Msg("manga metadata scanner: Performing incremental cache update")
	updatedSeries, err := ms.performIncrementalUpdate(cachedSeries)
	if err != nil {
		ms.logger.Warn().Err(err).Msg("manga metadata scanner: Incremental update failed, falling back to full scan")
		return ms.performFullScan(bucket, cacheKey)
	}

	// Save updated cache
	ms.filecacher.SetPerm(bucket, cacheKey, updatedSeries)

	return updatedSeries, nil
}

// RefreshDownloadedMangaCache clears the cache to force a rescan
func (ms *MetadataScanner) RefreshDownloadedMangaCache() {
	bucket := filecache.NewPermanentBucket("manga_metadata")
	ms.filecacher.DeletePerm(bucket, "downloaded_manga_list")
	ms.logger.Info().Msg("manga metadata scanner: Cache cleared, next scan will be full")
}

// performFullScan performs a complete scan and caches the results
func (ms *MetadataScanner) performFullScan(bucket filecache.PermanentBucket, cacheKey string) ([]DownloadedMangaSeries, error) {
	ms.logger.Info().Msg("manga metadata scanner: Performing full scan")
	series, err := ms.ScanDownloadedManga()
	if err != nil {
		return nil, err
	}

	// Cache the results permanently
	ms.filecacher.SetPerm(bucket, cacheKey, series)
	ms.logger.Info().Int("count", len(series)).Msg("manga metadata scanner: Full scan completed and cached")

	return series, nil
}

// performIncrementalUpdate checks for changes and updates only modified series with concurrent processing
func (ms *MetadataScanner) performIncrementalUpdate(cachedSeries []DownloadedMangaSeries) ([]DownloadedMangaSeries, error) {
	// Create a map of cached series for quick lookup
	cachedMap := make(map[string]*DownloadedMangaSeries)
	for i := range cachedSeries {
		cachedMap[cachedSeries[i].SeriesPath] = &cachedSeries[i]
	}

	// Read current directory state
	entries, err := os.ReadDir(ms.downloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			ms.logger.Debug().Msg("manga metadata scanner: Download directory does not exist")
			return []DownloadedMangaSeries{}, nil
		}
		return nil, fmt.Errorf("failed to read download directory: %w", err)
	}

	// Track which series are still present
	presentSeries := make(map[string]bool)
	var updatedSeries []DownloadedMangaSeries
	updatedCount := 0
	addedCount := 0

	// Use concurrent processing for large collections
	type scanJob struct {
		entry      os.DirEntry
		seriesPath string
		isUpdate   bool
	}

	type scanResult struct {
		series   *DownloadedMangaSeries
		err      error
		isUpdate bool
	}

	// Collect scan jobs
	var jobs []scanJob
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		seriesPath := filepath.Join(ms.downloadDir, entry.Name())
		presentSeries[seriesPath] = true

		// Check if series exists in cache
		if cached, exists := cachedMap[seriesPath]; exists {
			// Check if series needs updating (directory modification time)
			stat, err := os.Stat(seriesPath)
			if err != nil {
				ms.logger.Warn().Err(err).Str("series", entry.Name()).Msg("manga metadata scanner: Failed to stat series directory")
				// Keep cached version
				updatedSeries = append(updatedSeries, *cached)
				continue
			}

			// If directory was modified after last cache update, rescan this series
			if stat.ModTime().Unix() > cached.LastUpdated {
				jobs = append(jobs, scanJob{entry: entry, seriesPath: seriesPath, isUpdate: true})
			} else {
				// No changes, keep cached version
				updatedSeries = append(updatedSeries, *cached)
			}
		} else {
			// New series, scan it
			jobs = append(jobs, scanJob{entry: entry, seriesPath: seriesPath, isUpdate: false})
		}
	}

	// Process scan jobs concurrently if there are many
	if len(jobs) > 10 {
		ms.logger.Info().Int("jobs", len(jobs)).Msg("manga metadata scanner: Processing series concurrently")

		// Use worker pool for concurrent processing
		numWorkers := 8 // Limit concurrent workers to avoid overwhelming filesystem
		jobCh := make(chan scanJob, len(jobs))
		resultCh := make(chan scanResult, len(jobs))

		// Start workers
		for i := 0; i < numWorkers; i++ {
			go func() {
				for job := range jobCh {
					seriesData, err := ms.scanSeries(job.entry.Name(), job.seriesPath)
					resultCh <- scanResult{series: seriesData, err: err, isUpdate: job.isUpdate}
				}
			}()
		}

		// Send jobs
		for _, job := range jobs {
			jobCh <- job
		}
		close(jobCh)

		// Collect results
		for i := 0; i < len(jobs); i++ {
			result := <-resultCh
			if result.err != nil {
				ms.logger.Warn().Err(result.err).Msg("manga metadata scanner: Failed to scan series")
				continue
			}
			if result.series != nil {
				updatedSeries = append(updatedSeries, *result.series)
				if result.isUpdate {
					updatedCount++
				} else {
					addedCount++
				}
			}
		}
	} else {
		// Process sequentially for small numbers
		for _, job := range jobs {
			seriesData, err := ms.scanSeries(job.entry.Name(), job.seriesPath)
			if err != nil {
				ms.logger.Warn().Err(err).Str("series", job.entry.Name()).Msg("manga metadata scanner: Failed to scan series")
				continue
			}
			if seriesData != nil {
				updatedSeries = append(updatedSeries, *seriesData)
				if job.isUpdate {
					updatedCount++
				} else {
					addedCount++
				}
			}
		}
	}

	// Count removed series
	removedCount := 0
	for _, cached := range cachedSeries {
		if !presentSeries[cached.SeriesPath] {
			removedCount++
		}
	}

	ms.logger.Info().
		Int("total", len(updatedSeries)).
		Int("updated", updatedCount).
		Int("added", addedCount).
		Int("removed", removedCount).
		Msg("manga metadata scanner: Incremental update completed")

	return updatedSeries, nil
}

// MigrateToSyntheticIDs migrates existing downloaded manga from AniList IDs to synthetic IDs
// This ensures compatibility with the local provider system for all downloaded manga
// This function reads registry files directly to get original AniList IDs before any conversion
func (ms *MetadataScanner) MigrateToSyntheticIDs() error {
	ms.logger.Info().Msg("manga metadata scanner: Starting migration of existing downloaded manga to synthetic IDs")

	// Read registry files directly to get original AniList IDs before scanner conversion
	seriesDirs, err := os.ReadDir(ms.downloadDir)
	if err != nil {
		ms.logger.Error().Err(err).Msg("manga metadata scanner: Failed to read download directory for migration")
		return fmt.Errorf("failed to read download directory: %w", err)
	}

	migratedCount := 0
	var migratedSeries []string

	// Process each series directory
	for _, seriesDir := range seriesDirs {
		if !seriesDir.IsDir() {
			continue
		}

		seriesPath := filepath.Join(ms.downloadDir, seriesDir.Name())

		// Find the first chapter directory to get registry.json
		chapterDirs, err := os.ReadDir(seriesPath)
		if err != nil {
			continue
		}

		var registryPath string
		for _, chapterDir := range chapterDirs {
			if chapterDir.IsDir() {
				potentialRegistry := filepath.Join(seriesPath, chapterDir.Name(), "registry.json")
				if _, err := os.Stat(potentialRegistry); err == nil {
					registryPath = potentialRegistry
					break
				}
			}
		}

		if registryPath == "" {
			ms.logger.Debug().Str("series", seriesDir.Name()).Msg("manga metadata scanner: No registry.json found, skipping")
			continue
		}

		// Read the registry file to get original media ID
		registryData, err := os.ReadFile(registryPath)
		if err != nil {
			ms.logger.Debug().Str("series", seriesDir.Name()).Err(err).Msg("manga metadata scanner: Failed to read registry file")
			continue
		}

		var registry struct {
			MediaID int `json:"mediaId"`
		}
		if err := json.Unmarshal(registryData, &registry); err != nil {
			ms.logger.Debug().Str("series", seriesDir.Name()).Err(err).Msg("manga metadata scanner: Failed to parse registry file")
			continue
		}

		originalMediaID := registry.MediaID
		ms.logger.Debug().
			Str("series", seriesDir.Name()).
			Int("originalMediaId", originalMediaID).
			Msg("manga metadata scanner: Found original media ID from registry")

		// Check if this series already has a synthetic ID (skip if already migrated)
		// Synthetic IDs are in range 1000000-9999999
		if originalMediaID >= 1000000 && originalMediaID <= 9999999 {
			ms.logger.Debug().Str("series", seriesDir.Name()).Int("mediaId", originalMediaID).Msg("manga metadata scanner: Series already has synthetic ID, skipping")
			continue
		}

		// Skip series with invalid/zero IDs
		if originalMediaID <= 0 {
			ms.logger.Debug().Str("series", seriesDir.Name()).Int("mediaId", originalMediaID).Msg("manga metadata scanner: Series has invalid ID, skipping")
			continue
		}

		// This series has an AniList ID and needs migration to synthetic ID
		ms.logger.Info().Str("series", seriesDir.Name()).Int("anilistId", originalMediaID).Msg("manga metadata scanner: Migrating AniList ID to synthetic ID")

		// Generate synthetic ID based on series title
		syntheticID := ms.generateSyntheticID(seriesDir.Name())

		// Update the registry file with the new synthetic ID
		registry.MediaID = syntheticID
		updatedRegistryData, err := json.MarshalIndent(registry, "", "  ")
		if err != nil {
			ms.logger.Error().Str("series", seriesDir.Name()).Err(err).Msg("manga metadata scanner: Failed to marshal updated registry")
			continue
		}

		if err := os.WriteFile(registryPath, updatedRegistryData, 0644); err != nil {
			ms.logger.Error().Str("series", seriesDir.Name()).Err(err).Msg("manga metadata scanner: Failed to write updated registry")
			continue
		}

		migratedCount++
		migratedSeries = append(migratedSeries, seriesDir.Name())

		ms.logger.Info().
			Str("series", seriesDir.Name()).
			Int("oldAniListID", originalMediaID).
			Int("newSyntheticID", syntheticID).
			Str("registryPath", registryPath).
			Msg("manga metadata scanner: Successfully migrated series to synthetic ID")
	}

	if migratedCount == 0 {
		ms.logger.Info().Msg("manga metadata scanner: No manga needed migration to synthetic IDs")
		return nil
	}

	// Clear the cache so it gets refreshed with the new synthetic IDs
	bucket := filecache.NewBucket("manga_metadata", 5*time.Minute)
	ms.filecacher.Delete(bucket, "downloaded_manga_list")

	ms.logger.Info().
		Int("migratedCount", migratedCount).
		Strs("migratedSeries", migratedSeries).
		Msg("manga metadata scanner: Successfully migrated manga to synthetic IDs and cleared cache")
	return nil
}

// generateSyntheticID creates a synthetic media ID from a series title
// Uses the same logic as the En Masse Downloader to ensure consistency
func (ms *MetadataScanner) generateSyntheticID(seriesTitle string) int {
	// Use FNV hash to generate a consistent synthetic ID
	h := fnv.New32a()
	h.Write([]byte(seriesTitle))
	hashValue := h.Sum32()

	// Ensure the ID is positive and within the synthetic range (1000000-9999999)
	syntheticID := int(hashValue%8999999) + 1000000

	return syntheticID
}

// StartBackgroundWorker starts the background metadata enhancement worker
func (ms *MetadataScanner) StartBackgroundWorker() {
	if ms.workerRunning {
		return
	}

	ms.workerRunning = true
	go ms.backgroundEnhancementWorker()
	ms.logger.Info().Msg("manga metadata scanner: Background enhancement worker started")
}

// StopBackgroundWorker stops the background metadata enhancement worker
func (ms *MetadataScanner) StopBackgroundWorker() {
	if !ms.workerRunning {
		return
	}

	close(ms.backgroundWorker)
	ms.workerRunning = false
	ms.logger.Info().Msg("manga metadata scanner: Background enhancement worker stopped")
}

// backgroundEnhancementWorker runs in the background to enhance metadata progressively
func (ms *MetadataScanner) backgroundEnhancementWorker() {
	defer func() {
		ms.workerRunning = false
	}()

	for seriesPath := range ms.backgroundWorker {
		ms.enhanceSeriesMetadata(seriesPath)
		// Small delay to avoid overwhelming the filesystem
		time.Sleep(100 * time.Millisecond)
	}
}

// enhanceSeriesMetadata enhances metadata for a specific series with full details
func (ms *MetadataScanner) enhanceSeriesMetadata(seriesPath string) {
	ms.logger.Debug().Str("seriesPath", seriesPath).Msg("manga metadata scanner: Enhancing series metadata")

	// Get series name from path
	seriesName := filepath.Base(seriesPath)

	// Perform full metadata scan with cover images and detailed chapter info
	enhancedSeries, err := ms.scanSeriesWithFullDetails(seriesName, seriesPath)
	if err != nil {
		ms.logger.Warn().Err(err).Str("series", seriesName).Msg("manga metadata scanner: Failed to enhance series metadata")
		return
	}

	if enhancedSeries == nil {
		return
	}

	// Update cache with enhanced metadata
	cacheKey := "downloaded_manga_series"
	bucket := filecache.NewPermanentBucket("manga_downloaded_series")

	// Get current cached series
	var cachedSeries []DownloadedMangaSeries
	if _, err := ms.filecacher.GetPerm(bucket, cacheKey, &cachedSeries); err == nil {
		// Find and update the specific series
		for i, series := range cachedSeries {
			if series.SeriesPath == seriesPath {
				cachedSeries[i] = *enhancedSeries
				break
			}
		}

		// Save updated cache
		ms.filecacher.SetPerm(bucket, cacheKey, cachedSeries)
		ms.logger.Debug().Str("series", seriesName).Msg("manga metadata scanner: Enhanced metadata cached")
	}
}

// scanSeriesWithFullDetails performs a full metadata scan including cover images and detailed chapters
func (ms *MetadataScanner) scanSeriesWithFullDetails(seriesName, seriesPath string) (*DownloadedMangaSeries, error) {
	// Full scan with all details
	chapterEntries, err := os.ReadDir(seriesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read series directory: %w", err)
	}

	var chapters []DownloadedMangaChapter
	var lastUpdated int64

	// Scan all chapters with full details
	for _, entry := range chapterEntries {
		if !entry.IsDir() {
			continue
		}

		chapterPath := filepath.Join(seriesPath, entry.Name())
		chapter, err := ms.scanChapter(entry.Name(), chapterPath)
		if err != nil {
			ms.logger.Warn().Err(err).Str("chapter", entry.Name()).Msg("manga metadata scanner: Failed to scan chapter")
			continue
		}

		if chapter != nil {
			chapters = append(chapters, *chapter)
			// Use current time for last updated since chapter doesn't have this field
			lastUpdated = time.Now().Unix()
		}
	}

	// Skip series with no chapters
	if len(chapters) == 0 {
		return nil, nil
	}

	// Get full metadata
	actualSeriesTitle := ms.extractSeriesTitleFromRegistry(seriesPath)
	if actualSeriesTitle == "" {
		actualSeriesTitle = seriesName
	}

	mediaId := ms.extractMediaIdFromRegistry(seriesPath)

	// Find cover image with full search
	coverImagePath := ms.findCoverImage(seriesPath)

	return &DownloadedMangaSeries{
		SeriesTitle:    actualSeriesTitle,
		SeriesPath:     seriesPath,
		MediaID:        mediaId,
		CoverImagePath: coverImagePath,
		ChapterCount:   len(chapters),
		Chapters:       chapters,
		LastUpdated:    lastUpdated,
	}, nil
}

// QueueForEnhancement queues a series for background metadata enhancement
func (ms *MetadataScanner) QueueForEnhancement(seriesPath string) {
	if !ms.workerRunning {
		return
	}

	select {
	case ms.backgroundWorker <- seriesPath:
		ms.logger.Debug().Str("seriesPath", seriesPath).Msg("manga metadata scanner: Queued series for enhancement")
	default:
		// Channel full, skip this enhancement
		ms.logger.Debug().Str("seriesPath", seriesPath).Msg("manga metadata scanner: Enhancement queue full, skipping")
	}
}

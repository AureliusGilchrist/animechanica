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
		logger      *zerolog.Logger
		downloadDir string
		database    *db.Database
		filecacher  *filecache.Cacher
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
		logger:      opts.Logger,
		downloadDir: opts.DownloadDir,
		database:    opts.Database,
		filecacher:  opts.Filecacher,
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

// scanSeries scans a single manga series directory
func (ms *MetadataScanner) scanSeries(seriesName, seriesPath string) (*DownloadedMangaSeries, error) {
	// Read chapters in the series directory
	chapterEntries, err := os.ReadDir(seriesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read series directory: %w", err)
	}

	// Quick check: if no entries at all, skip this series
	if len(chapterEntries) == 0 {
		ms.logger.Debug().Str("series", seriesName).Msg("manga metadata scanner: Skipping empty series directory")
		return nil, nil
	}

	var chapters []DownloadedMangaChapter
	var coverImagePath string
	var lastUpdated int64

	// Process each chapter directory
	for _, chapterEntry := range chapterEntries {
		if !chapterEntry.IsDir() {
			continue
		}

		chapterPath := filepath.Join(seriesPath, chapterEntry.Name())
		chapterData, err := ms.scanChapter(chapterEntry.Name(), chapterPath)
		if err != nil {
			ms.logger.Warn().Err(err).Str("chapter", chapterEntry.Name()).Msg("manga metadata scanner: Failed to scan chapter")
			continue
		}

		// Only include chapters that have actual content (pages)
		if chapterData != nil && chapterData.PageCount > 0 {
			chapters = append(chapters, *chapterData)

			// Update last modified time
			if chapterData.LastModified > lastUpdated {
				lastUpdated = chapterData.LastModified
			}

			// Try to find a cover image from the series if we don't have one yet
			if coverImagePath == "" {
				absoluteCoverPath := ms.findCoverImage(seriesPath)
				if absoluteCoverPath != "" {
					// Convert absolute path to relative path from download directory
					if relPath, err := filepath.Rel(ms.downloadDir, absoluteCoverPath); err == nil {
						coverImagePath = relPath
						ms.logger.Debug().Str("coverPath", coverImagePath).Str("series", seriesName).Msg("manga metadata scanner: Found cover image")
					}
				}
			}
		}
	}

	// Skip series with no valid chapters (chapters must have content/pages)
	if len(chapters) == 0 {
		ms.logger.Debug().Str("series", seriesName).Msg("manga metadata scanner: Skipping series with no valid chapters")
		return nil, nil
	}

	// Try to extract the actual series title from registry files
	actualSeriesTitle := ms.extractSeriesTitleFromRegistry(seriesPath)
	if actualSeriesTitle == "" {
		// Fallback to directory name if we can't extract from registry
		actualSeriesTitle = seriesName
	}

	// Try to extract the media ID from registry files
	mediaId := ms.extractMediaIdFromRegistry(seriesPath)

	return &DownloadedMangaSeries{
		SeriesTitle:    actualSeriesTitle,
		SeriesPath:     seriesPath,
		MediaID:        mediaId,
		CoverImagePath: coverImagePath,
		ChapterCount:   len(chapters),
		Chapters:       chapters,
		LastUpdated:    time.Now().Unix(),
	}, nil
}

// scanChapter scans a single chapter directory
func (ms *MetadataScanner) scanChapter(chapterName, chapterPath string) (*DownloadedMangaChapter, error) {
	// Get chapter info
	info, err := os.Stat(chapterPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat chapter directory: %w", err)
	}

	// Parse chapter number and title from directory name
	chapterNumber, chapterTitle := ms.parseChapterName(chapterName)

	// Count pages in the chapter
	pageCount, err := ms.countPages(chapterPath)
	if err != nil {
		ms.logger.Warn().Err(err).Str("chapter", chapterName).Msg("manga metadata scanner: Failed to count pages")
		pageCount = 0
	}

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

// findCoverImage finds a proper cover image for a series
// Priority: registry cover URL > dedicated cover files > first page fallback
func (ms *MetadataScanner) findCoverImage(seriesDir string) string {
	// First, try to get cover image URL from registry.json files
	coverImageUrl := ms.extractCoverImageFromRegistry(seriesDir)
	if coverImageUrl != "" {
		ms.logger.Info().Str("coverImageUrl", coverImageUrl).Msg("metadata_scanner: Found cover image URL from registry metadata")
		return coverImageUrl
	}

	// Fallback to scanning chapter files if no cover URL found in registry
	ms.logger.Debug().Str("seriesDir", seriesDir).Msg("metadata_scanner: No cover URL in registry, scanning chapter files")
	return ms.findCoverImageInChapterFiles(seriesDir)
}

// findCoverImageInChapterFiles finds a proper cover image by scanning chapter files
// Prioritizes dedicated cover files, falls back to first page if needed
func (ms *MetadataScanner) findCoverImageInChapterFiles(seriesDir string) string {
	// Get all chapter directories in the series
	chapterDirs, err := os.ReadDir(seriesDir)
	if err != nil {
		ms.logger.Error().Err(err).Str("seriesDir", seriesDir).Msg("metadata_scanner: Failed to read series directory")
		return ""
	}

	// Look through all chapter directories for cover images
	for _, chapterDir := range chapterDirs {
		if !chapterDir.IsDir() {
			continue
		}

		chapterPath := filepath.Join(seriesDir, chapterDir.Name())
		
		// Check for registry.json first to see if this is a valid chapter
		registryPath := filepath.Join(chapterPath, "registry.json")
		if _, err := os.Stat(registryPath); os.IsNotExist(err) {
			ms.logger.Debug().Str("chapterPath", chapterPath).Msg("metadata_scanner: No registry.json found, skipping")
			continue
		}

		entries, err := os.ReadDir(chapterPath)
		if err != nil {
			ms.logger.Error().Err(err).Str("chapterPath", chapterPath).Msg("metadata_scanner: Failed to read chapter directory")
			continue
		}

		var coverCandidates []string
		var imageFiles []string

	// Look for both cover files and collect all images
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip registry.json
		if entry.Name() == "registry.json" {
			continue
		}

		// Check if it's an image file
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
			fileName := strings.ToLower(entry.Name())
			fileNameWithoutExt := strings.ToLower(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
			imageFiles = append(imageFiles, entry.Name())

			// Look for files that are specifically cover images
			if strings.Contains(fileName, "cover") ||
				strings.Contains(fileName, "thumbnail") ||
				fileNameWithoutExt == "cover" ||
				fileNameWithoutExt == "thumbnail" ||
				fileNameWithoutExt == "front" ||
				fileNameWithoutExt == "poster" ||
				strings.HasPrefix(fileName, "00") ||
				strings.HasPrefix(fileName, "01") { // Include first numbered file as potential cover
				coverCandidates = append(coverCandidates, entry.Name())
				ms.logger.Debug().Str("coverFile", entry.Name()).Str("chapterPath", chapterPath).Msg("manga metadata scanner: Found potential cover file")
			}
		}
	}

	// Return the best cover candidate (prioritize "cover" in name)
	if len(coverCandidates) > 0 {
		// First priority: files with "cover" in the name
		for _, candidate := range coverCandidates {
			if strings.Contains(strings.ToLower(candidate), "cover") {
				ms.logger.Debug().Str("selectedCover", candidate).Str("chapterPath", chapterPath).Msg("manga metadata scanner: Selected cover file")
				return filepath.Join(chapterPath, candidate)
			}
		}
		// Second priority: any other cover candidate
		ms.logger.Debug().Str("selectedCover", coverCandidates[0]).Str("chapterPath", chapterPath).Msg("manga metadata scanner: Selected first cover candidate")
		return filepath.Join(chapterPath, coverCandidates[0])
	}

	// Fallback: use first image file if no dedicated cover found
	if len(imageFiles) > 0 {
		ms.logger.Debug().Str("fallbackCover", imageFiles[0]).Str("chapterPath", chapterPath).Msg("manga metadata scanner: Using first image as cover fallback")
		return filepath.Join(chapterPath, imageFiles[0])
	}

		// No images found at all
		ms.logger.Debug().Str("chapterPath", chapterPath).Msg("manga metadata scanner: No images found for cover")
		return ""
	}

	// No cover found in any chapter
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

// performIncrementalUpdate checks for changes and updates only modified series
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

	// Process each directory
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
				continue
			}

			// If directory was modified after last cache update, rescan this series
			if stat.ModTime().Unix() > cached.LastUpdated {
				ms.logger.Debug().Str("series", entry.Name()).Msg("manga metadata scanner: Series modified, updating")
				seriesData, err := ms.scanSeries(entry.Name(), seriesPath)
				if err != nil {
					ms.logger.Warn().Err(err).Str("series", entry.Name()).Msg("manga metadata scanner: Failed to rescan series")
					// Keep the cached version if rescan fails
					updatedSeries = append(updatedSeries, *cached)
					continue
				}
				if seriesData != nil {
					updatedSeries = append(updatedSeries, *seriesData)
					updatedCount++
				}
			} else {
				// No changes, keep cached version
				updatedSeries = append(updatedSeries, *cached)
			}
		} else {
			// New series, scan it
			ms.logger.Debug().Str("series", entry.Name()).Msg("manga metadata scanner: New series found, scanning")
			seriesData, err := ms.scanSeries(entry.Name(), seriesPath)
			if err != nil {
				ms.logger.Warn().Err(err).Str("series", entry.Name()).Msg("manga metadata scanner: Failed to scan new series")
				continue
			}
			if seriesData != nil {
				updatedSeries = append(updatedSeries, *seriesData)
				addedCount++
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

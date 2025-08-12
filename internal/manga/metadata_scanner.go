package manga

import (
	"encoding/json"
	"fmt"
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

			// Try to find a cover image from the first chapter if we don't have one yet
			if coverImagePath == "" {
				absoluteCoverPath := ms.findCoverImage(chapterPath)
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
		LastUpdated:    lastUpdated,
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

// findCoverImage finds the first image in a chapter directory to use as cover
func (ms *MetadataScanner) findCoverImage(chapterPath string) string {
	entries, err := os.ReadDir(chapterPath)
	if err != nil {
		return ""
	}

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
			return filepath.Join(chapterPath, entry.Name())
		}
	}

	return ""
}

// GetDownloadedMangaList returns a cached list of downloaded manga or scans if not cached
func (ms *MetadataScanner) GetDownloadedMangaList() ([]DownloadedMangaSeries, error) {
	// Try to get from cache first
	cacheKey := "downloaded_manga_list"
	bucket := filecache.NewBucket("manga_metadata", 5*time.Minute)

	var series []DownloadedMangaSeries
	found, err := ms.filecacher.Get(bucket, cacheKey, &series)
	if err == nil && found {
		return series, nil
	}

	// Scan and cache the results
	series, err = ms.ScanDownloadedManga()
	if err != nil {
		return nil, err
	}

	// Cache the results for 5 minutes
	ms.filecacher.Set(bucket, cacheKey, series)

	return series, nil
}

// RefreshDownloadedMangaCache clears the cache to force a rescan
func (ms *MetadataScanner) RefreshDownloadedMangaCache() {
	bucket := filecache.NewBucket("manga_metadata", 5*time.Minute)
	ms.filecacher.Delete(bucket, "downloaded_manga_list")
}

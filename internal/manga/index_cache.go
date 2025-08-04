package manga

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type (
	// IndexCache provides persistent caching for manga directory structure
	// to avoid expensive file system scans on every startup
	IndexCache struct {
		logger    *zerolog.Logger
		cacheFile string
		mu        sync.RWMutex
		data      *IndexCacheData
	}

	// IndexCacheData stores the cached directory structure and metadata
	IndexCacheData struct {
		LastUpdated time.Time            `json:"lastUpdated"`
		Version     string               `json:"version"`
		MediaMap    MediaMap             `json:"mediaMap"`
		DirMtimes   map[string]time.Time `json:"dirMtimes"`  // Directory modification times
		FileHashes  map[string]string    `json:"fileHashes"` // File hashes for change detection
	}

	// IndexCacheOptions for creating a new IndexCache
	IndexCacheOptions struct {
		Logger      *zerolog.Logger
		CacheDir    string
		DownloadDir string
	}
)

const (
	indexCacheVersion = "1.0.0"
	indexCacheFile    = "manga_index_cache.json"
)

// NewIndexCache creates a new IndexCache instance
func NewIndexCache(opts IndexCacheOptions) *IndexCache {
	cacheFile := filepath.Join(opts.CacheDir, indexCacheFile)

	ic := &IndexCache{
		logger:    opts.Logger,
		cacheFile: cacheFile,
		data: &IndexCacheData{
			Version:    indexCacheVersion,
			MediaMap:   make(MediaMap),
			DirMtimes:  make(map[string]time.Time),
			FileHashes: make(map[string]string),
		},
	}

	// Try to load existing cache
	ic.loadFromDisk()

	return ic
}

// loadFromDisk loads the cache from disk if it exists
func (ic *IndexCache) loadFromDisk() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if _, err := os.Stat(ic.cacheFile); os.IsNotExist(err) {
		ic.logger.Debug().Msg("manga index cache: No existing cache file found")
		return
	}

	data, err := os.ReadFile(ic.cacheFile)
	if err != nil {
		ic.logger.Error().Err(err).Msg("manga index cache: Failed to read cache file")
		return
	}

	var cacheData IndexCacheData
	if err := json.Unmarshal(data, &cacheData); err != nil {
		ic.logger.Error().Err(err).Msg("manga index cache: Failed to unmarshal cache data")
		return
	}

	// Check version compatibility
	if cacheData.Version != indexCacheVersion {
		ic.logger.Info().Str("cached", cacheData.Version).Str("current", indexCacheVersion).
			Msg("manga index cache: Version mismatch, invalidating cache")
		return
	}

	ic.data = &cacheData
	ic.logger.Debug().Time("lastUpdated", cacheData.LastUpdated).
		Int("mediaCount", len(cacheData.MediaMap)).
		Msg("manga index cache: Loaded cache from disk")
}

// saveToDisk saves the current cache to disk
func (ic *IndexCache) saveToDisk() error {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	ic.data.LastUpdated = time.Now()

	data, err := json.MarshalIndent(ic.data, "", "  ")
	if err != nil {
		return err
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Dir(ic.cacheFile), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(ic.cacheFile, data, 0644); err != nil {
		return err
	}

	ic.logger.Debug().Int("mediaCount", len(ic.data.MediaMap)).
		Msg("manga index cache: Saved cache to disk")

	return nil
}

// IsValid checks if the cache is still valid by comparing directory modification times
func (ic *IndexCache) IsValid(downloadDir string) bool {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	// Check if cache is older than 1 hour (fallback invalidation)
	if time.Since(ic.data.LastUpdated) > time.Hour {
		ic.logger.Debug().Msg("manga index cache: Cache expired (> 1 hour)")
		return false
	}

	// Check if download directory has been modified
	dirInfo, err := os.Stat(downloadDir)
	if err != nil {
		ic.logger.Debug().Err(err).Msg("manga index cache: Cannot stat download directory")
		return false
	}

	cachedMtime, exists := ic.data.DirMtimes[downloadDir]
	if !exists || !dirInfo.ModTime().Equal(cachedMtime) {
		ic.logger.Debug().Time("cached", cachedMtime).Time("actual", dirInfo.ModTime()).
			Msg("manga index cache: Download directory modified")
		return false
	}

	// Quick validation: check if a few random directories still exist
	count := 0
	for mediaId := range ic.data.MediaMap {
		if count >= 3 { // Check max 3 directories for performance
			break
		}

		// Check if any chapter directory exists for this media
		found := false
		for _, chapters := range ic.data.MediaMap[mediaId] {
			if len(chapters) > 0 {
				// Try to find at least one chapter directory
				files, err := os.ReadDir(downloadDir)
				if err != nil {
					continue
				}

				for _, file := range files {
					if file.IsDir() {
						// Check both old and new directory structures
						chapterPath := filepath.Join(downloadDir, file.Name())
						if _, err := os.Stat(chapterPath); err == nil {
							found = true
							break
						}
					}
				}
				break
			}
		}

		if !found {
			ic.logger.Debug().Int("mediaId", mediaId).Msg("manga index cache: Media directory missing")
			return false
		}
		count++
	}

	ic.logger.Debug().Msg("manga index cache: Cache is valid")
	return true
}

// GetMediaMap returns the cached media map if valid, otherwise returns nil
func (ic *IndexCache) GetMediaMap(downloadDir string) *MediaMap {
	if !ic.IsValid(downloadDir) {
		return nil
	}

	ic.mu.RLock()
	defer ic.mu.RUnlock()

	// Return a copy to prevent external modifications
	mediaMapCopy := make(MediaMap)
	for mediaId, providerMap := range ic.data.MediaMap {
		mediaMapCopy[mediaId] = make(ProviderDownloadMap)
		for provider, chapters := range providerMap {
			mediaMapCopy[mediaId][provider] = append([]ProviderDownloadMapChapterInfo{}, chapters...)
		}
	}

	return &mediaMapCopy
}

// UpdateCache updates the cache with new media map data
func (ic *IndexCache) UpdateCache(downloadDir string, mediaMap *MediaMap) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Update directory modification times
	dirInfo, err := os.Stat(downloadDir)
	if err != nil {
		return err
	}

	ic.data.DirMtimes[downloadDir] = dirInfo.ModTime()
	ic.data.MediaMap = *mediaMap
	ic.data.LastUpdated = time.Now()

	// Save to disk asynchronously to avoid blocking
	go func() {
		if err := ic.saveToDisk(); err != nil {
			ic.logger.Error().Err(err).Msg("manga index cache: Failed to save cache to disk")
		}
	}()

	ic.logger.Debug().Int("mediaCount", len(*mediaMap)).
		Msg("manga index cache: Updated cache")

	return nil
}

// InvalidateCache clears the cache and removes the cache file
func (ic *IndexCache) InvalidateCache() error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.data = &IndexCacheData{
		Version:    indexCacheVersion,
		MediaMap:   make(MediaMap),
		DirMtimes:  make(map[string]time.Time),
		FileHashes: make(map[string]string),
	}

	// Remove cache file
	if err := os.Remove(ic.cacheFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	ic.logger.Info().Msg("manga index cache: Cache invalidated")
	return nil
}

// GetCacheStats returns statistics about the cache
func (ic *IndexCache) GetCacheStats() map[string]interface{} {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	totalChapters := 0
	providerCount := make(map[string]int)

	for _, providerMap := range ic.data.MediaMap {
		for provider, chapters := range providerMap {
			totalChapters += len(chapters)
			providerCount[provider] += len(chapters)
		}
	}

	return map[string]interface{}{
		"lastUpdated":   ic.data.LastUpdated,
		"version":       ic.data.Version,
		"mediaCount":    len(ic.data.MediaMap),
		"totalChapters": totalChapters,
		"providerStats": providerCount,
		"cacheFileSize": ic.getCacheFileSize(),
	}
}

// getCacheFileSize returns the size of the cache file in bytes
func (ic *IndexCache) getCacheFileSize() int64 {
	if info, err := os.Stat(ic.cacheFile); err == nil {
		return info.Size()
	}
	return 0
}

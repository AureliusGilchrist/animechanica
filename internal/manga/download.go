package manga

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"seanime/internal/api/anilist"
	"seanime/internal/database/db"
	"seanime/internal/database/models"
	"seanime/internal/events"
	"seanime/internal/hook"
	chapter_downloader "seanime/internal/manga/downloader"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/platforms/platform"
	"seanime/internal/util"
	"seanime/internal/util/filecache"
	"seanime/internal/util/result"
	"sync"

	"github.com/rs/zerolog"
)

// Global cache for downloaded manga metadata to minimize AniList API calls
var downloadedMangaCache = result.NewCache[int, *anilist.BaseManga]()

type (
	Downloader struct {
		logger            *zerolog.Logger
		wsEventManager    events.WSEventManagerInterface
		database          *db.Database
		downloadDir       string
		chapterDownloader *chapter_downloader.Downloader
		repository        *Repository
		filecacher        *filecache.Cacher
		indexCache        *IndexCache // Cache for faster manga indexing

		mediaMap   *MediaMap // Refreshed on start and after each download
		mediaMapMu sync.RWMutex

		chapterDownloadedCh chan chapter_downloader.DownloadID
		readingDownloadDir  bool
		isOffline           *bool
	}

	// MediaMap is created after reading the download directory.
	// It is used to store all downloaded chapters for each media.
	// The key is the media ID and the value is a map of provider to a list of chapters.
	//
	//	e.g., downloadDir/comick_1234_abc_13/
	//	      downloadDir/comick_1234_def_13.5/
	// -> { 1234: { "comick": [ { "chapterId": "abc", "chapterNumber": "13" }, { "chapterId": "def", "chapterNumber": "13.5" } ] } }
	MediaMap map[int]ProviderDownloadMap

	// ProviderDownloadMap is used to store all downloaded chapters for a specific media and provider.
	// The key is the provider and the value is a list of chapters.
	ProviderDownloadMap map[string][]ProviderDownloadMapChapterInfo

	ProviderDownloadMapChapterInfo struct {
		ChapterID     string `json:"chapterId"`
		ChapterNumber string `json:"chapterNumber"`
	}

	MediaDownloadData struct {
		Downloaded ProviderDownloadMap `json:"downloaded"`
		Queued     ProviderDownloadMap `json:"queued"`
	}
)

type (
	NewDownloaderOptions struct {
		Database       *db.Database
		Logger         *zerolog.Logger
		WSEventManager events.WSEventManagerInterface
		DownloadDir    string
		Repository     *Repository
		IsOffline      *bool
	}

	DownloadChapterOptions struct {
		Provider   string
		MediaId    int
		ChapterId  string
		StartNow   bool
		MangaTitle string // Optional field for series-based directory naming
	}
)

func NewDownloader(opts *NewDownloaderOptions) *Downloader {
	_ = os.MkdirAll(opts.DownloadDir, os.ModePerm)
	filecacher, _ := filecache.NewCacher(opts.DownloadDir)

	// Initialize index cache for faster manga loading
	indexCache := NewIndexCache(IndexCacheOptions{
		Logger:      opts.Logger,
		CacheDir:    filepath.Join(opts.DownloadDir, ".cache"),
		DownloadDir: opts.DownloadDir,
	})

	d := &Downloader{
		logger:         opts.Logger,
		wsEventManager: opts.WSEventManager,
		database:       opts.Database,
		downloadDir:    opts.DownloadDir,
		repository:     opts.Repository,
		mediaMap:       new(MediaMap),
		filecacher:     filecacher,
		indexCache:     indexCache,
		isOffline:      opts.IsOffline,
	}

	d.chapterDownloader = chapter_downloader.NewDownloader(&chapter_downloader.NewDownloaderOptions{
		Logger:         opts.Logger,
		WSEventManager: opts.WSEventManager,
		Database:       opts.Database,
		DownloadDir:    opts.DownloadDir,
	})

	go d.hydrateMediaMap()

	return d
}

// Start is called once to start the Chapter downloader 's main goroutine.
func (d *Downloader) Start() {
	d.chapterDownloader.Start()
	go func() {
		for {
			select {
			// Listen for downloaded chapters
			case downloadId := <-d.chapterDownloader.ChapterDownloaded():
				if d.isOffline != nil && *d.isOffline {
					continue
				}

				// When a chapter is downloaded, fetch the chapter container from the file cache
				// and store it in the permanent bucket.
				// DEVNOTE: This will be useful to avoid re-fetching the chapter container when the cache expires.
				// This is deleted when a chapter is deleted.
				go func() {
					chapterContainerKey := getMangaChapterContainerCacheKey(downloadId.Provider, downloadId.MediaId)
					chapterContainer, found := d.repository.getChapterContainerFromFilecache(downloadId.Provider, downloadId.MediaId)
					if found {
						// Store the chapter container in the permanent bucket
						permBucket := getPermanentChapterContainerCacheBucket(downloadId.Provider, downloadId.MediaId)
						_ = d.filecacher.SetPerm(permBucket, chapterContainerKey, chapterContainer)
					}
				}()

				// Refresh the media map when a chapter is downloaded
				d.hydrateMediaMap()
			}
		}
	}()
}

// The bucket for storing downloaded chapter containers.
// e.g. manga_downloaded_comick_chapters_1234
// The key is the chapter ID.
func getPermanentChapterContainerCacheBucket(provider string, mId int) filecache.PermanentBucket {
	return filecache.NewPermanentBucket(fmt.Sprintf("manga_downloaded_%s_chapters_%d", provider, mId))
}

// getChapterContainerFromFilecache returns the chapter container from the temporary file cache.
func (r *Repository) getChapterContainerFromFilecache(provider string, mId int) (*ChapterContainer, bool) {
	// Find chapter container in the file cache
	chapterBucket := r.getFcProviderBucket(provider, mId, bucketTypeChapter)

	chapterContainerKey := getMangaChapterContainerCacheKey(provider, mId)

	var chapterContainer *ChapterContainer
	// Get the key-value pair in the bucket
	if found, _ := r.fileCacher.Get(chapterBucket, chapterContainerKey, &chapterContainer); !found {
		// If the chapter container is not found, return an error
		// since it means that it wasn't fetched (for some reason) -- This shouldn't happen
		return nil, false
	}

	return chapterContainer, true
}

// getChapterContainerFromPermanentFilecache returns the chapter container from the permanent file cache.
func (r *Repository) getChapterContainerFromPermanentFilecache(provider string, mId int) (*ChapterContainer, bool) {
	permBucket := getPermanentChapterContainerCacheBucket(provider, mId)

	chapterContainerKey := getMangaChapterContainerCacheKey(provider, mId)

	var chapterContainer *ChapterContainer
	// Get the key-value pair in the bucket
	if found, _ := r.fileCacher.GetPerm(permBucket, chapterContainerKey, &chapterContainer); !found {
		// If the chapter container is not found, return an error
		// since it means that it wasn't fetched (for some reason) -- This shouldn't happen
		return nil, false
	}

	return chapterContainer, true
}

// DownloadChapter is called by the client to download a chapter.
// It fetches the chapter pages by using Repository.GetMangaPageContainer
// and invokes the chapter_downloader.Downloader 'Download' method to add the chapter to the download queue.
func (d *Downloader) DownloadChapter(opts DownloadChapterOptions) error {
	d.logger.Info().Msgf("[DownloadChapter] Queuing chapter: Provider=%s MediaId=%d ChapterId=%s", opts.Provider, opts.MediaId, opts.ChapterId)

	if d.isOffline != nil && *d.isOffline {
		d.logger.Error().Msg("[DownloadChapter] Manga downloader is in offline mode")
		return errors.New("manga downloader: Manga downloader is in offline mode")
	}

	chapterContainer, found := d.repository.getChapterContainerFromFilecache(opts.Provider, opts.MediaId)
	if !found {
		d.logger.Error().Msgf("[DownloadChapter] Chapters not found in filecache for Provider=%s MediaId=%d", opts.Provider, opts.MediaId)
		return errors.New("chapters not found")
	}

	// Find the chapter in the chapter container
	// e.g. Wind-Breaker$0062
	chapter, ok := chapterContainer.GetChapter(opts.ChapterId)
	if !ok {
		d.logger.Error().Msgf("[DownloadChapter] Chapter not found in container: ChapterId=%s", opts.ChapterId)
		return errors.New("chapter not found")
	}

	// Fetch the chapter pages
	pageContainer, err := d.repository.GetMangaPageContainer(opts.Provider, opts.MediaId, opts.ChapterId, false, &[]bool{false}[0])
	if err != nil {
		d.logger.Error().Err(err).Msgf("[DownloadChapter] Failed to get page container: Provider=%s MediaId=%d ChapterId=%s", opts.Provider, opts.MediaId, opts.ChapterId)
		return err
	}

	// Add the chapter to the download queue
	d.logger.Info().Msgf("[DownloadChapter] Adding chapter to queue: Provider=%s MediaId=%d ChapterId=%s", opts.Provider, opts.MediaId, opts.ChapterId)
	return d.chapterDownloader.AddToQueue(chapter_downloader.DownloadOptions{
		DownloadID: chapter_downloader.DownloadID{
			Provider:      opts.Provider,
			MediaId:       opts.MediaId,
			ChapterId:     opts.ChapterId,
			ChapterNumber: manga_providers.GetNormalizedChapter(chapter.Chapter),
			MangaTitle:    opts.MangaTitle,
		},
		Pages: pageContainer.Pages,
	})
}

// DeleteChapter is called by the client to delete a downloaded chapter.
func (d *Downloader) DeleteChapter(provider string, mediaId int, chapterId string, chapterNumber string) (err error) {
	err = d.chapterDownloader.DeleteChapter(chapter_downloader.DownloadID{
		Provider:      provider,
		MediaId:       mediaId,
		ChapterId:     chapterId,
		ChapterNumber: chapterNumber,
	})
	if err != nil {
		return err
	}

	permBucket := getPermanentChapterContainerCacheBucket(provider, mediaId)
	_ = d.filecacher.DeletePerm(permBucket, chapterId)

	d.hydrateMediaMap()

	return nil
}

// DeleteChapters is called by the client to delete downloaded chapters.
func (d *Downloader) DeleteChapters(ids []chapter_downloader.DownloadID) (err error) {
	for _, id := range ids {
		err = d.chapterDownloader.DeleteChapter(chapter_downloader.DownloadID{
			Provider:      id.Provider,
			MediaId:       id.MediaId,
			ChapterId:     id.ChapterId,
			ChapterNumber: id.ChapterNumber,
		})

		permBucket := getPermanentChapterContainerCacheBucket(id.Provider, id.MediaId)
		_ = d.filecacher.DeletePerm(permBucket, id.ChapterId)
	}
	if err != nil {
		return err
	}

	d.hydrateMediaMap()

	return nil
}

func (d *Downloader) GetMediaDownloads(mediaId int, cached bool) (ret MediaDownloadData, err error) {
	defer util.HandlePanicInModuleWithError("manga/GetMediaDownloads", &err)

	if !cached {
		d.hydrateMediaMap()
	}

	return d.mediaMap.getMediaDownload(mediaId, d.database)
}

func (d *Downloader) RunChapterDownloadQueue() {
	d.chapterDownloader.Run()
}

func (d *Downloader) StopChapterDownloadQueue() {
	_ = d.database.ResetDownloadingChapterDownloadQueueItems()
	d.chapterDownloader.Stop()
}

// InvalidateMangaIndexCache invalidates the manga index cache
// This should be called when manga is added or removed to ensure fresh data
func (d *Downloader) InvalidateMangaIndexCache() error {
	d.logger.Info().Msg("manga downloader: Invalidating index cache")
	return d.indexCache.InvalidateCache()
}

// RefreshMangaIndex forces a refresh of the manga index, bypassing cache
func (d *Downloader) RefreshMangaIndex() {
	d.logger.Info().Msg("manga downloader: Forcing manga index refresh")
	// Invalidate cache first
	if err := d.indexCache.InvalidateCache(); err != nil {
		d.logger.Error().Err(err).Msg("manga downloader: Failed to invalidate cache")
	}
	// Trigger fresh scan
	go d.hydrateMediaMap()
}

// GetMangaIndexCacheStats returns statistics about the manga index cache
func (d *Downloader) GetMangaIndexCacheStats() map[string]interface{} {
	return d.indexCache.GetCacheStats()
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type (
	NewDownloadListOptions struct {
		MangaCollection *anilist.MangaCollection
		AnilistPlatform platform.Platform
	}

	DownloadListItem struct {
		MediaId int `json:"mediaId"`
		// Media will be nil if the manga is no longer in the user's collection.
		// The client should handle this case by displaying the download data without the media data.
		Media        *anilist.BaseManga  `json:"media"`
		DownloadData ProviderDownloadMap `json:"downloadData"`
	}
)

// NewDownloadList returns a list of DownloadListItem for the client to display.
// Updated to fetch AniList metadata for manga not in user's collection to ensure visibility.
func (d *Downloader) NewDownloadList(opts *NewDownloadListOptions) (ret []*DownloadListItem, err error) {
	defer util.HandlePanicInModuleWithError("manga/NewDownloadList", &err)

	mm := d.mediaMap

	ret = make([]*DownloadListItem, 0)

	for mId, data := range *mm {
		var media *anilist.BaseManga

		// First check if manga is in user's collection (fast lookup)
		if listEntry, ok := opts.MangaCollection.GetListEntryFromMangaId(mId); ok {
			// Manga is in user's collection, use the existing media data (no API call needed)
			media = listEntry.GetMedia()
		} else {
			// Check if we have cached metadata
			if cached, found := downloadedMangaCache.Get(mId); found {
				media = cached
			} else {
				// Manga not in collection and not cached - fetch from AniList to ensure visibility
				// This allows users without manga in their AniList collection to see downloaded manga
				if opts.AnilistPlatform != nil {
					d.logger.Debug().Int("mediaId", mId).Msg("manga: Fetching metadata for downloaded manga not in collection")
					fetchedManga, fetchErr := opts.AnilistPlatform.GetManga(context.Background(), mId)
					if fetchErr == nil && fetchedManga != nil {
						media = fetchedManga
						// Cache the fetched metadata for future use
						downloadedMangaCache.Set(mId, fetchedManga)
						d.logger.Debug().Int("mediaId", mId).Str("title", fetchedManga.GetTitleSafe()).Msg("manga: Successfully fetched and cached metadata")
					} else {
						d.logger.Warn().Int("mediaId", mId).Err(fetchErr).Msg("manga: Failed to fetch metadata from AniList")
						media = nil
					}
				} else {
					d.logger.Warn().Int("mediaId", mId).Msg("manga: No AniList platform available to fetch metadata")
					media = nil
				}
			}
		}

		item := &DownloadListItem{
			MediaId:      mId,
			Media:        media,
			DownloadData: data,
		}

		ret = append(ret, item)
	}

	return
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Media map
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (mm *MediaMap) getMediaDownload(mediaId int, db *db.Database) (MediaDownloadData, error) {

	if mm == nil {
		return MediaDownloadData{}, errors.New("could not check downloaded chapters")
	}

	// Get all downloaded chapters for the media
	downloads, ok := (*mm)[mediaId]
	if !ok {
		downloads = make(map[string][]ProviderDownloadMapChapterInfo)
	}

	// Get all queued chapters for the media
	queued, err := db.GetMediaQueuedChapters(mediaId)
	if err != nil {
		queued = make([]*models.ChapterDownloadQueueItem, 0)
	}

	qm := make(ProviderDownloadMap)
	for _, item := range queued {
		if _, ok := qm[item.Provider]; !ok {
			qm[item.Provider] = []ProviderDownloadMapChapterInfo{
				{
					ChapterID:     item.ChapterID,
					ChapterNumber: item.ChapterNumber,
				},
			}
		} else {
			qm[item.Provider] = append(qm[item.Provider], ProviderDownloadMapChapterInfo{
				ChapterID:     item.ChapterID,
				ChapterNumber: item.ChapterNumber,
			})
		}
	}

	data := MediaDownloadData{
		Downloaded: downloads,
		Queued:     qm,
	}

	return data, nil

}

// hydrateMediaMap hydrates the MediaMap by reading the download directory.
// Uses IndexCache for dramatically faster loading of already downloaded manga.
func (d *Downloader) hydrateMediaMap() {

	if d.readingDownloadDir {
		return
	}

	d.mediaMapMu.Lock()
	defer d.mediaMapMu.Unlock()

	d.readingDownloadDir = true
	defer func() {
		d.readingDownloadDir = false
	}()

	d.logger.Debug().Msg("manga downloader: Reading download directory")

	// Try to load from cache first for faster startup
	if cachedMediaMap := d.indexCache.GetMediaMap(d.downloadDir); cachedMediaMap != nil {
		d.logger.Info().Int("mediaCount", len(*cachedMediaMap)).Msg("manga downloader: Loaded media map from cache (fast path)")
		d.mediaMap = cachedMediaMap

		// Trigger hook event with cached data
		ev := &MangaDownloadMapEvent{
			MediaMap: cachedMediaMap,
		}
		_ = hook.GlobalHookManager.OnMangaDownloadMap().Trigger(ev)
		if ev.MediaMap != nil {
			*d.mediaMap = *ev.MediaMap
		}

		// Send refresh event to client
		d.wsEventManager.SendEvent(events.RefreshedMangaDownloadData, nil)
		return
	}

	// Cache miss or invalid - perform full directory scan
	d.logger.Info().Msg("manga downloader: Cache miss, performing full directory scan")
	ret := make(MediaMap)

	files, err := os.ReadDir(d.downloadDir)
	if err != nil {
		d.logger.Error().Err(err).Msg("manga downloader: Failed to read download directory")
		return
	}

	// Hydrate MediaMap by going through all directories (both old flat structure and new series structure)
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, file := range files {
		wg.Add(1)
		go func(file os.DirEntry) {
			defer wg.Done()

			if file.IsDir() {
				// Try to parse as chapter directory first (old flat structure)
				id, ok := chapter_downloader.ParseChapterDirName(file.Name())
				if ok {
					// Old flat structure: comick_1234_abc_13.5
					d.addChapterToMediaMap(&ret, &mu, id)
				} else {
					// Check if this is a series directory (new structure)
					seriesPath := filepath.Join(d.downloadDir, file.Name())
					seriesFiles, err := os.ReadDir(seriesPath)
					if err != nil {
						return
					}

					// Check if any subdirectory contains a provider name (indicates it's a series folder)
					hasProviderChapters := false
					for _, seriesFile := range seriesFiles {
						if seriesFile.IsDir() {
							if chapterId, chapterOk := chapter_downloader.ParseChapterDirName(seriesFile.Name()); chapterOk {
								hasProviderChapters = true
								d.addChapterToMediaMap(&ret, &mu, chapterId)
							}
						}
					}

					// If no provider chapters found, this might be an unrelated directory
					if !hasProviderChapters {
						d.logger.Debug().Msgf("Skipping directory '%s' - no manga chapters found", file.Name())
					}
				}
			}
		}(file)
	}
	wg.Wait()

	// Trigger hook event
	ev := &MangaDownloadMapEvent{
		MediaMap: &ret,
	}
	_ = hook.GlobalHookManager.OnMangaDownloadMap().Trigger(ev) // ignore the error
	// make sure the media map is not nil
	if ev.MediaMap != nil {
		ret = *ev.MediaMap
	}

	d.mediaMap = &ret

	// Update cache with newly scanned data for faster future loading
	if err := d.indexCache.UpdateCache(d.downloadDir, &ret); err != nil {
		d.logger.Error().Err(err).Msg("manga downloader: Failed to update index cache")
	} else {
		d.logger.Debug().Int("mediaCount", len(ret)).Msg("manga downloader: Updated index cache")
	}

	// When done refreshing, send a message to the client to refetch the download data
	d.wsEventManager.SendEvent(events.RefreshedMangaDownloadData, nil)
}

// addChapterToMediaMap is a helper function to add chapter information to the MediaMap
// This reduces code duplication in hydrateMediaMap
func (d *Downloader) addChapterToMediaMap(ret *MediaMap, mu *sync.Mutex, id chapter_downloader.DownloadID) {
	mu.Lock()
	defer mu.Unlock()

	newMapInfo := ProviderDownloadMapChapterInfo{
		ChapterID:     id.ChapterId,
		ChapterNumber: id.ChapterNumber,
	}

	if _, ok := (*ret)[id.MediaId]; !ok {
		(*ret)[id.MediaId] = make(map[string][]ProviderDownloadMapChapterInfo)
		(*ret)[id.MediaId][id.Provider] = []ProviderDownloadMapChapterInfo{newMapInfo}
	} else {
		if _, ok := (*ret)[id.MediaId][id.Provider]; !ok {
			(*ret)[id.MediaId][id.Provider] = []ProviderDownloadMapChapterInfo{newMapInfo}
		} else {
			(*ret)[id.MediaId][id.Provider] = append((*ret)[id.MediaId][id.Provider], newMapInfo)
		}
	}
}

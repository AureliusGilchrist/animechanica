package cache

import (
	"seanime/internal/api/anilist"
	"seanime/internal/util/result"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type AnilistCacheManager struct {
	logger *zerolog.Logger
	mu     sync.RWMutex

	// Collection caches - per session
	animeCollections    map[string]*result.Cache[string, *anilist.AnimeCollection]
	rawAnimeCollections map[string]*result.Cache[string, *anilist.AnimeCollection]
	mangaCollections    map[string]*result.Cache[string, *anilist.MangaCollection]
	rawMangaCollections map[string]*result.Cache[string, *anilist.MangaCollection]

	// Individual media caches - shared across sessions
	baseAnimeCache     *result.Cache[int, *anilist.BaseAnime]
	baseMangaCache     *result.Cache[int, *anilist.BaseManga]
	completeAnimeCache *result.Cache[int, *anilist.CompleteAnime]
	animeDetailsCache  *result.Cache[int, *anilist.AnimeDetailsById_Media]
	mangaDetailsCache  *result.Cache[int, *anilist.MangaDetailsById_Media]

	// Character and studio caches - shared across sessions
	characterCache *result.Cache[int, *anilist.Character]
	studioCache    *result.Cache[int, *anilist.StudioDetails]

	// Stats caches - per session
	statsCache map[string]*result.Cache[string, *anilist.ViewerStats]

	// Airing schedule cache - shared across sessions
	airingScheduleCache *result.Cache[string, *anilist.AnimeAiringSchedule]

	// Cache expiration times
	collectionExpiry    time.Duration
	mediaExpiry         time.Duration
	characterExpiry     time.Duration
	statsExpiry         time.Duration
	airingScheduleExpiry time.Duration
}

func NewAnilistCacheManager(logger *zerolog.Logger) *AnilistCacheManager {
	return &AnilistCacheManager{
		logger: logger,

		// Initialize per-session caches
		animeCollections:    make(map[string]*result.Cache[string, *anilist.AnimeCollection]),
		rawAnimeCollections: make(map[string]*result.Cache[string, *anilist.AnimeCollection]),
		mangaCollections:    make(map[string]*result.Cache[string, *anilist.MangaCollection]),
		rawMangaCollections: make(map[string]*result.Cache[string, *anilist.MangaCollection]),
		statsCache:          make(map[string]*result.Cache[string, *anilist.ViewerStats]),

		// Initialize shared caches
		baseAnimeCache:      result.NewCache[int, *anilist.BaseAnime](),
		baseMangaCache:      result.NewCache[int, *anilist.BaseManga](),
		completeAnimeCache:  result.NewCache[int, *anilist.CompleteAnime](),
		animeDetailsCache:   result.NewCache[int, *anilist.AnimeDetailsById_Media](),
		mangaDetailsCache:   result.NewCache[int, *anilist.MangaDetailsById_Media](),
		characterCache:      result.NewCache[int, *anilist.Character](),
		studioCache:         result.NewCache[int, *anilist.StudioDetails](),
		airingScheduleCache: result.NewCache[string, *anilist.AnimeAiringSchedule](),

		// Set cache expiration times - optimized for reduced API calls
		collectionExpiry:     2 * time.Hour,    // Collections expire after 2 hours (was 30 min)
		mediaExpiry:          6 * time.Hour,    // Individual media expires after 6 hours (was 2 hours)
		characterExpiry:      7 * 24 * time.Hour, // Characters expire after 1 week (was 24 hours)
		statsExpiry:          4 * time.Hour,    // Stats expire after 4 hours (was 1 hour)
		airingScheduleExpiry: 1 * time.Hour,    // Airing schedule expires after 1 hour (was 15 min)
	}
}

// Collection cache methods

func (acm *AnilistCacheManager) GetAnimeCollection(sessionID string, key string) (*anilist.AnimeCollection, bool) {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	cache, exists := acm.animeCollections[sessionID]
	if !exists {
		return nil, false
	}

	return cache.Get(key)
}

func (acm *AnilistCacheManager) SetAnimeCollection(sessionID string, key string, collection *anilist.AnimeCollection) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if _, exists := acm.animeCollections[sessionID]; !exists {
		acm.animeCollections[sessionID] = result.NewCache[string, *anilist.AnimeCollection]()
	}

	acm.animeCollections[sessionID].SetT(key, collection, acm.collectionExpiry)
	acm.logger.Debug().Str("sessionID", sessionID).Str("key", key).Msg("Cached anime collection")
}

func (acm *AnilistCacheManager) GetRawAnimeCollection(sessionID string, key string) (*anilist.AnimeCollection, bool) {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	cache, exists := acm.rawAnimeCollections[sessionID]
	if !exists {
		return nil, false
	}

	return cache.Get(key)
}

func (acm *AnilistCacheManager) SetRawAnimeCollection(sessionID string, key string, collection *anilist.AnimeCollection) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if _, exists := acm.rawAnimeCollections[sessionID]; !exists {
		acm.rawAnimeCollections[sessionID] = result.NewCache[string, *anilist.AnimeCollection]()
	}

	acm.rawAnimeCollections[sessionID].SetT(key, collection, acm.collectionExpiry)
	acm.logger.Debug().Str("sessionID", sessionID).Str("key", key).Msg("Cached raw anime collection")
}

func (acm *AnilistCacheManager) GetMangaCollection(sessionID string, key string) (*anilist.MangaCollection, bool) {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	cache, exists := acm.mangaCollections[sessionID]
	if !exists {
		return nil, false
	}

	return cache.Get(key)
}

func (acm *AnilistCacheManager) SetMangaCollection(sessionID string, key string, collection *anilist.MangaCollection) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if _, exists := acm.mangaCollections[sessionID]; !exists {
		acm.mangaCollections[sessionID] = result.NewCache[string, *anilist.MangaCollection]()
	}

	acm.mangaCollections[sessionID].SetT(key, collection, acm.collectionExpiry)
	acm.logger.Debug().Str("sessionID", sessionID).Str("key", key).Msg("Cached manga collection")
}

func (acm *AnilistCacheManager) GetRawMangaCollection(sessionID string, key string) (*anilist.MangaCollection, bool) {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	cache, exists := acm.rawMangaCollections[sessionID]
	if !exists {
		return nil, false
	}

	return cache.Get(key)
}

func (acm *AnilistCacheManager) SetRawMangaCollection(sessionID string, key string, collection *anilist.MangaCollection) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if _, exists := acm.rawMangaCollections[sessionID]; !exists {
		acm.rawMangaCollections[sessionID] = result.NewCache[string, *anilist.MangaCollection]()
	}

	acm.rawMangaCollections[sessionID].SetT(key, collection, acm.collectionExpiry)
	acm.logger.Debug().Str("sessionID", sessionID).Str("key", key).Msg("Cached raw manga collection")
}

// Individual media cache methods

func (acm *AnilistCacheManager) GetBaseAnime(mediaID int) (*anilist.BaseAnime, bool) {
	return acm.baseAnimeCache.Get(mediaID)
}

func (acm *AnilistCacheManager) SetBaseAnime(mediaID int, anime *anilist.BaseAnime) {
	acm.baseAnimeCache.SetT(mediaID, anime, acm.mediaExpiry)
	acm.logger.Debug().Int("mediaID", mediaID).Msg("Cached base anime")
}

func (acm *AnilistCacheManager) GetBaseManga(mediaID int) (*anilist.BaseManga, bool) {
	return acm.baseMangaCache.Get(mediaID)
}

func (acm *AnilistCacheManager) SetBaseManga(mediaID int, manga *anilist.BaseManga) {
	acm.baseMangaCache.SetT(mediaID, manga, acm.mediaExpiry)
	acm.logger.Debug().Int("mediaID", mediaID).Msg("Cached base manga")
}

func (acm *AnilistCacheManager) GetCompleteAnime(mediaID int) (*anilist.CompleteAnime, bool) {
	return acm.completeAnimeCache.Get(mediaID)
}

func (acm *AnilistCacheManager) SetCompleteAnime(mediaID int, anime *anilist.CompleteAnime) {
	acm.completeAnimeCache.SetT(mediaID, anime, acm.mediaExpiry)
	acm.logger.Debug().Int("mediaID", mediaID).Msg("Cached complete anime")
}

func (acm *AnilistCacheManager) GetAnimeDetails(mediaID int) (*anilist.AnimeDetailsById_Media, bool) {
	return acm.animeDetailsCache.Get(mediaID)
}

func (acm *AnilistCacheManager) SetAnimeDetails(mediaID int, details *anilist.AnimeDetailsById_Media) {
	acm.animeDetailsCache.SetT(mediaID, details, acm.mediaExpiry)
	acm.logger.Debug().Int("mediaID", mediaID).Msg("Cached anime details")
}

func (acm *AnilistCacheManager) GetMangaDetails(mediaID int) (*anilist.MangaDetailsById_Media, bool) {
	return acm.mangaDetailsCache.Get(mediaID)
}

func (acm *AnilistCacheManager) SetMangaDetails(mediaID int, details *anilist.MangaDetailsById_Media) {
	acm.mangaDetailsCache.SetT(mediaID, details, acm.mediaExpiry)
	acm.logger.Debug().Int("mediaID", mediaID).Msg("Cached manga details")
}

// Character and studio cache methods

func (acm *AnilistCacheManager) GetCharacter(characterID int) (*anilist.Character, bool) {
	return acm.characterCache.Get(characterID)
}

func (acm *AnilistCacheManager) SetCharacter(characterID int, character *anilist.Character) {
	acm.characterCache.SetT(characterID, character, acm.characterExpiry)
	acm.logger.Debug().Int("characterID", characterID).Msg("Cached character")
}

func (acm *AnilistCacheManager) GetStudio(studioID int) (*anilist.StudioDetails, bool) {
	return acm.studioCache.Get(studioID)
}

func (acm *AnilistCacheManager) SetStudio(studioID int, studio *anilist.StudioDetails) {
	acm.studioCache.SetT(studioID, studio, acm.characterExpiry)
	acm.logger.Debug().Int("studioID", studioID).Msg("Cached studio")
}

// Stats cache methods

func (acm *AnilistCacheManager) GetViewerStats(sessionID string) (*anilist.ViewerStats, bool) {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	cache, exists := acm.statsCache[sessionID]
	if !exists {
		return nil, false
	}

	return cache.Get("stats")
}

func (acm *AnilistCacheManager) SetViewerStats(sessionID string, stats *anilist.ViewerStats) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if _, exists := acm.statsCache[sessionID]; !exists {
		acm.statsCache[sessionID] = result.NewCache[string, *anilist.ViewerStats]()
	}

	acm.statsCache[sessionID].SetT("stats", stats, acm.statsExpiry)
	acm.logger.Debug().Str("sessionID", sessionID).Msg("Cached viewer stats")
}

// Airing schedule cache methods

func (acm *AnilistCacheManager) GetAiringSchedule(key string) (*anilist.AnimeAiringSchedule, bool) {
	return acm.airingScheduleCache.Get(key)
}

func (acm *AnilistCacheManager) SetAiringSchedule(key string, schedule *anilist.AnimeAiringSchedule) {
	acm.airingScheduleCache.SetT(key, schedule, acm.airingScheduleExpiry)
	acm.logger.Debug().Str("key", key).Msg("Cached airing schedule")
}

// Cache clearing methods

func (acm *AnilistCacheManager) ClearAllCaches() {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	// Clear per-session caches
	acm.animeCollections = make(map[string]*result.Cache[string, *anilist.AnimeCollection])
	acm.rawAnimeCollections = make(map[string]*result.Cache[string, *anilist.AnimeCollection])
	acm.mangaCollections = make(map[string]*result.Cache[string, *anilist.MangaCollection])
	acm.rawMangaCollections = make(map[string]*result.Cache[string, *anilist.MangaCollection])
	acm.statsCache = make(map[string]*result.Cache[string, *anilist.ViewerStats])

	// Clear shared caches
	acm.baseAnimeCache.Clear()
	acm.baseMangaCache.Clear()
	acm.completeAnimeCache.Clear()
	acm.animeDetailsCache.Clear()
	acm.mangaDetailsCache.Clear()
	acm.characterCache.Clear()
	acm.studioCache.Clear()
	acm.airingScheduleCache.Clear()

	acm.logger.Info().Msg("Cleared all AniList caches")
}

func (acm *AnilistCacheManager) ClearCollectionCaches() {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	acm.animeCollections = make(map[string]*result.Cache[string, *anilist.AnimeCollection])
	acm.rawAnimeCollections = make(map[string]*result.Cache[string, *anilist.AnimeCollection])
	acm.mangaCollections = make(map[string]*result.Cache[string, *anilist.MangaCollection])
	acm.rawMangaCollections = make(map[string]*result.Cache[string, *anilist.MangaCollection])

	acm.logger.Info().Msg("Cleared collection caches")
}

func (acm *AnilistCacheManager) ClearMediaCaches() {
	acm.baseAnimeCache.Clear()
	acm.baseMangaCache.Clear()
	acm.completeAnimeCache.Clear()
	acm.animeDetailsCache.Clear()
	acm.mangaDetailsCache.Clear()

	acm.logger.Info().Msg("Cleared media caches")
}

func (acm *AnilistCacheManager) ClearCharacterCaches() {
	acm.characterCache.Clear()
	acm.studioCache.Clear()

	acm.logger.Info().Msg("Cleared character and studio caches")
}

func (acm *AnilistCacheManager) ClearStatsCaches() {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	acm.statsCache = make(map[string]*result.Cache[string, *anilist.ViewerStats])

	acm.logger.Info().Msg("Cleared stats caches")
}

func (acm *AnilistCacheManager) ClearSessionCaches(sessionID string) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	delete(acm.animeCollections, sessionID)
	delete(acm.rawAnimeCollections, sessionID)
	delete(acm.mangaCollections, sessionID)
	delete(acm.rawMangaCollections, sessionID)
	delete(acm.statsCache, sessionID)

	acm.logger.Info().Str("sessionID", sessionID).Msg("Cleared session caches")
}

// Character Details Cache Methods
func (acm *AnilistCacheManager) GetCharacterDetails(characterID int) (*anilist.Character, bool) {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	if cached, found := acm.characterCache.Get(characterID); found {
		acm.logger.Debug().Int("characterID", characterID).Msg("anilist cache: Character details cache hit")
		return cached, true
	}
	return nil, false
}

func (acm *AnilistCacheManager) SetCharacterDetails(characterID int, character *anilist.Character) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	acm.characterCache.SetT(characterID, character, 24*time.Hour) // Cache for 24 hours
	acm.logger.Debug().Int("characterID", characterID).Msg("anilist cache: Cached character details")
}

// Studio Details Cache Methods
func (acm *AnilistCacheManager) GetStudioDetails(studioID int) (*anilist.StudioDetails, bool) {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	if cached, found := acm.studioCache.Get(studioID); found {
		acm.logger.Debug().Int("studioID", studioID).Msg("anilist cache: Studio details cache hit")
		return cached, true
	}
	return nil, false
}

func (acm *AnilistCacheManager) SetStudioDetails(studioID int, studio *anilist.StudioDetails) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	acm.studioCache.SetT(studioID, studio, 24*time.Hour) // Cache for 24 hours
	acm.logger.Debug().Int("studioID", studioID).Msg("anilist cache: Cached studio details")
}

// Cache statistics methods

func (acm *AnilistCacheManager) countCacheEntries(cache interface{}) int {
	count := 0
	switch c := cache.(type) {
	case *result.Cache[int, *anilist.BaseAnime]:
		c.Range(func(key int, value *anilist.BaseAnime) bool {
			count++
			return true
		})
	case *result.Cache[int, *anilist.BaseManga]:
		c.Range(func(key int, value *anilist.BaseManga) bool {
			count++
			return true
		})
	case *result.Cache[int, *anilist.CompleteAnime]:
		c.Range(func(key int, value *anilist.CompleteAnime) bool {
			count++
			return true
		})
	case *result.Cache[int, *anilist.AnimeDetailsById_Media]:
		c.Range(func(key int, value *anilist.AnimeDetailsById_Media) bool {
			count++
			return true
		})
	case *result.Cache[int, *anilist.MangaDetailsById_Media]:
		c.Range(func(key int, value *anilist.MangaDetailsById_Media) bool {
			count++
			return true
		})
	case *result.Cache[int, *anilist.Character]:
		c.Range(func(key int, value *anilist.Character) bool {
			count++
			return true
		})
	case *result.Cache[int, *anilist.StudioDetails]:
		c.Range(func(key int, value *anilist.StudioDetails) bool {
			count++
			return true
		})
	case *result.Cache[string, *anilist.AnimeAiringSchedule]:
		c.Range(func(key string, value *anilist.AnimeAiringSchedule) bool {
			count++
			return true
		})
	}
	return count
}

func (acm *AnilistCacheManager) GetCacheStats() map[string]interface{} {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	stats := map[string]interface{}{
		"collection_caches": map[string]interface{}{
			"anime_collections":     len(acm.animeCollections),
			"raw_anime_collections": len(acm.rawAnimeCollections),
			"manga_collections":     len(acm.mangaCollections),
			"raw_manga_collections": len(acm.rawMangaCollections),
		},
		"media_caches": map[string]interface{}{
			"base_anime":     acm.countCacheEntries(acm.baseAnimeCache),
			"base_manga":     acm.countCacheEntries(acm.baseMangaCache),
			"complete_anime": acm.countCacheEntries(acm.completeAnimeCache),
			"anime_details":  acm.countCacheEntries(acm.animeDetailsCache),
			"manga_details":  acm.countCacheEntries(acm.mangaDetailsCache),
		},
		"other_caches": map[string]interface{}{
			"characters":      acm.countCacheEntries(acm.characterCache),
			"studios":         acm.countCacheEntries(acm.studioCache),
			"stats_sessions":  len(acm.statsCache),
			"airing_schedule": acm.countCacheEntries(acm.airingScheduleCache),
		},
	}

	return stats
}

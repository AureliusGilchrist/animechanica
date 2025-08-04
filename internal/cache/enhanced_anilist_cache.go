package cache

import (
	"context"
	"seanime/internal/api/anilist"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// EnhancedAnilistCacheManager extends the existing cache with API optimization features
type EnhancedAnilistCacheManager struct {
	*AnilistCacheManager
	
	// API optimization components
	apiOptimizer     *APIOptimizer
	rateLimiter      *SmartRateLimiter
	
	// Request coalescing
	coalescingMu     sync.RWMutex
	pendingRequests  map[string][]chan interface{}
	
	// Prefetching
	prefetchQueue    []PrefetchRequest
	prefetchMu       sync.RWMutex
	prefetchEnabled  bool
	
	// Statistics
	stats            *CacheStats
	statsMu          sync.RWMutex
}

type PrefetchRequest struct {
	Type     string
	ID       int
	Priority RequestPriority
}

type CacheStats struct {
	TotalRequests     int64
	CacheHits         int64
	CacheMisses       int64
	APICallsSaved     int64
	DeduplicatedCalls int64
	BatchedCalls      int64
	PrefetchedItems   int64
	LastReset         time.Time
}

func NewEnhancedAnilistCacheManager(logger *zerolog.Logger, anilistClient anilist.AnilistClient) *EnhancedAnilistCacheManager {
	baseCacheManager := NewAnilistCacheManager(logger)
	
	eacm := &EnhancedAnilistCacheManager{
		AnilistCacheManager: baseCacheManager,
		pendingRequests:     make(map[string][]chan interface{}),
		prefetchQueue:       make([]PrefetchRequest, 0),
		prefetchEnabled:     true,
		stats: &CacheStats{
			LastReset: time.Now(),
		},
	}
	
	eacm.apiOptimizer = NewAPIOptimizer(logger, anilistClient, baseCacheManager)
	eacm.rateLimiter = NewSmartRateLimiter(logger)
	
	// Start background prefetching goroutine
	go eacm.prefetchWorker()
	
	return eacm
}

// GetBaseAnimeEnhanced retrieves anime with all optimizations applied
func (eacm *EnhancedAnilistCacheManager) GetBaseAnimeEnhanced(ctx context.Context, mediaID int, priority RequestPriority) (*anilist.BaseAnime, error) {
	eacm.recordRequest()
	
	// Check cache first
	if cached, found := eacm.GetBaseAnime(mediaID); found {
		eacm.recordCacheHit()
		eacm.logger.Debug().Int("mediaID", mediaID).Msg("Enhanced cache: Anime found in cache")
		return cached, nil
	}
	
	eacm.recordCacheMiss()
	
	// Use request coalescing to prevent duplicate requests
	requestKey := "anime_" + string(rune(mediaID))
	
	eacm.coalescingMu.Lock()
	if channels, exists := eacm.pendingRequests[requestKey]; exists {
		// Request already in progress, wait for result
		ch := make(chan interface{}, 1)
		eacm.pendingRequests[requestKey] = append(channels, ch)
		eacm.coalescingMu.Unlock()
		
		eacm.recordDeduplicatedCall()
		eacm.logger.Debug().Int("mediaID", mediaID).Msg("Enhanced cache: Coalescing anime request")
		
		result := <-ch
		if anime, ok := result.(*anilist.BaseAnime); ok {
			return anime, nil
		}
		return nil, result.(error)
	}
	
	// Start new request
	eacm.pendingRequests[requestKey] = make([]chan interface{}, 0)
	eacm.coalescingMu.Unlock()
	
	// Execute with rate limiting and optimization
	var result *anilist.BaseAnime
	var err error
	
	rateLimitErr := eacm.rateLimiter.ExecuteWithRateLimit(ctx, priority, func() error {
		result, err = eacm.apiOptimizer.GetAnimeOptimized(ctx, mediaID)
		return err
	})
	
	if rateLimitErr != nil {
		err = rateLimitErr
	}
	
	// Notify all waiting requests
	eacm.coalescingMu.Lock()
	channels := eacm.pendingRequests[requestKey]
	delete(eacm.pendingRequests, requestKey)
	eacm.coalescingMu.Unlock()
	
	for _, ch := range channels {
		if err != nil {
			ch <- err
		} else {
			ch <- result
		}
	}
	
	// Schedule related prefetching
	if err == nil && result != nil {
		eacm.schedulePrefetch(ctx, result)
	}
	
	return result, err
}

// GetBaseMangaEnhanced retrieves manga with all optimizations applied
func (eacm *EnhancedAnilistCacheManager) GetBaseMangaEnhanced(ctx context.Context, mediaID int, priority RequestPriority) (*anilist.BaseManga, error) {
	eacm.recordRequest()
	
	// Check cache first
	if cached, found := eacm.GetBaseManga(mediaID); found {
		eacm.recordCacheHit()
		eacm.logger.Debug().Int("mediaID", mediaID).Msg("Enhanced cache: Manga found in cache")
		return cached, nil
	}
	
	eacm.recordCacheMiss()
	
	// Use request coalescing to prevent duplicate requests
	requestKey := "manga_" + string(rune(mediaID))
	
	eacm.coalescingMu.Lock()
	if channels, exists := eacm.pendingRequests[requestKey]; exists {
		// Request already in progress, wait for result
		ch := make(chan interface{}, 1)
		eacm.pendingRequests[requestKey] = append(channels, ch)
		eacm.coalescingMu.Unlock()
		
		eacm.recordDeduplicatedCall()
		eacm.logger.Debug().Int("mediaID", mediaID).Msg("Enhanced cache: Coalescing manga request")
		
		result := <-ch
		if manga, ok := result.(*anilist.BaseManga); ok {
			return manga, nil
		}
		return nil, result.(error)
	}
	
	// Start new request
	eacm.pendingRequests[requestKey] = make([]chan interface{}, 0)
	eacm.coalescingMu.Unlock()
	
	// Execute with rate limiting and optimization
	var result *anilist.BaseManga
	var err error
	
	rateLimitErr := eacm.rateLimiter.ExecuteWithRateLimit(ctx, priority, func() error {
		result, err = eacm.apiOptimizer.GetMangaOptimized(ctx, mediaID)
		return err
	})
	
	if rateLimitErr != nil {
		err = rateLimitErr
	}
	
	// Notify all waiting requests
	eacm.coalescingMu.Lock()
	channels := eacm.pendingRequests[requestKey]
	delete(eacm.pendingRequests, requestKey)
	eacm.coalescingMu.Unlock()
	
	for _, ch := range channels {
		if err != nil {
			ch <- err
		} else {
			ch <- result
		}
	}
	
	return result, err
}

// GetCharacterEnhanced retrieves character with all optimizations applied
func (eacm *EnhancedAnilistCacheManager) GetCharacterEnhanced(ctx context.Context, characterID int, priority RequestPriority) (*anilist.Character, error) {
	eacm.recordRequest()
	
	// Check cache first
	if cached, found := eacm.GetCharacter(characterID); found {
		eacm.recordCacheHit()
		eacm.logger.Debug().Int("characterID", characterID).Msg("Enhanced cache: Character found in cache")
		return cached, nil
	}
	
	eacm.recordCacheMiss()
	
	// Use request coalescing to prevent duplicate requests
	requestKey := "character_" + string(rune(characterID))
	
	eacm.coalescingMu.Lock()
	if channels, exists := eacm.pendingRequests[requestKey]; exists {
		// Request already in progress, wait for result
		ch := make(chan interface{}, 1)
		eacm.pendingRequests[requestKey] = append(channels, ch)
		eacm.coalescingMu.Unlock()
		
		eacm.recordDeduplicatedCall()
		eacm.logger.Debug().Int("characterID", characterID).Msg("Enhanced cache: Coalescing character request")
		
		result := <-ch
		if character, ok := result.(*anilist.Character); ok {
			return character, nil
		}
		return nil, result.(error)
	}
	
	// Start new request
	eacm.pendingRequests[requestKey] = make([]chan interface{}, 0)
	eacm.coalescingMu.Unlock()
	
	// Execute with rate limiting and optimization
	var result *anilist.Character
	var err error
	
	rateLimitErr := eacm.rateLimiter.ExecuteWithRateLimit(ctx, priority, func() error {
		result, err = eacm.apiOptimizer.GetCharacterOptimized(ctx, characterID)
		return err
	})
	
	if rateLimitErr != nil {
		err = rateLimitErr
	}
	
	// Notify all waiting requests
	eacm.coalescingMu.Lock()
	channels := eacm.pendingRequests[requestKey]
	delete(eacm.pendingRequests, requestKey)
	eacm.coalescingMu.Unlock()
	
	for _, ch := range channels {
		if err != nil {
			ch <- err
		} else {
			ch <- result
		}
	}
	
	return result, err
}

// schedulePrefetch schedules related content for prefetching
func (eacm *EnhancedAnilistCacheManager) schedulePrefetch(ctx context.Context, anime *anilist.BaseAnime) {
	if !eacm.prefetchEnabled {
		return
	}
	
	eacm.prefetchMu.Lock()
	defer eacm.prefetchMu.Unlock()
	
	// Prefetch related characters (if available in complete anime data)
	// Note: BaseAnime may not have character data, this would be in CompleteAnime
	// This prefetching will be handled when we fetch complete anime data
	// if anime.Characters != nil && anime.Characters.Edges != nil {
	// 	for _, edge := range anime.Characters.Edges {
	// 		if edge.Node != nil && edge.Node.ID != nil {
	// 			eacm.prefetchQueue = append(eacm.prefetchQueue, PrefetchRequest{
	// 				Type:     "character",
	// 				ID:       *edge.Node.ID,
	// 				Priority: LowPriority,
	// 			})
	// 		}
	// 	}
	// }
	
	// Prefetch related studios (if available in complete anime data)
	// Note: BaseAnime may not have studio data, this would be in CompleteAnime
	// This prefetching will be handled when we fetch complete anime data
	// if anime.Studios != nil && anime.Studios.Edges != nil {
	// 	for _, edge := range anime.Studios.Edges {
	// 		if edge.Node != nil && edge.Node.ID != nil {
	// 			eacm.prefetchQueue = append(eacm.prefetchQueue, PrefetchRequest{
	// 				Type:     "studio",
	// 				ID:       *edge.Node.ID,
	// 				Priority: LowPriority,
	// 			})
	// 		}
	// 	}
	// }
	
	eacm.logger.Debug().Int("queueSize", len(eacm.prefetchQueue)).Msg("Enhanced cache: Scheduled prefetch requests")
}

// prefetchWorker processes prefetch requests in the background
func (eacm *EnhancedAnilistCacheManager) prefetchWorker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		eacm.processPrefetchQueue()
	}
}

// processPrefetchQueue processes items from the prefetch queue
func (eacm *EnhancedAnilistCacheManager) processPrefetchQueue() {
	eacm.prefetchMu.Lock()
	if len(eacm.prefetchQueue) == 0 {
		eacm.prefetchMu.Unlock()
		return
	}
	
	// Process up to 3 items at a time
	batchSize := minInt(len(eacm.prefetchQueue), 3)
	batch := make([]PrefetchRequest, batchSize)
	copy(batch, eacm.prefetchQueue[:batchSize])
	eacm.prefetchQueue = eacm.prefetchQueue[batchSize:]
	eacm.prefetchMu.Unlock()
	
	ctx := context.Background()
	
	for _, req := range batch {
		switch req.Type {
		case "character":
			if _, found := eacm.GetCharacter(req.ID); !found {
				go func(id int) {
					_, err := eacm.GetCharacterEnhanced(ctx, id, LowPriority)
					if err == nil {
						eacm.recordPrefetchedItem()
					}
				}(req.ID)
			}
		case "studio":
			if _, found := eacm.GetStudio(req.ID); !found {
				// Prefetch studio if needed (implementation would be similar)
				eacm.recordPrefetchedItem()
			}
		}
	}
}

// Statistics recording methods
func (eacm *EnhancedAnilistCacheManager) recordRequest() {
	eacm.statsMu.Lock()
	defer eacm.statsMu.Unlock()
	eacm.stats.TotalRequests++
}

func (eacm *EnhancedAnilistCacheManager) recordCacheHit() {
	eacm.statsMu.Lock()
	defer eacm.statsMu.Unlock()
	eacm.stats.CacheHits++
	eacm.stats.APICallsSaved++
}

func (eacm *EnhancedAnilistCacheManager) recordCacheMiss() {
	eacm.statsMu.Lock()
	defer eacm.statsMu.Unlock()
	eacm.stats.CacheMisses++
}

func (eacm *EnhancedAnilistCacheManager) recordDeduplicatedCall() {
	eacm.statsMu.Lock()
	defer eacm.statsMu.Unlock()
	eacm.stats.DeduplicatedCalls++
	eacm.stats.APICallsSaved++
}

func (eacm *EnhancedAnilistCacheManager) recordBatchedCall() {
	eacm.statsMu.Lock()
	defer eacm.statsMu.Unlock()
	eacm.stats.BatchedCalls++
}

func (eacm *EnhancedAnilistCacheManager) recordPrefetchedItem() {
	eacm.statsMu.Lock()
	defer eacm.statsMu.Unlock()
	eacm.stats.PrefetchedItems++
}

// GetEnhancedCacheStats returns comprehensive statistics about the enhanced cache
func (eacm *EnhancedAnilistCacheManager) GetEnhancedCacheStats() map[string]interface{} {
	eacm.statsMu.RLock()
	stats := *eacm.stats
	eacm.statsMu.RUnlock()
	
	// Calculate cache hit rate
	hitRate := 0.0
	if stats.TotalRequests > 0 {
		hitRate = float64(stats.CacheHits) / float64(stats.TotalRequests) * 100
	}
	
	// Get base cache stats
	baseCacheStats := eacm.AnilistCacheManager.GetCacheStats()
	
	// Get optimizer and rate limiter stats
	optimizerStats := eacm.apiOptimizer.GetOptimizationStats()
	rateLimiterStats := eacm.rateLimiter.GetQueueStats()
	
	return map[string]interface{}{
		"performance_stats": map[string]interface{}{
			"total_requests":      stats.TotalRequests,
			"cache_hits":          stats.CacheHits,
			"cache_misses":        stats.CacheMisses,
			"cache_hit_rate":      hitRate,
			"api_calls_saved":     stats.APICallsSaved,
			"deduplicated_calls":  stats.DeduplicatedCalls,
			"batched_calls":       stats.BatchedCalls,
			"prefetched_items":    stats.PrefetchedItems,
			"stats_since":         stats.LastReset,
		},
		"cache_contents":   baseCacheStats,
		"api_optimizer":    optimizerStats,
		"rate_limiter":     rateLimiterStats,
		"prefetch_queue":   len(eacm.prefetchQueue),
		"pending_requests": len(eacm.pendingRequests),
	}
}

// ResetStats resets all performance statistics
func (eacm *EnhancedAnilistCacheManager) ResetStats() {
	eacm.statsMu.Lock()
	defer eacm.statsMu.Unlock()
	
	eacm.stats = &CacheStats{
		LastReset: time.Now(),
	}
	
	eacm.logger.Info().Msg("Enhanced cache: Statistics reset")
}

// SetPrefetchEnabled enables or disables prefetching
func (eacm *EnhancedAnilistCacheManager) SetPrefetchEnabled(enabled bool) {
	eacm.prefetchMu.Lock()
	defer eacm.prefetchMu.Unlock()
	
	eacm.prefetchEnabled = enabled
	if !enabled {
		eacm.prefetchQueue = eacm.prefetchQueue[:0] // Clear queue
	}
	
	eacm.logger.Info().Bool("enabled", enabled).Msg("Enhanced cache: Prefetching toggled")
}

// Helper function for minInt
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

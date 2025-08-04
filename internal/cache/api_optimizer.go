package cache

import (
	"context"
	"seanime/internal/api/anilist"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// APIOptimizer provides intelligent batching and deduplication for API calls
type APIOptimizer struct {
	logger *zerolog.Logger
	mu     sync.RWMutex

	// Pending request deduplication
	pendingAnimeRequests map[int]chan *anilist.BaseAnime
	pendingMangaRequests map[int]chan *anilist.BaseManga
	pendingCharacterRequests map[int]chan *anilist.Character
	pendingStudioRequests map[int]chan *anilist.StudioDetails

	// Batch processing queues
	animeBatchQueue    []int
	mangaBatchQueue    []int
	characterBatchQueue []int
	studioBatchQueue    []int

	// Batch processing timers
	animeBatchTimer    *time.Timer
	mangaBatchTimer    *time.Timer
	characterBatchTimer *time.Timer
	studioBatchTimer    *time.Timer

	// Configuration
	batchSize    int
	batchTimeout time.Duration

	// Dependencies
	anilistClient anilist.AnilistClient
	cacheManager  *AnilistCacheManager
}

func NewAPIOptimizer(logger *zerolog.Logger, anilistClient anilist.AnilistClient, cacheManager *AnilistCacheManager) *APIOptimizer {
	return &APIOptimizer{
		logger: logger,
		
		pendingAnimeRequests:     make(map[int]chan *anilist.BaseAnime),
		pendingMangaRequests:     make(map[int]chan *anilist.BaseManga),
		pendingCharacterRequests: make(map[int]chan *anilist.Character),
		pendingStudioRequests:    make(map[int]chan *anilist.StudioDetails),

		animeBatchQueue:     make([]int, 0),
		mangaBatchQueue:     make([]int, 0),
		characterBatchQueue: make([]int, 0),
		studioBatchQueue:    make([]int, 0),

		batchSize:    10,  // Process up to 10 items in a batch
		batchTimeout: 500 * time.Millisecond, // Wait max 500ms before processing batch

		anilistClient: anilistClient,
		cacheManager:  cacheManager,
	}
}

// GetAnimeOptimized retrieves anime with intelligent caching and deduplication
func (ao *APIOptimizer) GetAnimeOptimized(ctx context.Context, mediaID int) (*anilist.BaseAnime, error) {
	// Check cache first
	if cached, found := ao.cacheManager.GetBaseAnime(mediaID); found {
		ao.logger.Debug().Int("mediaID", mediaID).Msg("API optimizer: Anime found in cache")
		return cached, nil
	}

	ao.mu.Lock()
	defer ao.mu.Unlock()

	// Check if request is already pending (deduplication)
	if ch, exists := ao.pendingAnimeRequests[mediaID]; exists {
		ao.logger.Debug().Int("mediaID", mediaID).Msg("API optimizer: Anime request already pending, waiting")
		ao.mu.Unlock()
		result := <-ch
		ao.mu.Lock()
		return result, nil
	}

	// Create channel for this request
	ch := make(chan *anilist.BaseAnime, 1)
	ao.pendingAnimeRequests[mediaID] = ch

	// Add to batch queue
	ao.animeBatchQueue = append(ao.animeBatchQueue, mediaID)

	// Start batch timer if not already running
	if ao.animeBatchTimer == nil {
		ao.animeBatchTimer = time.AfterFunc(ao.batchTimeout, func() {
			ao.processAnimeBatch(ctx)
		})
	}

	// Process immediately if batch is full
	if len(ao.animeBatchQueue) >= ao.batchSize {
		if ao.animeBatchTimer != nil {
			ao.animeBatchTimer.Stop()
			ao.animeBatchTimer = nil
		}
		go ao.processAnimeBatch(ctx)
	}

	ao.mu.Unlock()
	result := <-ch
	ao.mu.Lock()
	
	return result, nil
}

// processAnimeBatch processes a batch of anime requests
func (ao *APIOptimizer) processAnimeBatch(ctx context.Context) {
	ao.mu.Lock()
	defer ao.mu.Unlock()

	if len(ao.animeBatchQueue) == 0 {
		return
	}

	batch := make([]int, len(ao.animeBatchQueue))
	copy(batch, ao.animeBatchQueue)
	ao.animeBatchQueue = ao.animeBatchQueue[:0] // Clear queue
	ao.animeBatchTimer = nil

	ao.logger.Debug().Int("batchSize", len(batch)).Msg("API optimizer: Processing anime batch")

	// Process each item in the batch
	for _, mediaID := range batch {
		go func(id int) {
			anime, err := ao.anilistClient.BaseAnimeByID(ctx, &id)
			
			ao.mu.Lock()
			ch, exists := ao.pendingAnimeRequests[id]
			if exists {
				delete(ao.pendingAnimeRequests, id)
			}
			ao.mu.Unlock()

			if err != nil {
				ao.logger.Error().Err(err).Int("mediaID", id).Msg("API optimizer: Failed to fetch anime")
				if exists {
					ch <- nil
				}
				return
			}

			// Cache the result
			if anime != nil && anime.Media != nil {
				ao.cacheManager.SetBaseAnime(id, anime.Media)
				if exists {
					ch <- anime.Media
				}
			} else {
				if exists {
					ch <- nil
				}
			}
		}(mediaID)
	}
}

// GetMangaOptimized retrieves manga with intelligent caching and deduplication
func (ao *APIOptimizer) GetMangaOptimized(ctx context.Context, mediaID int) (*anilist.BaseManga, error) {
	// Check cache first
	if cached, found := ao.cacheManager.GetBaseManga(mediaID); found {
		ao.logger.Debug().Int("mediaID", mediaID).Msg("API optimizer: Manga found in cache")
		return cached, nil
	}

	ao.mu.Lock()
	defer ao.mu.Unlock()

	// Check if request is already pending (deduplication)
	if ch, exists := ao.pendingMangaRequests[mediaID]; exists {
		ao.logger.Debug().Int("mediaID", mediaID).Msg("API optimizer: Manga request already pending, waiting")
		ao.mu.Unlock()
		result := <-ch
		ao.mu.Lock()
		return result, nil
	}

	// Create channel for this request
	ch := make(chan *anilist.BaseManga, 1)
	ao.pendingMangaRequests[mediaID] = ch

	// Add to batch queue
	ao.mangaBatchQueue = append(ao.mangaBatchQueue, mediaID)

	// Start batch timer if not already running
	if ao.mangaBatchTimer == nil {
		ao.mangaBatchTimer = time.AfterFunc(ao.batchTimeout, func() {
			ao.processMangaBatch(ctx)
		})
	}

	// Process immediately if batch is full
	if len(ao.mangaBatchQueue) >= ao.batchSize {
		if ao.mangaBatchTimer != nil {
			ao.mangaBatchTimer.Stop()
			ao.mangaBatchTimer = nil
		}
		go ao.processMangaBatch(ctx)
	}

	ao.mu.Unlock()
	result := <-ch
	ao.mu.Lock()
	
	return result, nil
}

// processMangaBatch processes a batch of manga requests
func (ao *APIOptimizer) processMangaBatch(ctx context.Context) {
	ao.mu.Lock()
	defer ao.mu.Unlock()

	if len(ao.mangaBatchQueue) == 0 {
		return
	}

	batch := make([]int, len(ao.mangaBatchQueue))
	copy(batch, ao.mangaBatchQueue)
	ao.mangaBatchQueue = ao.mangaBatchQueue[:0] // Clear queue
	ao.mangaBatchTimer = nil

	ao.logger.Debug().Int("batchSize", len(batch)).Msg("API optimizer: Processing manga batch")

	// Process each item in the batch
	for _, mediaID := range batch {
		go func(id int) {
			manga, err := ao.anilistClient.BaseMangaByID(ctx, &id)
			
			ao.mu.Lock()
			ch, exists := ao.pendingMangaRequests[id]
			if exists {
				delete(ao.pendingMangaRequests, id)
			}
			ao.mu.Unlock()

			if err != nil {
				ao.logger.Error().Err(err).Int("mediaID", id).Msg("API optimizer: Failed to fetch manga")
				if exists {
					ch <- nil
				}
				return
			}

			// Cache the result
			if manga != nil && manga.Media != nil {
				ao.cacheManager.SetBaseManga(id, manga.Media)
				if exists {
					ch <- manga.Media
				}
			} else {
				if exists {
					ch <- nil
				}
			}
		}(mediaID)
	}
}

// GetCharacterOptimized retrieves character with intelligent caching and deduplication
func (ao *APIOptimizer) GetCharacterOptimized(ctx context.Context, characterID int) (*anilist.Character, error) {
	// Check cache first
	if cached, found := ao.cacheManager.GetCharacter(characterID); found {
		ao.logger.Debug().Int("characterID", characterID).Msg("API optimizer: Character found in cache")
		return cached, nil
	}

	ao.mu.Lock()
	defer ao.mu.Unlock()

	// Check if request is already pending (deduplication)
	if ch, exists := ao.pendingCharacterRequests[characterID]; exists {
		ao.logger.Debug().Int("characterID", characterID).Msg("API optimizer: Character request already pending, waiting")
		ao.mu.Unlock()
		result := <-ch
		ao.mu.Lock()
		return result, nil
	}

	// Create channel for this request
	ch := make(chan *anilist.Character, 1)
	ao.pendingCharacterRequests[characterID] = ch

	// For characters, make individual requests since they're less frequently accessed
	go func() {
		character, err := ao.anilistClient.CharacterDetails(ctx, &characterID)
		
		ao.mu.Lock()
		delete(ao.pendingCharacterRequests, characterID)
		ao.mu.Unlock()

		if err != nil {
			ao.logger.Error().Err(err).Int("characterID", characterID).Msg("API optimizer: Failed to fetch character")
			ch <- nil
			return
		}

		// Cache the result
		if character != nil {
			ao.cacheManager.SetCharacter(characterID, character)
			ch <- character
		} else {
			ch <- nil
		}
	}()

	ao.mu.Unlock()
	result := <-ch
	ao.mu.Lock()
	
	return result, nil
}

// GetOptimizationStats returns statistics about the API optimizer
func (ao *APIOptimizer) GetOptimizationStats() map[string]interface{} {
	ao.mu.RLock()
	defer ao.mu.RUnlock()

	return map[string]interface{}{
		"pending_requests": map[string]int{
			"anime":     len(ao.pendingAnimeRequests),
			"manga":     len(ao.pendingMangaRequests),
			"character": len(ao.pendingCharacterRequests),
			"studio":    len(ao.pendingStudioRequests),
		},
		"batch_queues": map[string]int{
			"anime":     len(ao.animeBatchQueue),
			"manga":     len(ao.mangaBatchQueue),
			"character": len(ao.characterBatchQueue),
			"studio":    len(ao.studioBatchQueue),
		},
		"configuration": map[string]interface{}{
			"batch_size":    ao.batchSize,
			"batch_timeout": ao.batchTimeout.String(),
		},
	}
}

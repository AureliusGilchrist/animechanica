//go:build disabled
// +build disabled

package anime

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"seanime/internal/events"
	"seanime/internal/extension"
	"seanime/internal/library/anime"
	"seanime/internal/util"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	hibikeanime "github.com/5rahim/hibike/pkg/extension/anime"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

type (
	// EpisodeContainer holds episode data for a specific anime
	EpisodeContainer struct {
		MediaId   int                              `json:"mediaId"`
		Provider  string                           `json:"provider"`
		Episodes  []*hibikeanime.EpisodeDetails    `json:"episodes"`
		CreatedAt time.Time                        `json:"createdAt"`
	}

	// EpisodePageContainer holds episode page/stream data
	EpisodePageContainer struct {
		Provider     string                           `json:"provider"`
		MediaId      int                              `json:"mediaId"`
		EpisodeId    string                           `json:"episodeId"`
		StreamLinks  []*hibikeanime.EpisodeStreamLink `json:"streamLinks"`
		SubtitleLinks []*hibikeanime.SubtitleLink     `json:"subtitleLinks"`
		CreatedAt    time.Time                        `json:"createdAt"`
	}

	// AnimeLatestEpisodeNumberItem represents latest episode info for a provider
	AnimeLatestEpisodeNumberItem struct {
		Provider string `json:"provider"`
		Language string `json:"language"`
		Number   int    `json:"number"`
	}

	// AnimeLatestEpisodeNumbersMapEvent is emitted when latest episode numbers are updated
	AnimeLatestEpisodeNumbersMapEvent struct {
		LatestEpisodeNumbersMap map[int][]AnimeLatestEpisodeNumberItem `json:"latestEpisodeNumbersMap"`
	}
)

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Episode Container
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SearchEpisodes searches for episodes of an anime using the given provider
func (r *Repository) SearchEpisodes(ctx context.Context, provider string, mediaId int, titles []string) (*EpisodeContainer, error) {
	if len(titles) == 0 {
		return nil, ErrNoTitlesProvided
	}

	r.logger.Debug().
		Str("provider", provider).
		Int("mediaId", mediaId).
		Strs("titles", titles).
		Msg("anime: Searching for episodes")

	// Check cache first
	bucket := r.getFcProviderBucket(provider, mediaId, bucketTypeEpisode)
	cached, found := bucket.Get(bucketTypeEpisodeKey)
	if found {
		var container EpisodeContainer
		if bucket.Unmarshal(cached, &container) == nil {
			// Check if cache is still valid (24 hours)
			if time.Since(container.CreatedAt) < 24*time.Hour {
				r.logger.Debug().
					Str("provider", provider).
					Int("mediaId", mediaId).
					Msg("anime: Using cached episodes")
				return &container, nil
			}
		}
	}

	// Get provider extension
	ext := r.providerExtensionBank.GetAnimeExtension(provider)
	if ext == nil {
		return nil, fmt.Errorf("anime provider extension not found: %s", provider)
	}

	// Search for anime
	searchResults, err := ext.Search(hibikeanime.AnimeSearchOptions{
		Query: titles[0],
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search anime: %w", err)
	}

	if len(searchResults) == 0 {
		return nil, ErrNoResults
	}

	// Find best match
	bestMatch := findBestAnimeMatch(searchResults, titles)
	if bestMatch == nil {
		return nil, ErrNoResults
	}

	// Get episodes
	episodes, err := ext.FindEpisodes(bestMatch.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get episodes: %w", err)
	}

	if len(episodes) == 0 {
		return nil, ErrNoEpisodes
	}

	// Create container
	container := &EpisodeContainer{
		MediaId:   mediaId,
		Provider:  provider,
		Episodes:  episodes,
		CreatedAt: time.Now(),
	}

	// Cache the result
	if err := bucket.Set(bucketTypeEpisodeKey, container, time.Hour*24); err != nil {
		r.logger.Warn().Err(err).Msg("anime: Failed to cache episodes")
	}

	r.logger.Info().
		Str("provider", provider).
		Int("mediaId", mediaId).
		Int("episodeCount", len(episodes)).
		Msg("anime: Found episodes")

	return container, nil
}

// GetEpisodeContainer retrieves cached episode container
func (r *Repository) GetEpisodeContainer(provider string, mediaId int) (*EpisodeContainer, bool) {
	bucket := r.getFcProviderBucket(provider, mediaId, bucketTypeEpisode)
	cached, found := bucket.Get(bucketTypeEpisodeKey)
	if !found {
		return nil, false
	}

	var container EpisodeContainer
	if err := bucket.Unmarshal(cached, &container); err != nil {
		return nil, false
	}

	return &container, true
}

// findBestAnimeMatch finds the best matching anime from search results
func findBestAnimeMatch(results []*hibikeanime.AnimeSearchResult, titles []string) *hibikeanime.AnimeSearchResult {
	if len(results) == 0 {
		return nil
	}

	// Score each result
	bestScore := -1.0
	var bestMatch *hibikeanime.AnimeSearchResult

	for _, result := range results {
		score := 0.0
		resultTitle := strings.ToLower(result.Title)

		for _, title := range titles {
			titleLower := strings.ToLower(title)
			
			// Exact match gets highest score
			if resultTitle == titleLower {
				score += 100.0
				break
			}

			// Partial matches
			if strings.Contains(resultTitle, titleLower) {
				score += 50.0
			} else if strings.Contains(titleLower, resultTitle) {
				score += 40.0
			}

			// Fuzzy similarity
			similarity := util.LevenshteinSimilarity(resultTitle, titleLower)
			score += similarity * 30.0
		}

		if score > bestScore {
			bestScore = score
			bestMatch = result
		}
	}

	return bestMatch
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Episode Page Container
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetEpisodeStreamLinks gets stream links for a specific episode
func (r *Repository) GetEpisodeStreamLinks(ctx context.Context, provider string, mediaId int, episodeId string) (*EpisodePageContainer, error) {
	r.logger.Debug().
		Str("provider", provider).
		Int("mediaId", mediaId).
		Str("episodeId", episodeId).
		Msg("anime: Getting episode stream links")

	// Check cache first
	bucket := r.getFcProviderBucket(provider, mediaId, bucketTypeVideo)
	cacheKey := fmt.Sprintf("episode_%s", episodeId)
	cached, found := bucket.Get(cacheKey)
	if found {
		var container EpisodePageContainer
		if bucket.Unmarshal(cached, &container) == nil {
			// Check if cache is still valid (6 hours)
			if time.Since(container.CreatedAt) < 6*time.Hour {
				r.logger.Debug().
					Str("provider", provider).
					Str("episodeId", episodeId).
					Msg("anime: Using cached stream links")
				return &container, nil
			}
		}
	}

	// Get provider extension
	ext := r.providerExtensionBank.GetAnimeExtension(provider)
	if ext == nil {
		return nil, fmt.Errorf("anime provider extension not found: %s", provider)
	}

	// Get stream links
	streamLinks, err := ext.FindEpisodeStreamLinks(episodeId)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream links: %w", err)
	}

	// Get subtitles if available
	var subtitleLinks []*hibikeanime.SubtitleLink
	if subtitleExt, ok := ext.(hibikeanime.SubtitleProvider); ok {
		subtitleLinks, _ = subtitleExt.FindEpisodeSubtitles(episodeId)
	}

	// Create container
	container := &EpisodePageContainer{
		Provider:      provider,
		MediaId:       mediaId,
		EpisodeId:     episodeId,
		StreamLinks:   streamLinks,
		SubtitleLinks: subtitleLinks,
		CreatedAt:     time.Now(),
	}

	// Cache the result
	if err := bucket.Set(cacheKey, container, time.Hour*6); err != nil {
		r.logger.Warn().Err(err).Msg("anime: Failed to cache stream links")
	}

	r.logger.Info().
		Str("provider", provider).
		Str("episodeId", episodeId).
		Int("streamCount", len(streamLinks)).
		Int("subtitleCount", len(subtitleLinks)).
		Msg("anime: Found stream links")

	return container, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Latest Episode Numbers
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetAnimeLatestEpisodeNumbersMap gets the latest episode numbers for all anime
func (r *Repository) GetAnimeLatestEpisodeNumbersMap(mediaIds []int) map[int][]AnimeLatestEpisodeNumberItem {
	ret := make(map[int][]AnimeLatestEpisodeNumberItem)

	if r.providerExtensionBank == nil {
		return ret
	}

	animeExtensions := r.providerExtensionBank.GetAnimeExtensions()
	
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, mediaId := range mediaIds {
		wg.Add(1)
		go func(mId int) {
			defer wg.Done()
			
			for provider, ext := range animeExtensions {
				// Get cached episodes
				container, found := r.GetEpisodeContainer(provider, mId)
				if !found {
					continue
				}

				episodes := container.Episodes
				if len(episodes) == 0 {
					continue
				}

				// Find latest episode
				lastEpisode := slices.MaxFunc(episodes, func(a *hibikeanime.EpisodeDetails, b *hibikeanime.EpisodeDetails) int {
					return cmp.Compare(a.Number, b.Number)
				})

				if lastEpisode == nil {
					continue
				}

				episodeNumFloat, _ := strconv.ParseFloat(fmt.Sprintf("%.1f", lastEpisode.Number), 32)
				episodeCount := int(math.Floor(episodeNumFloat))

				mu.Lock()
				if _, ok := ret[mId]; !ok {
					ret[mId] = []AnimeLatestEpisodeNumberItem{}
				}

				ret[mId] = append(ret[mId], AnimeLatestEpisodeNumberItem{
					Provider: provider,
					Language: lastEpisode.Language,
					Number:   episodeCount,
				})
				mu.Unlock()
			}
		}(mediaId)
	}

	wg.Wait()

	// Trigger hook event
	ev := &AnimeLatestEpisodeNumbersMapEvent{
		LatestEpisodeNumbersMap: ret,
	}

	if r.wsEventManager != nil {
		r.wsEventManager.SendEvent(events.AnimeLatestEpisodeNumbers, ev)
	}

	return ret
}

// RefreshAnimeLatestEpisodeNumbers refreshes latest episode data for specific anime
func (r *Repository) RefreshAnimeLatestEpisodeNumbers(mediaIds []int) {
	if r.providerExtensionBank == nil {
		return
	}

	animeExtensions := r.providerExtensionBank.GetAnimeExtensions()
	
	for _, mediaId := range mediaIds {
		for provider := range animeExtensions {
			// Clear cache to force refresh
			bucket := r.getFcProviderBucket(provider, mediaId, bucketTypeEpisode)
			_ = bucket.Delete(bucketTypeEpisodeKey)
		}
	}

	r.logger.Info().
		Ints("mediaIds", mediaIds).
		Msg("anime: Cleared episode cache for refresh")
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Helpers
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetAvailableProviders returns list of available anime providers
func (r *Repository) GetAvailableProviders() []string {
	if r.providerExtensionBank == nil {
		return []string{}
	}

	extensions := r.providerExtensionBank.GetAnimeExtensions()
	providers := make([]string, 0, len(extensions))
	
	for provider := range extensions {
		providers = append(providers, provider)
	}

	return providers
}

// GetProviderInfo returns information about a specific provider
func (r *Repository) GetProviderInfo(provider string) map[string]interface{} {
	if r.providerExtensionBank == nil {
		return nil
	}

	ext := r.providerExtensionBank.GetAnimeExtension(provider)
	if ext == nil {
		return nil
	}

	info := map[string]interface{}{
		"name":     provider,
		"type":     "anime",
		"features": []string{"search", "episodes", "streaming"},
	}

	// Check for additional features
	if _, ok := ext.(hibikeanime.SubtitleProvider); ok {
		features := info["features"].([]string)
		info["features"] = append(features, "subtitles")
	}

	return info
}

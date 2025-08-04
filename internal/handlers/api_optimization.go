package handlers

import (
	"net/http"
	"seanime/internal/cache"
	"strconv"

	"github.com/labstack/echo/v4"
)

// HandleGetAPIOptimizationStats returns comprehensive API optimization statistics
func (h *Handler) HandleGetAPIOptimizationStats(c echo.Context) error {
	// Get session ID for authentication
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Session required"})
	}

	// Get enhanced cache manager if available
	if h.App.EnhancedAnilistCacheManager != nil {
		stats := h.App.EnhancedAnilistCacheManager.GetEnhancedCacheStats()
		return c.JSON(http.StatusOK, stats)
	}

	// Fallback to basic cache stats
	if h.App.AnilistCacheManager != nil {
		stats := h.App.AnilistCacheManager.GetCacheStats()
		return c.JSON(http.StatusOK, map[string]interface{}{
			"cache_contents": stats,
			"message":        "Enhanced optimization not available, showing basic cache stats",
		})
	}

	return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Cache manager not available"})
}

// HandleResetAPIOptimizationStats resets API optimization statistics
func (h *Handler) HandleResetAPIOptimizationStats(c echo.Context) error {
	// Get session ID for authentication
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Session required"})
	}

	// Reset enhanced cache stats if available
	if h.App.EnhancedAnilistCacheManager != nil {
		h.App.EnhancedAnilistCacheManager.ResetStats()
		return c.JSON(http.StatusOK, map[string]string{"message": "API optimization statistics reset successfully"})
	}

	return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Enhanced cache manager not available"})
}

// HandleTogglePrefetching enables or disables prefetching
func (h *Handler) HandleTogglePrefetching(c echo.Context) error {
	// Get session ID for authentication
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Session required"})
	}

	type TogglePrefetchingRequest struct {
		Enabled bool `json:"enabled"`
	}

	var body TogglePrefetchingRequest
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Toggle prefetching if enhanced cache manager is available
	if h.App.EnhancedAnilistCacheManager != nil {
		h.App.EnhancedAnilistCacheManager.SetPrefetchEnabled(body.Enabled)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Prefetching toggled successfully",
			"enabled": body.Enabled,
		})
	}

	return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Enhanced cache manager not available"})
}

// HandleGetCacheConfiguration returns current cache configuration
func (h *Handler) HandleGetCacheConfiguration(c echo.Context) error {
	// Get session ID for authentication
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Session required"})
	}

	config := map[string]interface{}{
		"cache_expiration_times": map[string]string{
			"collections":      "2 hours",
			"individual_media": "6 hours",
			"characters":       "1 week",
			"stats":            "4 hours",
			"airing_schedule":  "1 hour",
		},
		"optimization_features": map[string]interface{}{
			"request_batching":    true,
			"request_coalescing":  true,
			"smart_rate_limiting": true,
			"prefetching":         h.App.EnhancedAnilistCacheManager != nil,
			"deduplication":       true,
		},
		"rate_limiting": map[string]interface{}{
			"requests_per_minute": 90,
			"burst_size":          10,
			"adaptive_backoff":    true,
		},
	}

	return c.JSON(http.StatusOK, config)
}

// HandleOptimizeSpecificMedia triggers optimization for specific media
func (h *Handler) HandleOptimizeSpecificMedia(c echo.Context) error {
	// Get session ID for authentication
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Session required"})
	}

	mediaIDStr := c.Param("id")
	mediaID, err := strconv.Atoi(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid media ID"})
	}

	mediaType := c.QueryParam("type") // "anime" or "manga"
	if mediaType != "anime" && mediaType != "manga" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Media type must be 'anime' or 'manga'"})
	}

	// Use enhanced cache manager if available
	if h.App.EnhancedAnilistCacheManager != nil {
		ctx := c.Request().Context()
		
		if mediaType == "anime" {
			anime, err := h.App.EnhancedAnilistCacheManager.GetBaseAnimeEnhanced(ctx, mediaID, cache.HighPriority)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]interface{}{
				"message": "Anime optimized successfully",
				"media":   anime,
			})
		} else {
			manga, err := h.App.EnhancedAnilistCacheManager.GetBaseMangaEnhanced(ctx, mediaID, cache.HighPriority)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]interface{}{
				"message": "Manga optimized successfully",
				"media":   manga,
			})
		}
	}

	return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Enhanced cache manager not available"})
}

// HandleBulkOptimizeMedia triggers optimization for multiple media items
func (h *Handler) HandleBulkOptimizeMedia(c echo.Context) error {
	// Get session ID for authentication
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Session required"})
	}

	type BulkOptimizeRequest struct {
		MediaIDs []int  `json:"mediaIds"`
		Type     string `json:"type"` // "anime" or "manga"
		Priority string `json:"priority,omitempty"` // "high", "normal", "low"
	}

	var body BulkOptimizeRequest
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if body.Type != "anime" && body.Type != "manga" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Media type must be 'anime' or 'manga'"})
	}

	if len(body.MediaIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No media IDs provided"})
	}

	if len(body.MediaIDs) > 50 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Too many media IDs (max 50)"})
	}

	// Determine priority
	priority := cache.NormalPriority
	switch body.Priority {
	case "high":
		priority = cache.HighPriority
	case "low":
		priority = cache.LowPriority
	}

	// Use enhanced cache manager if available
	if h.App.EnhancedAnilistCacheManager != nil {
		ctx := c.Request().Context()
		
		// Process requests asynchronously
		go func() {
			for _, mediaID := range body.MediaIDs {
				if body.Type == "anime" {
					_, err := h.App.EnhancedAnilistCacheManager.GetBaseAnimeEnhanced(ctx, mediaID, priority)
					if err != nil {
						h.App.Logger.Error().Err(err).Int("mediaID", mediaID).Msg("Bulk optimize: Failed to optimize anime")
					}
				} else {
					_, err := h.App.EnhancedAnilistCacheManager.GetBaseMangaEnhanced(ctx, mediaID, priority)
					if err != nil {
						h.App.Logger.Error().Err(err).Int("mediaID", mediaID).Msg("Bulk optimize: Failed to optimize manga")
					}
				}
			}
		}()

		return c.JSON(http.StatusAccepted, map[string]interface{}{
			"message":     "Bulk optimization started",
			"media_count": len(body.MediaIDs),
			"type":        body.Type,
			"priority":    body.Priority,
		})
	}

	return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Enhanced cache manager not available"})
}

// HandleGetAPICallHistory returns recent API call history and patterns
func (h *Handler) HandleGetAPICallHistory(c echo.Context) error {
	// Get session ID for authentication
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Session required"})
	}

	// This would return historical data about API calls, rate limiting, etc.
	// For now, return current state information
	history := map[string]interface{}{
		"message": "API call history tracking not yet implemented",
		"current_stats": map[string]interface{}{
			"timestamp": "now",
			"note":      "Historical tracking will be added in future updates",
		},
	}

	if h.App.EnhancedAnilistCacheManager != nil {
		stats := h.App.EnhancedAnilistCacheManager.GetEnhancedCacheStats()
		history["current_performance"] = stats["performance_stats"]
	}

	return c.JSON(http.StatusOK, history)
}

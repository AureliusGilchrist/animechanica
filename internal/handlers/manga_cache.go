package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Manga Index Cache Management
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// HandleGetMangaCacheStats returns statistics about the manga index cache
func (h *Handler) HandleGetMangaCacheStats(c echo.Context) error {
	if h.App.MangaRepository == nil || h.App.MangaDownloader == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"error": "Manga service not available",
		})
	}

	stats := h.App.MangaDownloader.GetMangaIndexCacheStats()
	
	h.App.Logger.Debug().Interface("stats", stats).Msg("manga cache: Retrieved cache statistics")
	
	return c.JSON(http.StatusOK, stats)
}

// HandleClearMangaCache clears the manga index cache
func (h *Handler) HandleClearMangaCache(c echo.Context) error {
	if h.App.MangaRepository == nil || h.App.MangaDownloader == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"error": "Manga service not available",
		})
	}

	if err := h.App.MangaDownloader.InvalidateMangaIndexCache(); err != nil {
		h.App.Logger.Error().Err(err).Msg("manga cache: Failed to clear manga index cache")
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to clear manga index cache",
		})
	}

	h.App.Logger.Info().Msg("manga cache: Manga index cache cleared successfully")
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Manga index cache cleared successfully",
	})
}

// HandleRefreshMangaIndex forces a refresh of the manga index, bypassing cache
func (h *Handler) HandleRefreshMangaIndex(c echo.Context) error {
	if h.App.MangaRepository == nil || h.App.MangaDownloader == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"error": "Manga service not available",
		})
	}

	// Trigger refresh in background
	go h.App.MangaDownloader.RefreshMangaIndex()

	h.App.Logger.Info().Msg("manga cache: Manga index refresh triggered")
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Manga index refresh triggered",
	})
}

// HandleGetAllCacheStats returns comprehensive cache statistics including both AniList and manga caches
func (h *Handler) HandleGetAllCacheStats(c echo.Context) error {
	allStats := make(map[string]interface{})

	// Get AniList cache stats
	if h.App.AnilistCacheManager != nil {
		anilistStats := h.App.AnilistCacheManager.GetCacheStats()
		allStats["anilist"] = anilistStats
	}

	// Get manga cache stats
	if h.App.MangaDownloader != nil {
		mangaStats := h.App.MangaDownloader.GetMangaIndexCacheStats()
		allStats["manga"] = mangaStats
	}

	h.App.Logger.Debug().Interface("stats", allStats).Msg("cache: Retrieved comprehensive cache statistics")
	
	return c.JSON(http.StatusOK, allStats)
}

// HandleClearAllSystemCaches clears both AniList and manga caches
func (h *Handler) HandleClearAllSystemCaches(c echo.Context) error {
	errors := make([]string, 0)

	// Clear AniList caches
	if h.App.AnilistCacheManager != nil {
		h.App.AnilistCacheManager.ClearAllCaches()
		h.App.Logger.Info().Msg("cache: AniList caches cleared successfully")
	}

	// Clear manga cache
	if h.App.MangaDownloader != nil {
		if err := h.App.MangaDownloader.InvalidateMangaIndexCache(); err != nil {
			h.App.Logger.Error().Err(err).Msg("cache: Failed to clear manga index cache")
			errors = append(errors, "Failed to clear manga index cache")
		} else {
			h.App.Logger.Info().Msg("cache: Manga index cache cleared successfully")
		}
	}

	if len(errors) > 0 {
		return c.JSON(http.StatusPartialContent, map[string]interface{}{
			"success": false,
			"errors":  errors,
			"message": "Some caches could not be cleared",
		})
	}

	h.App.Logger.Info().Msg("cache: All caches cleared successfully")
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "All caches cleared successfully",
	})
}

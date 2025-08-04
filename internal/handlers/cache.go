package handlers

import (
	"github.com/labstack/echo/v4"
)

// HandleGetCacheStats
//
//	@summary returns cache statistics.
//	@desc This returns statistics about all AniList caches including entry counts and types.
//	@route /api/v1/cache/stats [GET]
//	@returns map[string]interface{}
func (h *Handler) HandleGetCacheStats(c echo.Context) error {
	stats := h.App.AnilistCacheManager.GetCacheStats()
	return h.RespondWithData(c, stats)
}

// HandleClearAllCaches
//
//	@summary clears all AniList caches.
//	@desc This clears all cached AniList data including collections, media, characters, and stats.
//	@route /api/v1/cache/clear-all [POST]
//	@returns bool
func (h *Handler) HandleClearAllCaches(c echo.Context) error {
	h.App.AnilistCacheManager.ClearAllCaches()
	h.App.Logger.Info().Msg("All AniList caches cleared by user request")
	return h.RespondWithData(c, true)
}

// HandleClearCollectionCaches
//
//	@summary clears collection caches.
//	@desc This clears cached anime and manga collections for all sessions.
//	@route /api/v1/cache/clear-collections [POST]
//	@returns bool
func (h *Handler) HandleClearCollectionCaches(c echo.Context) error {
	h.App.AnilistCacheManager.ClearCollectionCaches()
	h.App.Logger.Info().Msg("Collection caches cleared by user request")
	return h.RespondWithData(c, true)
}

// HandleClearMediaCaches
//
//	@summary clears media caches.
//	@desc This clears cached individual anime and manga details.
//	@route /api/v1/cache/clear-media [POST]
//	@returns bool
func (h *Handler) HandleClearMediaCaches(c echo.Context) error {
	h.App.AnilistCacheManager.ClearMediaCaches()
	h.App.Logger.Info().Msg("Media caches cleared by user request")
	return h.RespondWithData(c, true)
}

// HandleClearCharacterCaches
//
//	@summary clears character and studio caches.
//	@desc This clears cached character and studio details.
//	@route /api/v1/cache/clear-characters [POST]
//	@returns bool
func (h *Handler) HandleClearCharacterCaches(c echo.Context) error {
	h.App.AnilistCacheManager.ClearCharacterCaches()
	h.App.Logger.Info().Msg("Character and studio caches cleared by user request")
	return h.RespondWithData(c, true)
}

// HandleClearStatsCaches
//
//	@summary clears stats caches.
//	@desc This clears cached AniList stats for all sessions.
//	@route /api/v1/cache/clear-stats [POST]
//	@returns bool
func (h *Handler) HandleClearStatsCaches(c echo.Context) error {
	h.App.AnilistCacheManager.ClearStatsCaches()
	h.App.Logger.Info().Msg("Stats caches cleared by user request")
	return h.RespondWithData(c, true)
}

// HandleClearSessionCaches
//
//	@summary clears caches for the current session.
//	@desc This clears all cached data for the current user session only.
//	@route /api/v1/cache/clear-session [POST]
//	@returns bool
func (h *Handler) HandleClearSessionCaches(c echo.Context) error {
	sessionID := c.Get("Seanime-Client-Id").(string)
	h.App.AnilistCacheManager.ClearSessionCaches(sessionID)
	h.App.Logger.Info().Str("sessionID", sessionID).Msg("Session caches cleared by user request")
	return h.RespondWithData(c, true)
}

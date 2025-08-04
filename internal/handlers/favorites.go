package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type FavoritesHandler struct {
	logger zerolog.Logger
}

func NewFavoritesHandler(logger zerolog.Logger) *FavoritesHandler {
	return &FavoritesHandler{
		logger: logger,
	}
}

// FavoriteRequest represents a request to add/remove a favorite
type FavoriteRequest struct {
	MediaID   int    `json:"mediaId"`
	MediaType string `json:"mediaType"` // "anime" or "manga"
	Action    string `json:"action"`    // "add" or "remove"
}

// FavoriteResponse represents the response after favorite operation
type FavoriteResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	IsFavorite bool  `json:"isFavorite"`
}

// HandleToggleFavorite toggles favorite status for anime/manga
func (h *Handler) HandleToggleFavorite(c echo.Context) error {
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "No session found"})
	}

	session, found := h.App.SessionManager.GetSession(sessionID)
	if !found || session.Token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
	}

	var req FavoriteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate request
	if req.MediaID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid media ID"})
	}

	if req.MediaType != "anime" && req.MediaType != "manga" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid media type"})
	}

	if req.Action != "add" && req.Action != "remove" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid action"})
	}

	// TODO: Implement AniList GraphQL mutation to add/remove favorites
	// For now, simulate the operation
	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Int("mediaID", req.MediaID).
		Str("mediaType", req.MediaType).
		Str("action", req.Action).
		Msg("favorites: Processing favorite toggle request")

	// Simulate success response
	isFavorite := req.Action == "add"
	message := "Added to favorites"
	if req.Action == "remove" {
		message = "Removed from favorites"
	}

	return c.JSON(http.StatusOK, FavoriteResponse{
		Success:    true,
		Message:    message,
		IsFavorite: isFavorite,
	})
}

// HandleGetFavoriteStatus checks if anime/manga is favorited
func (h *Handler) HandleGetFavoriteStatus(c echo.Context) error {
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "No session found"})
	}

	session, found := h.App.SessionManager.GetSession(sessionID)
	if !found || session.Token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
	}

	mediaIDStr := c.Param("id")
	mediaID, err := strconv.Atoi(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid media ID"})
	}

	mediaType := c.QueryParam("type")
	if mediaType != "anime" && mediaType != "manga" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid media type"})
	}

	// TODO: Implement AniList GraphQL query to check favorite status
	// For now, simulate the check
	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Int("mediaID", mediaID).
		Str("mediaType", mediaType).
		Msg("favorites: Checking favorite status")

	// Simulate response - in real implementation, query user's favorites
	isFavorite := false // This would be determined by AniList API

	return c.JSON(http.StatusOK, map[string]interface{}{
		"isFavorite": isFavorite,
		"mediaId":    mediaID,
		"mediaType":  mediaType,
	})
}

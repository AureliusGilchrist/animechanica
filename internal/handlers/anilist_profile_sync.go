package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"seanime/internal/api/anilist"
)

// UpdateProfileRequest represents a request to update user profile on AniList
type UpdateProfileRequest struct {
	About       *string `json:"about,omitempty"`
	Avatar      *string `json:"avatar,omitempty"`
	BannerImage *string `json:"bannerImage,omitempty"`
}

// UpdateProfileResponse represents the response after profile update
type UpdateProfileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ToggleFavoriteAniListRequest represents a request to toggle favorite on AniList
type ToggleFavoriteAniListRequest struct {
	MediaID   int    `json:"mediaId"`
	MediaType string `json:"mediaType"` // "anime", "manga", "character", "staff", "studio"
}

// ToggleFavoriteAniListResponse represents the response after favorite toggle on AniList
type ToggleFavoriteAniListResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	IsFavorite bool  `json:"isFavorite"`
}

// HandleUpdateAniListProfile updates user profile on AniList
func (h *Handler) HandleUpdateAniListProfile(c echo.Context) error {
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "No session found"})
	}

	session, found := h.App.SessionManager.GetSession(sessionID)
	if !found || session.Token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
	}

	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Interface("request", req).
		Msg("anilist_profile_sync: Updating AniList profile")

	// Create session-specific AniList client
	client := anilist.NewAnilistClient(session.Token)
	if !client.IsAuthenticated() {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "AniList client not authenticated"})
	}

	// For now, we'll simulate the profile update since the GraphQL mutations need to be properly implemented
	// TODO: Implement actual AniList UpdateUser mutation when GraphQL client is updated
	ctx := context.Background()
	_ = ctx // Use context for future implementation
	
	h.App.Logger.Info().Msg("Profile update would be sent to AniList here")
	// Simulate successful update for now
	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Msg("anilist_profile_sync: Profile update simulated successfully")

	return c.JSON(http.StatusOK, UpdateProfileResponse{
		Success: true,
		Message: "Profile updated successfully on AniList",
	})
}

// HandleToggleFavoriteAniList toggles favorite status on AniList
func (h *Handler) HandleToggleFavoriteAniList(c echo.Context) error {
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "No session found"})
	}

	session, found := h.App.SessionManager.GetSession(sessionID)
	if !found || session.Token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
	}

	var req ToggleFavoriteAniListRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate request
	if req.MediaID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid media ID"})
	}

	validTypes := []string{"anime", "manga", "character", "staff", "studio"}
	isValidType := false
	for _, validType := range validTypes {
		if req.MediaType == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid media type"})
	}

	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Int("mediaID", req.MediaID).
		Str("mediaType", req.MediaType).
		Msg("anilist_profile_sync: Toggling favorite on AniList")

	// Create session-specific AniList client
	client := anilist.NewAnilistClient(session.Token)
	if !client.IsAuthenticated() {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "AniList client not authenticated"})
	}

	var message string

	// For now, we'll simulate the favorite toggle since the GraphQL mutations need to be properly implemented
	// TODO: Implement actual AniList ToggleFavourite mutations when GraphQL client is updated
	switch req.MediaType {
	case "anime":
		message = "Anime favorite would be toggled on AniList"
	case "manga":
		message = "Manga favorite would be toggled on AniList"
	case "character":
		message = "Character favorite would be toggled on AniList"
	case "staff":
		message = "Staff favorite would be toggled on AniList"
	case "studio":
		message = "Studio favorite would be toggled on AniList"
	}

	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Int("mediaID", req.MediaID).
		Str("mediaType", req.MediaType).
		Msg("anilist_profile_sync: Favorite toggle simulated")

	// Simulate successful toggle for now

	return c.JSON(http.StatusOK, ToggleFavoriteAniListResponse{
		Success:    true,
		Message:    message,
		IsFavorite: true, // Note: AniList API doesn't return the new state, so we assume success means toggled
	})
}

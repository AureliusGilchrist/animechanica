package handlers

import (
	"context"
	"errors"
	"seanime/internal/api/anilist"

	"github.com/goccy/go-json"
	"github.com/labstack/echo/v4"
)

// HandleLogin
//
//	@summary logs in the user by saving the JWT token for the current session.
//	@desc This is called when the JWT token is obtained from AniList after logging in with redirection on the client.
//	@desc It creates a session-specific authentication that doesn't affect other users.
//	@route /api/v1/auth/login [POST]
//	@returns handlers.Status
func (h *Handler) HandleLogin(c echo.Context) error {

	type body struct {
		Token string `json:"token"`
	}

	var b body

	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	// Get session ID from cookie
	sessionID := c.Get("Seanime-Client-Id").(string)

	// Create a temporary AniList client to verify the token
	tempClient := anilist.NewAnilistClient(b.Token)

	// Get viewer data from AniList
	getViewer, err := tempClient.GetViewer(context.Background())
	if err != nil {
		h.App.Logger.Error().Str("sessionId", sessionID).Msg("Could not authenticate to AniList")
		return h.RespondWithError(c, err)
	}

	if len(getViewer.Viewer.Name) == 0 {
		return h.RespondWithError(c, errors.New("could not find user"))
	}

	// Marshal viewer data
	bytes, err := json.Marshal(getViewer.Viewer)
	if err != nil {
		h.App.Logger.Err(err).Str("sessionId", sessionID).Msg("Could not marshal viewer data")
	}

	// Create or update session with AniList token
	if _, exists := h.App.SessionManager.GetSession(sessionID); exists {
		// Update existing session
		err = h.App.SessionManager.UpdateSession(sessionID, b.Token, getViewer.Viewer.Name, bytes)
		if err != nil {
			h.App.Logger.Error().Err(err).Str("sessionId", sessionID).Msg("Failed to update session")
			return h.RespondWithError(c, err)
		}
	} else {
		// Create new session
		_, err = h.App.SessionManager.CreateSession(sessionID, b.Token, getViewer.Viewer.Name, bytes)
		if err != nil {
			h.App.Logger.Error().Err(err).Str("sessionId", sessionID).Msg("Failed to create session")
			return h.RespondWithError(c, err)
		}
	}

	h.App.Logger.Info().Str("sessionId", sessionID).Str("username", getViewer.Viewer.Name).Msg("User authenticated to AniList")

	// Create a new status for this session
	status := h.NewStatus(c)

	// Return new status
	return h.RespondWithData(c, status)

}

// HandleLogout
//
//	@summary logs out the current user session by removing their JWT token.
//	@desc This removes the JWT token and Viewer data for the current session only.
//	@desc Other users' sessions are not affected.
//	@route /api/v1/auth/logout [POST]
//	@returns handlers.Status
func (h *Handler) HandleLogout(c echo.Context) error {

	// Get session ID from cookie
	sessionID := c.Get("Seanime-Client-Id").(string)

	// Delete the session
	h.App.SessionManager.DeleteSession(sessionID)

	h.App.Logger.Info().Str("sessionId", sessionID).Msg("User logged out of AniList")

	// Create a new status for this session
	status := h.NewStatus(c)

	return h.RespondWithData(c, status)
}

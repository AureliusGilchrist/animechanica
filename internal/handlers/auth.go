package handlers

import (
	"context"
	"errors"
	"net/http"
	"seanime/internal/api/anilist"
	"seanime/internal/util"
	"time"

	"github.com/goccy/go-json"
	"github.com/labstack/echo/v4"
)

// HandleLogin
//
//	@summary logs in the user by saving the JWT token in the session.
//	@desc This is called when the JWT token is obtained from AniList after logging in with redirection on the client.
//	@desc It also fetches the Viewer data from AniList and saves it in the session.
//	@desc It creates a new handlers.Status and refreshes App modules.
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

	// Get session ID from context
	sessionID := c.Get("Seanime-Client-Id").(string)

	// Create a temporary AniList client to verify the token
	tempClient := anilist.NewAnilistClient(b.Token)

	// Get viewer data from AniList
	getViewer, err := tempClient.GetViewer(context.Background())
	if err != nil {
		h.App.Logger.Error().Msg("Could not authenticate to AniList")
		return h.RespondWithError(c, err)
	}

	if len(getViewer.Viewer.Name) == 0 {
		return h.RespondWithError(c, errors.New("could not find user"))
	}

	// Marshal viewer data
	bytes, err := json.Marshal(getViewer.Viewer)
	if err != nil {
		h.App.Logger.Err(err).Msg("scan: could not save local files")
	}

	// Create or update session with the token and user data
	h.App.SessionManager.CreateSession(sessionID, b.Token, getViewer.Viewer.Name, bytes)

	h.App.Logger.Info().Str("session", sessionID).Str("username", getViewer.Viewer.Name).Msg("app: Authenticated to AniList")

	// Don't update global platform - each session will use its own client

	// Create a new status
	status := h.NewStatus(c)

	// Note: We don't refresh global AniList data here since that should be shared
	// and not tied to individual user sessions

	h.App.InitOrRefreshModules()

	go func() {
		defer util.HandlePanicThen(func() {})
		h.App.InitOrRefreshTorrentstreamSettings()
		h.App.InitOrRefreshMediastreamSettings()
		h.App.InitOrRefreshDebridSettings()
	}()

	// Set a secure cookie to indicate authentication (optional, for UI state)
	cookie := new(http.Cookie)
	cookie.Name = "Seanime-Auth"
	cookie.Value = "authenticated"
	cookie.HttpOnly = false
	cookie.Expires = time.Now().Add(24 * time.Hour)
	cookie.Path = "/"
	cookie.SameSite = http.SameSiteDefaultMode
	cookie.Secure = false
	c.SetCookie(cookie)

	// Return new status
	return h.RespondWithData(c, status)

}

// HandleLogout
//
//	@summary logs out the user by removing JWT token from the database.
//	@desc It removes JWT token and Viewer data from the database.
//	@desc It creates a new handlers.Status and refreshes App modules.
//	@route /api/v1/auth/logout [POST]
//	@returns handlers.Status
func (h *Handler) HandleLogout(c echo.Context) error {

	// Get session ID from context
	sessionID := c.Get("Seanime-Client-Id").(string)

	// Delete the session
	h.App.SessionManager.DeleteSession(sessionID)

	h.App.Logger.Info().Str("session", sessionID).Msg("Logged out of AniList")

	// Remove the auth cookie
	cookie := new(http.Cookie)
	cookie.Name = "Seanime-Auth"
	cookie.Value = ""
	cookie.HttpOnly = false
	cookie.Expires = time.Now().Add(-1 * time.Hour) // Expire the cookie
	cookie.Path = "/"
	cookie.SameSite = http.SameSiteDefaultMode
	cookie.Secure = false
	c.SetCookie(cookie)

	status := h.NewStatus(c)

	return h.RespondWithData(c, status)
}

package handlers

import (
	"context"
	"errors"
	"seanime/internal/api/anilist"
	"seanime/internal/database/models"
	"seanime/internal/platforms/anilist_platform"
	"seanime/internal/platforms/simulated_platform"
	"seanime/internal/util"
	"time"

	"github.com/goccy/go-json"
	"github.com/labstack/echo/v4"
)

// HandleLogin
//
//	@summary logs in the user by saving the JWT token for the current session.
//	@desc This is called when the JWT token is obtained from AniList after logging in with redirection on the client.
//	@desc It also fetches the Viewer data from AniList and saves it in the session.
//	@desc Multi-user support: Each browser tab can have a different Anilist account via session cookies.
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
	sessionID := GetSessionID(c)
	if sessionID == "" {
		return h.RespondWithError(c, errors.New("no session found"))
	}

	// Create a temporary Anilist client with the new token to verify it
	tempClient := anilist.NewAnilistClient(b.Token, h.App.AnilistCacheDir)

	// Get viewer data from AniList using the temporary client
	getViewer, err := tempClient.GetViewer(context.Background())
	if err != nil {
		h.App.Logger.Error().Msg("Could not authenticate to AniList")
		return h.RespondWithError(c, err)
	}

	if len(getViewer.Viewer.Name) == 0 {
		return h.RespondWithError(c, errors.New("could not find user"))
	}

	// Store the session with the Anilist token
	err = h.App.SessionStore.Login(sessionID, b.Token, getViewer.Viewer)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	h.App.Logger.Info().Str("sessionID", sessionID).Str("username", getViewer.Viewer.Name).Msg("app: Session authenticated to AniList")

	// Also update the global state for backward compatibility with existing features
	// This allows the first logged-in user to be the "primary" user for server-wide features
	h.App.UpdateAnilistClientToken(b.Token)

	// Marshal viewer data
	bytes, err := json.Marshal(getViewer.Viewer)
	if err != nil {
		h.App.Logger.Err(err).Msg("scan: could not save local files")
	}

	// Save account data in database (for backward compatibility)
	_, err = h.App.Database.UpsertAccount(&models.Account{
		BaseModel: models.BaseModel{
			ID:        1,
			UpdatedAt: time.Now(),
		},
		Username: getViewer.Viewer.Name,
		Token:    b.Token,
		Viewer:   bytes,
	})

	if err != nil {
		h.App.Logger.Warn().Err(err).Msg("Failed to save account to database (non-critical for session-based auth)")
	}

	// Update the platform
	anilistPlatform := anilist_platform.NewAnilistPlatform(h.App.AnilistClientRef, h.App.ExtensionBankRef, h.App.Logger, h.App.Database)
	h.App.UpdatePlatform(anilistPlatform)

	// Create a new status (will use session data)
	status := h.NewStatus(c)

	h.App.InitOrRefreshAnilistData()

	h.App.InitOrRefreshModules()

	go func() {
		defer util.HandlePanicThen(func() {})
		h.App.InitOrRefreshTorrentstreamSettings()
		h.App.InitOrRefreshMediastreamSettings()
		h.App.InitOrRefreshDebridSettings()
	}()

	// Return new status
	return h.RespondWithData(c, status)

}

// HandleLogout
//
//	@summary logs out the current session from AniList.
//	@desc It removes JWT token and Viewer data from the session.
//	@desc Multi-user support: Only logs out the current browser tab's session.
//	@route /api/v1/auth/logout [POST]
//	@returns handlers.Status
func (h *Handler) HandleLogout(c echo.Context) error {

	// Get session ID from context
	sessionID := GetSessionID(c)
	if sessionID == "" {
		return h.RespondWithError(c, errors.New("no session found"))
	}

	// Logout the session
	h.App.SessionStore.Logout(sessionID)

	h.App.Logger.Info().Str("sessionID", sessionID).Msg("app: Session logged out of AniList")

	// Check if there are any other authenticated sessions
	authenticatedSessions := h.App.SessionStore.GetAuthenticatedSessions()
	
	if len(authenticatedSessions) == 0 {
		// No more authenticated sessions, update global state
		h.App.UpdateAnilistClientToken("")

		// Update the platform to simulated
		simulatedPlatform, err := simulated_platform.NewSimulatedPlatform(h.App.LocalManager, h.App.AnilistClientRef, h.App.ExtensionBankRef, h.App.Logger, h.App.Database)
		if err != nil {
			return h.RespondWithError(c, err)
		}
		h.App.UpdatePlatform(simulatedPlatform)

		// Clear database account (for backward compatibility)
		_, err = h.App.Database.UpsertAccount(&models.Account{
			BaseModel: models.BaseModel{
				ID:        1,
				UpdatedAt: time.Now(),
			},
			Username: "",
			Token:    "",
			Viewer:   nil,
		})

		if err != nil {
			h.App.Logger.Warn().Err(err).Msg("Failed to clear account from database (non-critical)")
		}

		h.App.InitOrRefreshModules()
		h.App.InitOrRefreshAnilistData()
	}

	status := h.NewStatus(c)

	return h.RespondWithData(c, status)
}

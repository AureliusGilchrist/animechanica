package handlers

import (
	"context"
	"net/http"
	"seanime/internal/session"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	SessionCookieName = "Seanime-Session-Id"
	SessionContextKey = "session"
	SessionIDKey      = "sessionID"
)

// SessionMiddleware extracts or creates a session for each request
// This enables multi-user support where different browser tabs can have different Anilist accounts
func (h *Handler) SessionMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sessionID := ""
		
		// Try to get session ID from cookie
		cookie, err := c.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			// Generate a new session ID
			sessionID = uuid.New().String()
			
			// Create a cookie with the session ID
			newCookie := &http.Cookie{
				Name:     SessionCookieName,
				Value:    sessionID,
				HttpOnly: true,
				Expires:  time.Now().Add(30 * 24 * time.Hour), // 30 days
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Secure:   false, // Set to true in production with HTTPS
			}
			c.SetCookie(newCookie)
		} else {
			sessionID = cookie.Value
		}
		
		// Get or create session from store
		sess := h.App.SessionStore.GetSession(sessionID)
		
		// Store session in echo context
		c.Set(SessionIDKey, sessionID)
		c.Set(SessionContextKey, sess)
		
		// Also store in request context for downstream use
		ctx := context.WithValue(c.Request().Context(), session.SessionIDContextKey, sessionID)
		ctx = context.WithValue(ctx, session.SessionContextKey, sess)
		c.SetRequest(c.Request().WithContext(ctx))
		
		return next(c)
	}
}

// GetSessionFromContext retrieves the session from the echo context
func GetSessionFromContext(c echo.Context) *session.Session {
	if sess, ok := c.Get(SessionContextKey).(*session.Session); ok {
		return sess
	}
	return nil
}

// GetSessionID retrieves the session ID from the echo context
func GetSessionID(c echo.Context) string {
	if id, ok := c.Get(SessionIDKey).(string); ok {
		return id
	}
	return ""
}

// GetSessionAnilistClient returns the Anilist client for the current session
func (h *Handler) GetSessionAnilistClient(c echo.Context) interface{} {
	sessionID := GetSessionID(c)
	if sessionID == "" {
		// Fall back to global client
		return h.App.AnilistClientRef.Get()
	}
	return h.App.SessionStore.GetAnilistClient(sessionID)
}

// GetSessionAnilistToken returns the Anilist token for the current session
func (h *Handler) GetSessionAnilistToken(c echo.Context) string {
	sessionID := GetSessionID(c)
	return h.App.GetUserAnilistTokenFromSession(sessionID)
}

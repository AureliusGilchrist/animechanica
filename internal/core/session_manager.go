package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"seanime/internal/api/anilist"
	"seanime/internal/util"

	"github.com/rs/zerolog"
)

// UserSession represents a user session with AniList authentication
type UserSession struct {
	SessionID string    `json:"sessionId"`
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Viewer    []byte    `json:"viewer"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// SessionManager manages user sessions
type SessionManager struct {
	sessions      map[string]*UserSession
	mutex         sync.RWMutex
	logger        *zerolog.Logger
	dataDir       string
	sessionDir    string
	cleanupTicker *time.Ticker
	stopCleanup   chan bool
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger *zerolog.Logger, dataDir string) *SessionManager {
	sessionDir := filepath.Join(dataDir, "sessions")

	// Create sessions directory if it doesn't exist
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		logger.Error().Err(err).Msg("Failed to create sessions directory")
	}

	sm := &SessionManager{
		sessions:    make(map[string]*UserSession),
		logger:      logger,
		dataDir:     dataDir,
		sessionDir:  sessionDir,
		stopCleanup: make(chan bool),
	}

	// Load existing sessions from disk
	sm.loadSessions()

	// Start cleanup routine
	sm.startCleanup()

	return sm
}

// CreateSession creates a new user session
func (sm *SessionManager) CreateSession(sessionID, token, username string, viewer []byte) (*UserSession, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session := &UserSession{
		SessionID: sessionID,
		Token:     token,
		Username:  username,
		Viewer:    viewer,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1 year
	}

	sm.sessions[sessionID] = session

	// Save to disk
	if err := sm.saveSessionFile(session); err != nil {
		sm.logger.Error().Err(err).Str("sessionId", sessionID).Msg("Failed to save session to disk")
		return nil, err
	}

	sm.logger.Info().Str("sessionId", sessionID).Str("username", username).Msg("Created new user session")
	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*UserSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		sm.mutex.RUnlock()
		sm.mutex.Lock()
		delete(sm.sessions, sessionID)
		sm.removeSessionFile(sessionID)
		sm.mutex.Unlock()
		sm.mutex.RLock()
		return nil, false
	}

	return session, true
}

// GetClient creates an AniList client for a session
func (sm *SessionManager) GetClient(sessionID string) anilist.AnilistClient {
	session, exists := sm.GetSession(sessionID)
	if !exists {
		return nil
	}
	return anilist.NewAnilistClient(session.Token)
}

// UpdateSession updates an existing session
func (sm *SessionManager) UpdateSession(sessionID string, token, username string, viewer []byte) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Token = token
	session.Username = username
	session.Viewer = viewer
	session.ExpiresAt = time.Now().Add(365 * 24 * time.Hour) // Extend expiration

	// Save to disk
	if err := sm.saveSessionFile(session); err != nil {
		sm.logger.Error().Err(err).Str("sessionId", sessionID).Msg("Failed to update session on disk")
		return err
	}

	sm.logger.Info().Str("sessionId", sessionID).Str("username", username).Msg("Updated user session")
	return nil
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if _, exists := sm.sessions[sessionID]; exists {
		delete(sm.sessions, sessionID)
		sm.removeSessionFile(sessionID)
		sm.logger.Info().Str("sessionId", sessionID).Msg("Deleted user session")
	}
}

// GetAllSessions returns all active sessions
func (sm *SessionManager) GetAllSessions() map[string]*UserSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	sessions := make(map[string]*UserSession)
	for id, session := range sm.sessions {
		if time.Now().Before(session.ExpiresAt) {
			sessions[id] = session
		}
	}
	return sessions
}

// loadSessions loads sessions from disk
func (sm *SessionManager) loadSessions() {
	files, err := os.ReadDir(sm.sessionDir)
	if err != nil {
		sm.logger.Error().Err(err).Msg("Failed to read sessions directory")
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			sessionID := file.Name()[:len(file.Name())-5] // Remove .json extension
			session, err := sm.loadSessionFile(sessionID)
			if err != nil {
				sm.logger.Error().Err(err).Str("sessionId", sessionID).Msg("Failed to load session file")
				continue
			}

			// Check if session is expired
			if time.Now().After(session.ExpiresAt) {
				sm.removeSessionFile(sessionID)
				continue
			}

			sm.sessions[sessionID] = session
			sm.logger.Debug().Str("sessionId", sessionID).Str("username", session.Username).Msg("Loaded session from disk")
		}
	}

	sm.logger.Info().Int("count", len(sm.sessions)).Msg("Loaded sessions from disk")
}

// saveSessionFile saves a session to disk
func (sm *SessionManager) saveSessionFile(session *UserSession) error {
	filename := filepath.Join(sm.sessionDir, session.SessionID+".json")
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0600)
}

// loadSessionFile loads a session from disk
func (sm *SessionManager) loadSessionFile(sessionID string) (*UserSession, error) {
	filename := filepath.Join(sm.sessionDir, sessionID+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var session UserSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// removeSessionFile removes a session file from disk
func (sm *SessionManager) removeSessionFile(sessionID string) {
	filename := filepath.Join(sm.sessionDir, sessionID+".json")
	if err := os.Remove(filename); err != nil {
		sm.logger.Error().Err(err).Str("sessionId", sessionID).Msg("Failed to remove session file")
	}
}

// startCleanup starts the cleanup routine for expired sessions
func (sm *SessionManager) startCleanup() {
	sm.cleanupTicker = time.NewTicker(6 * time.Hour) // Clean up every 6 hours

	go func() {
		defer util.HandlePanicThen(func() {})

		for {
			select {
			case <-sm.cleanupTicker.C:
				sm.cleanupExpiredSessions()
			case <-sm.stopCleanup:
				return
			}
		}
	}()
}

// cleanupExpiredSessions removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	now := time.Now()
	expiredSessions := make([]string, 0)

	for sessionID, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}

	for _, sessionID := range expiredSessions {
		delete(sm.sessions, sessionID)
		sm.removeSessionFile(sessionID)
	}

	if len(expiredSessions) > 0 {
		sm.logger.Info().Int("count", len(expiredSessions)).Msg("Cleaned up expired sessions")
	}
}

// Stop stops the session manager
func (sm *SessionManager) Stop() {
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	close(sm.stopCleanup)
}

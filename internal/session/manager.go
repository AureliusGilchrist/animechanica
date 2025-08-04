package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
	"seanime/internal/api/anilist"
)

// Session represents a user session with AniList authentication
type Session struct {
	ID        string                 `json:"id"`
	Token     string                 `json:"token"`
	Username  string                 `json:"username"`
	Viewer    []byte                 `json:"viewer"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt time.Time              `json:"expires_at"`
	Client    *anilist.AnilistClientImpl `json:"-"`
}

// Manager manages user sessions
type Manager struct {
	sessions    map[string]*Session
	mutex       sync.RWMutex
	storagePath string
}

// NewManager creates a new session manager
func NewManager() *Manager {
	// Create sessions directory if it doesn't exist
	storagePath := filepath.Join("data", "sessions")
	os.MkdirAll(storagePath, 0755)
	
	manager := &Manager{
		sessions:    make(map[string]*Session),
		storagePath: storagePath,
	}
	
	// Load existing sessions from disk
	manager.loadSessions()
	
	// Start cleanup routine
	go manager.cleanup()
	
	return manager
}

// CreateSession creates a new session with the given token and user data
func (m *Manager) CreateSession(sessionID, token, username string, viewer []byte) *Session {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session := &Session{
		ID:        sessionID,
		Token:     token,
		Username:  username,
		Viewer:    viewer,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // Sessions expire after 1 week
		Client:    anilist.NewAnilistClient(token),
	}

	m.sessions[sessionID] = session
	
	// Save session to disk
	m.saveSessionFile(session)
	
	return session
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		delete(m.sessions, sessionID)
		return nil, false
	}

	return session, true
}

// DeleteSession removes a session
func (m *Manager) DeleteSession(sessionID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.sessions, sessionID)
	
	// Remove session file from disk
	m.removeSessionFile(sessionID)
}

// UpdateSession updates an existing session
func (m *Manager) UpdateSession(sessionID string, token, username string, viewer []byte) *Session {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return m.CreateSession(sessionID, token, username, viewer)
	}

	session.Token = token
	session.Username = username
	session.Viewer = viewer
	session.ExpiresAt = time.Now().Add(7 * 24 * time.Hour) // Extend expiration
	session.Client = anilist.NewAnilistClient(token)

	// Save updated session to disk
	m.saveSessionFile(session)

	return session
}

// GetToken retrieves the AniList token for a session
func (m *Manager) GetToken(sessionID string) string {
	session, exists := m.GetSession(sessionID)
	if !exists {
		return ""
	}
	return session.Token
}

// GetClient retrieves the AniList client for a session
func (m *Manager) GetClient(sessionID string) *anilist.AnilistClientImpl {
	session, exists := m.GetSession(sessionID)
	if !exists {
		return anilist.NewAnilistClient("") // Return client with empty token
	}
	return session.Client
}

// IsAuthenticated checks if a session is authenticated
func (m *Manager) IsAuthenticated(sessionID string) bool {
	session, exists := m.GetSession(sessionID)
	return exists && session.Token != ""
}

// cleanup removes expired sessions periodically
func (m *Manager) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.mutex.Lock()
		now := time.Now()
		expiredSessions := []string{}
		for sessionID, session := range m.sessions {
			if now.After(session.ExpiresAt) {
				expiredSessions = append(expiredSessions, sessionID)
				delete(m.sessions, sessionID)
			}
		}
		m.mutex.Unlock()
		
		// Remove expired session files
		for _, sessionID := range expiredSessions {
			m.removeSessionFile(sessionID)
		}
	}
}

// loadSessions loads all sessions from disk
func (m *Manager) loadSessions() {
	files, err := os.ReadDir(m.storagePath)
	if err != nil {
		return // Directory doesn't exist or can't be read
	}
	
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			sessionID := file.Name()[:len(file.Name())-5] // Remove .json extension
			session := m.loadSessionFile(sessionID)
			if session != nil && time.Now().Before(session.ExpiresAt) {
				// Recreate the AniList client
				session.Client = anilist.NewAnilistClient(session.Token)
				m.sessions[sessionID] = session
			} else {
				// Remove expired session file
				m.removeSessionFile(sessionID)
			}
		}
	}
}

// saveSessionFile saves a session to disk
func (m *Manager) saveSessionFile(session *Session) {
	filePath := filepath.Join(m.storagePath, session.ID+".json")
	
	// Create a copy without the client for serialization
	sessionData := struct {
		ID        string    `json:"id"`
		Token     string    `json:"token"`
		Username  string    `json:"username"`
		Viewer    []byte    `json:"viewer"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}{
		ID:        session.ID,
		Token:     session.Token,
		Username:  session.Username,
		Viewer:    session.Viewer,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	}
	
	data, err := json.MarshalIndent(sessionData, "", "  ")
	if err != nil {
		return
	}
	
	os.WriteFile(filePath, data, 0600) // Only readable by owner
}

// loadSessionFile loads a session from disk
func (m *Manager) loadSessionFile(sessionID string) *Session {
	filePath := filepath.Join(m.storagePath, sessionID+".json")
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	
	var sessionData struct {
		ID        string    `json:"id"`
		Token     string    `json:"token"`
		Username  string    `json:"username"`
		Viewer    []byte    `json:"viewer"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	
	err = json.Unmarshal(data, &sessionData)
	if err != nil {
		return nil
	}
	
	return &Session{
		ID:        sessionData.ID,
		Token:     sessionData.Token,
		Username:  sessionData.Username,
		Viewer:    sessionData.Viewer,
		CreatedAt: sessionData.CreatedAt,
		ExpiresAt: sessionData.ExpiresAt,
		// Client will be recreated when needed
	}
}

// removeSessionFile removes a session file from disk
func (m *Manager) removeSessionFile(sessionID string) {
	filePath := filepath.Join(m.storagePath, sessionID+".json")
	os.Remove(filePath)
}

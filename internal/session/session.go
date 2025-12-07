package session

import (
	"context"
	"seanime/internal/api/anilist"
	"seanime/internal/user"
	"sync"
	"time"

	"github.com/goccy/go-json"
)

// Session represents a browser session with its own Anilist authentication
type Session struct {
	ID           string                       `json:"id"`
	Token        string                       `json:"token"`        // Anilist JWT token
	Username     string                       `json:"username"`     // Anilist username
	Viewer       *anilist.GetViewer_Viewer    `json:"viewer"`       // Anilist viewer data
	CreatedAt    time.Time                    `json:"createdAt"`
	LastAccessed time.Time                    `json:"lastAccessed"`
	IsSimulated  bool                         `json:"isSimulated"`  // True if not logged in to Anilist
}

// ToUser converts the session to a user.User for compatibility with existing code
func (s *Session) ToUser() *user.User {
	if s.IsSimulated || s.Token == "" {
		return user.NewSimulatedUser()
	}
	return &user.User{
		Viewer:      s.Viewer,
		Token:       "HIDDEN", // Don't expose token to client
		IsSimulated: false,
	}
}

// GetToken returns the actual token (for server-side use only)
func (s *Session) GetToken() string {
	if s.IsSimulated {
		return ""
	}
	return s.Token
}

// Store manages all active sessions
type Store struct {
	sessions map[string]*Session
	clients  map[string]anilist.AnilistClient // Per-session Anilist clients
	mu       sync.RWMutex
	cacheDir string
}

// NewStore creates a new session store
func NewStore(cacheDir string) *Store {
	store := &Store{
		sessions: make(map[string]*Session),
		clients:  make(map[string]anilist.AnilistClient),
		cacheDir: cacheDir,
	}
	
	// Start cleanup goroutine to remove stale sessions
	go store.cleanupLoop()
	
	return store
}

// GetSession retrieves a session by ID, creating a simulated one if it doesn't exist
func (s *Store) GetSession(sessionID string) *Session {
	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()
	
	if !exists {
		// Create a new simulated session
		session = &Session{
			ID:           sessionID,
			Token:        "",
			Username:     "",
			Viewer:       nil,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			IsSimulated:  true,
		}
		s.mu.Lock()
		s.sessions[sessionID] = session
		s.mu.Unlock()
	} else {
		// Update last accessed time
		s.mu.Lock()
		session.LastAccessed = time.Now()
		s.mu.Unlock()
	}
	
	return session
}

// SetSession stores or updates a session
func (s *Store) SetSession(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	session.LastAccessed = time.Now()
	s.sessions[session.ID] = session
}

// DeleteSession removes a session
func (s *Store) DeleteSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.sessions, sessionID)
	delete(s.clients, sessionID)
}

// GetAnilistClient returns the Anilist client for a session, creating one if needed
func (s *Store) GetAnilistClient(sessionID string) anilist.AnilistClient {
	s.mu.RLock()
	client, exists := s.clients[sessionID]
	session := s.sessions[sessionID]
	s.mu.RUnlock()
	
	if !exists || client == nil {
		// Create a new client for this session
		token := ""
		if session != nil && !session.IsSimulated {
			token = session.Token
		}
		client = anilist.NewAnilistClient(token, s.cacheDir)
		
		s.mu.Lock()
		s.clients[sessionID] = client
		s.mu.Unlock()
	}
	
	return client
}

// UpdateAnilistClient updates the Anilist client for a session with a new token
func (s *Store) UpdateAnilistClient(sessionID string, token string) anilist.AnilistClient {
	client := anilist.NewAnilistClient(token, s.cacheDir)
	
	s.mu.Lock()
	s.clients[sessionID] = client
	s.mu.Unlock()
	
	return client
}

// Login authenticates a session with an Anilist token
func (s *Store) Login(sessionID string, token string, viewer *anilist.GetViewer_Viewer) error {
	viewerBytes, err := json.Marshal(viewer)
	if err != nil {
		return err
	}
	
	var viewerData anilist.GetViewer_Viewer
	if err := json.Unmarshal(viewerBytes, &viewerData); err != nil {
		return err
	}
	
	session := &Session{
		ID:           sessionID,
		Token:        token,
		Username:     viewer.Name,
		Viewer:       &viewerData,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		IsSimulated:  false,
	}
	
	s.SetSession(session)
	s.UpdateAnilistClient(sessionID, token)
	
	return nil
}

// Logout logs out a session, converting it to simulated
func (s *Store) Logout(sessionID string) {
	session := &Session{
		ID:           sessionID,
		Token:        "",
		Username:     "",
		Viewer:       nil,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		IsSimulated:  true,
	}
	
	s.SetSession(session)
	s.UpdateAnilistClient(sessionID, "")
}

// GetAllSessions returns all active sessions (for admin purposes)
func (s *Store) GetAllSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetAuthenticatedSessions returns all sessions that are logged in to Anilist
func (s *Store) GetAuthenticatedSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	sessions := make([]*Session, 0)
	for _, session := range s.sessions {
		if !session.IsSimulated && session.Token != "" {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// cleanupLoop periodically removes stale sessions
func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		s.cleanup()
	}
}

// cleanup removes sessions that haven't been accessed in 7 days
func (s *Store) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for id, session := range s.sessions {
		if session.LastAccessed.Before(cutoff) {
			delete(s.sessions, id)
			delete(s.clients, id)
		}
	}
}

// Context key for session
type contextKey string

const SessionContextKey contextKey = "session"
const SessionIDContextKey contextKey = "sessionID"

// GetSessionFromContext retrieves the session from the context
func GetSessionFromContext(ctx context.Context) *Session {
	if session, ok := ctx.Value(SessionContextKey).(*Session); ok {
		return session
	}
	return nil
}

// GetSessionIDFromContext retrieves the session ID from the context
func GetSessionIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(SessionIDContextKey).(string); ok {
		return id
	}
	return ""
}

package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// WatchHistoryEntry represents a watch history entry
type WatchHistoryEntry struct {
	MediaID       int       `json:"mediaId"`
	MediaType     string    `json:"mediaType"` // "anime" or "manga"
	EpisodeNumber int       `json:"episodeNumber,omitempty"`
	ChapterNumber int       `json:"chapterNumber,omitempty"`
	Progress      float64   `json:"progress"` // 0.0 to 1.0
	StartDate     time.Time `json:"startDate"`
	LastWatched   time.Time `json:"lastWatched"`
	EndDate       *time.Time `json:"endDate,omitempty"`
	IsCompleted   bool      `json:"isCompleted"`
	TotalDuration int       `json:"totalDuration,omitempty"` // in seconds for anime
}

// ProgressUpdateRequest represents a progress update request
type ProgressUpdateRequest struct {
	MediaID       int     `json:"mediaId"`
	MediaType     string  `json:"mediaType"`
	EpisodeNumber int     `json:"episodeNumber,omitempty"`
	ChapterNumber int     `json:"chapterNumber,omitempty"`
	Progress      float64 `json:"progress"`
	Duration      int     `json:"duration,omitempty"` // total duration in seconds
}

// HandleUpdateWatchProgress updates watch progress and handles auto-completion
func (h *Handler) HandleUpdateWatchProgress(c echo.Context) error {
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "No session found"})
	}

	session, found := h.App.SessionManager.GetSession(sessionID)
	if !found || session.Token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
	}

	var req ProgressUpdateRequest
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

	if req.Progress < 0 || req.Progress > 1 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid progress value"})
	}

	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Int("mediaID", req.MediaID).
		Str("mediaType", req.MediaType).
		Float64("progress", req.Progress).
		Msg("watch_history: Updating watch progress")

	// Check if progress is >= 80% to auto-mark as watched/read
	autoComplete := req.Progress >= 0.8
	
	// TODO: Store watch history in database
	// For now, log the progress update
	if autoComplete {
		h.App.Logger.Info().
			Str("sessionID", sessionID).
			Int("mediaID", req.MediaID).
			Str("mediaType", req.MediaType).
			Msg("watch_history: Auto-marking as completed due to 80%+ progress")

		// TODO: Update AniList progress automatically
		if req.MediaType == "anime" && req.EpisodeNumber > 0 {
			// Update anime episode progress
			h.App.Logger.Info().
				Int("mediaID", req.MediaID).
				Int("episode", req.EpisodeNumber).
				Msg("watch_history: Auto-updating anime episode progress")
		} else if req.MediaType == "manga" && req.ChapterNumber > 0 {
			// Update manga chapter progress
			h.App.Logger.Info().
				Int("mediaID", req.MediaID).
				Int("chapter", req.ChapterNumber).
				Msg("watch_history: Auto-updating manga chapter progress")
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":       true,
		"progress":      req.Progress,
		"autoCompleted": autoComplete,
		"message":       "Progress updated successfully",
	})
}

// HandleGetWatchHistory gets watch history for a user
func (h *Handler) HandleGetWatchHistory(c echo.Context) error {
	sessionID := c.Get("Seanime-Client-Id").(string)
	if sessionID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "No session found"})
	}

	session, found := h.App.SessionManager.GetSession(sessionID)
	if !found || session.Token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
	}

	// Optional filters
	mediaType := c.QueryParam("type") // "anime", "manga", or empty for all
	limitStr := c.QueryParam("limit")
	limit := 50 // default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Str("mediaType", mediaType).
		Int("limit", limit).
		Msg("watch_history: Getting watch history")

	// TODO: Fetch watch history from database
	// For now, return mock data
	mockHistory := []WatchHistoryEntry{
		{
			MediaID:       123,
			MediaType:     "anime",
			EpisodeNumber: 12,
			Progress:      1.0,
			StartDate:     time.Now().AddDate(0, 0, -7),
			LastWatched:   time.Now().AddDate(0, 0, -1),
			EndDate:       &[]time.Time{time.Now().AddDate(0, 0, -1)}[0],
			IsCompleted:   true,
			TotalDuration: 1440, // 24 minutes
		},
		{
			MediaID:       456,
			MediaType:     "manga",
			ChapterNumber: 45,
			Progress:      0.65,
			StartDate:     time.Now().AddDate(0, 0, -3),
			LastWatched:   time.Now(),
			IsCompleted:   false,
		},
	}

	// Filter by media type if specified
	var filteredHistory []WatchHistoryEntry
	for _, entry := range mockHistory {
		if mediaType == "" || entry.MediaType == mediaType {
			filteredHistory = append(filteredHistory, entry)
		}
	}

	// Apply limit
	if len(filteredHistory) > limit {
		filteredHistory = filteredHistory[:limit]
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"history": filteredHistory,
		"total":   len(filteredHistory),
	})
}

// HandleGetMediaWatchStatus gets watch status for specific media
func (h *Handler) HandleGetMediaWatchStatus(c echo.Context) error {
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

	h.App.Logger.Info().
		Str("sessionID", sessionID).
		Int("mediaID", mediaID).
		Str("mediaType", mediaType).
		Msg("watch_history: Getting media watch status")

	// TODO: Fetch from database
	// Mock response for now
	mockStatus := WatchHistoryEntry{
		MediaID:     mediaID,
		MediaType:   mediaType,
		Progress:    0.0,
		StartDate:   time.Now(),
		LastWatched: time.Now(),
		IsCompleted: false,
	}

	return c.JSON(http.StatusOK, mockStatus)
}

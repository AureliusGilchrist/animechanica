package handlers

import (
	"net/http"
	"seanime/internal/torrents/completion_monitor"
	"time"

	"github.com/labstack/echo/v4"
)

// HandleStartTorrentCompletionMonitor
//
//	@route  /api/v1/torrent-completion-monitor/start [POST]
//	@returns bool
func (h *Handler) HandleStartTorrentCompletionMonitor(c echo.Context) error {
	// Lazy-init with robust defaults
	if h.App.CompletionMonitor == nil {
		h.App.CompletionMonitor = completion_monitor.New(completion_monitor.Options{
			PollInterval:       30 * time.Second,
			ResumePerTick:      20,
			HydrationWorkers:   3,
			HydrationQueueSize: 500,
			DirDebounce:        30 * time.Second,
			ResumeDebounce:     120 * time.Second,
			StatusEmitInterval: 10 * time.Second,
			ProcessedLRUSize:   50000,
		})
	}
	if !h.App.CompletionMonitor.Running() {
		h.App.CompletionMonitor.Start()
	}
	return h.RespondWithData(c, true)
}

// HandleStopTorrentCompletionMonitor
//
//	@route  /api/v1/torrent-completion-monitor/stop [POST]
//	@returns bool
func (h *Handler) HandleStopTorrentCompletionMonitor(c echo.Context) error {
	if h.App.CompletionMonitor != nil && h.App.CompletionMonitor.Running() {
		h.App.CompletionMonitor.Stop()
	}
	return h.RespondWithData(c, true)
}

// HandleGetTorrentCompletionMonitorStatus
//
//	@route  /api/v1/torrent-completion-monitor/status [GET]
//	@returns completion_monitor.Status
func (h *Handler) HandleGetTorrentCompletionMonitorStatus(c echo.Context) error {
	if h.App.CompletionMonitor == nil {
		// Return inactive default
		return c.JSON(http.StatusOK, completion_monitor.Status{Running: false})
	}
	return h.RespondWithData(c, h.App.CompletionMonitor.GetStatus())
}

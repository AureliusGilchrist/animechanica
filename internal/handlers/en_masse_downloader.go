package handlers

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

// HandleStartEnMasseDownload starts the en masse download process
//
//	@summary starts the en masse download process for all WeebCentral manga.
//	@route /api/v1/manga/en-masse/start [POST]
//	@returns bool
func (h *Handler) HandleStartEnMasseDownload(c echo.Context) error {
	if h.App.EnMasseDownloader == nil {
		return h.RespondWithError(c, fmt.Errorf("en masse downloader not initialized"))
	}

	if h.App.EnMasseDownloader.IsRunning() {
		return h.RespondWithError(c, fmt.Errorf("en masse download is already running"))
	}

	go h.App.EnMasseDownloader.Start()

	return h.RespondWithData(c, true)
}

// HandlePauseEnMasseDownload pauses the en masse download process
//
//	@summary pauses the en masse download process.
//	@route /api/v1/manga/en-masse/pause [POST]
//	@returns bool
func (h *Handler) HandlePauseEnMasseDownload(c echo.Context) error {
	if h.App.EnMasseDownloader == nil {
		return h.RespondWithError(c, fmt.Errorf("en masse downloader not initialized"))
	}

	h.App.EnMasseDownloader.Pause()
	return h.RespondWithData(c, true)
}

// HandleResumeEnMasseDownload resumes the en masse download process
//
//	@summary resumes the en masse download process.
//	@route /api/v1/manga/en-masse/resume [POST]
//	@returns bool
func (h *Handler) HandleResumeEnMasseDownload(c echo.Context) error {
	if h.App.EnMasseDownloader == nil {
		return h.RespondWithError(c, fmt.Errorf("en masse downloader not initialized"))
	}

	h.App.EnMasseDownloader.Resume()
	return h.RespondWithData(c, true)
}

// HandleStopEnMasseDownload stops the en masse download process
//
//	@summary stops the en masse download process.
//	@route /api/v1/manga/en-masse/stop [POST]
//	@returns bool
func (h *Handler) HandleStopEnMasseDownload(c echo.Context) error {
	if h.App.EnMasseDownloader == nil {
		return h.RespondWithError(c, fmt.Errorf("en masse downloader not initialized"))
	}

	h.App.EnMasseDownloader.Stop()
	return h.RespondWithData(c, true)
}

// HandleGetEnMasseStatus returns the current status of the en masse download process
//
//	@summary returns the current status of the en masse download process.
//	@route /api/v1/manga/en-masse/status [GET]
//	@returns map[string]interface{}
func (h *Handler) HandleGetEnMasseStatus(c echo.Context) error {
	if h.App.EnMasseDownloader == nil {
		return h.RespondWithError(c, fmt.Errorf("en masse downloader not initialized"))
	}

	status := h.App.EnMasseDownloader.GetStatus()
	return h.RespondWithData(c, status)
}

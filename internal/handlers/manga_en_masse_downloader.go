package handlers

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

// HandleStartEnMasseDownloader
//
//	@summary starts the en masse downloader process.
//	@desc This will begin downloading all manga series from the weebcentral catalogue sequentially.
//	@route /api/v1/manga/en-masse-downloader/start [POST]
//	@returns bool
func (h *Handler) HandleStartEnMasseDownloader(c echo.Context) error {

	if h.App.EnMasseDownloader == nil {
		return h.RespondWithError(c, fmt.Errorf("en masse downloader not initialized"))
	}

	err := h.App.EnMasseDownloader.Start()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleStopEnMasseDownloader
//
//	@summary stops the en masse downloader process.
//	@route /api/v1/manga/en-masse-downloader/stop [POST]
//	@returns bool
func (h *Handler) HandleStopEnMasseDownloader(c echo.Context) error {

	if h.App.EnMasseDownloader == nil {
		return h.RespondWithError(c, fmt.Errorf("en masse downloader not initialized"))
	}

	err := h.App.EnMasseDownloader.Stop()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleGetEnMasseDownloaderStatus
//
//	@summary returns the current status of the en masse downloader.
//	@route /api/v1/manga/en-masse-downloader/status [GET]
//	@returns manga.EnMasseDownloaderStatus
func (h *Handler) HandleGetEnMasseDownloaderStatus(c echo.Context) error {

	if h.App.EnMasseDownloader == nil {
		return h.RespondWithError(c, fmt.Errorf("en masse downloader not initialized"))
	}

	status := h.App.EnMasseDownloader.GetStatus()
	return h.RespondWithData(c, status)
}

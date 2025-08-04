package handlers

import (
	"fmt"
	"seanime/internal/events"
	"seanime/internal/manga"
	chapter_downloader "seanime/internal/manga/downloader"
	"time"

	"github.com/labstack/echo/v4"
)

// HandleDownloadMangaChapters
//
//	@summary adds chapters to the download queue.
//	@route /api/v1/manga/download-chapters [POST]
//	@returns bool
func (h *Handler) HandleDownloadMangaChapters(c echo.Context) error {

	type body struct {
		MediaId    int      `json:"mediaId"`
		Provider   string   `json:"provider"`
		ChapterIds []string `json:"chapterIds"`
		StartNow   bool     `json:"startNow"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	h.App.WSEventManager.SendEvent(events.InfoToast, "Adding chapters to download queue...")

	// Get manga title for series-based directory structure
	mangaTitle, err := h.getMangaTitleForDownload(c, b.MediaId)
	if err != nil {
		h.App.Logger.Warn().Err(err).Msg("Failed to get manga title, using fallback")
		mangaTitle = fmt.Sprintf("Manga_%d", b.MediaId)
	}

	// Get provider-specific rate limit for chapter queuing (to prevent 429 errors)
	queueRateLimit := getProviderChapterQueueRateLimit(b.Provider)

	// Add all chapters to the download queue with retry mechanism
	for i, chapterId := range b.ChapterIds {
		err := h.retryChapterQueue(manga.DownloadChapterOptions{
			Provider:   b.Provider,
			MediaId:    b.MediaId,
			ChapterId:  chapterId,
			StartNow:   b.StartNow,
			MangaTitle: mangaTitle,
		}, fmt.Sprintf("Chapter %s", chapterId))
		if err != nil {
			return h.RespondWithError(c, fmt.Errorf("failed to queue chapter %s after retries: %w", chapterId, err))
		}
		// Apply minimal rate limiting between chapter queuing to prevent 429 errors
		if i < len(b.ChapterIds)-1 {
			time.Sleep(queueRateLimit)
		}
	}

	return h.RespondWithData(c, true)
}

// HandleGetMangaDownloadData
//
//	@summary returns the download data for a specific media.
//	@desc This is used to display information about the downloaded and queued chapters in the UI.
//	@desc If the 'cached' parameter is false, it will refresh the data by rescanning the download folder.
//	@route /api/v1/manga/download-data [POST]
//	@returns manga.MediaDownloadData
func (h *Handler) HandleGetMangaDownloadData(c echo.Context) error {

	type body struct {
		MediaId int  `json:"mediaId"`
		Cached  bool `json:"cached"` // If false, it will refresh the data
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	data, err := h.App.MangaDownloader.GetMediaDownloads(b.MediaId, b.Cached)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, data)
}

// HandleGetMangaDownloadQueue
//
//	@summary returns the items in the download queue.
//	@route /api/v1/manga/download-queue [GET]
//	@returns []models.ChapterDownloadQueueItem
func (h *Handler) HandleGetMangaDownloadQueue(c echo.Context) error {

	data, err := h.App.Database.GetChapterDownloadQueue()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, data)
}

// HandleStartMangaDownloadQueue
//
//	@summary starts the download queue if it's not already running.
//	@desc This will start the download queue if it's not already running.
//	@desc Returns 'true' whether the queue was started or not.
//	@route /api/v1/manga/download-queue/start [POST]
//	@returns bool
func (h *Handler) HandleStartMangaDownloadQueue(c echo.Context) error {

	h.App.MangaDownloader.RunChapterDownloadQueue()

	return h.RespondWithData(c, true)
}

// HandleStopMangaDownloadQueue
//
//	@summary stops the manga download queue.
//	@desc This will stop the manga download queue.
//	@desc Returns 'true' whether the queue was stopped or not.
//	@route /api/v1/manga/download-queue/stop [POST]
//	@returns bool
func (h *Handler) HandleStopMangaDownloadQueue(c echo.Context) error {

	h.App.MangaDownloader.StopChapterDownloadQueue()

	return h.RespondWithData(c, true)

}

// HandleClearAllChapterDownloadQueue
//
//	@summary clears all chapters from the download queue.
//	@desc This will clear all chapters from the download queue.
//	@desc Returns 'true' whether the queue was cleared or not.
//	@desc This will also send a websocket event telling the client to refetch the download queue.
//	@route /api/v1/manga/download-queue [DELETE]
//	@returns bool
func (h *Handler) HandleClearAllChapterDownloadQueue(c echo.Context) error {

	err := h.App.Database.ClearAllChapterDownloadQueueItems()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	h.App.WSEventManager.SendEvent(events.ChapterDownloadQueueUpdated, nil)

	return h.RespondWithData(c, true)
}

// HandleResetErroredChapterDownloadQueue
//
//	@summary resets the errored chapters in the download queue.
//	@desc This will reset the errored chapters in the download queue, so they can be re-downloaded.
//	@desc Returns 'true' whether the queue was reset or not.
//	@desc This will also send a websocket event telling the client to refetch the download queue.
//	@route /api/v1/manga/download-queue/reset-errored [POST]
//	@returns bool
func (h *Handler) HandleResetErroredChapterDownloadQueue(c echo.Context) error {

	err := h.App.Database.ResetErroredChapterDownloadQueueItems()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	h.App.WSEventManager.SendEvent(events.ChapterDownloadQueueUpdated, nil)

	return h.RespondWithData(c, true)
}

// HandleDeleteMangaDownloadedChapters
//
//	@summary deletes downloaded chapters.
//	@desc This will delete downloaded chapters from the filesystem.
//	@desc Returns 'true' whether the chapters were deleted or not.
//	@desc The client should refetch the download data after this.
//	@route /api/v1/manga/download-chapter [DELETE]
//	@returns bool
func (h *Handler) HandleDeleteMangaDownloadedChapters(c echo.Context) error {

	type body struct {
		DownloadIds []chapter_downloader.DownloadID `json:"downloadIds"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	err := h.App.MangaDownloader.DeleteChapters(b.DownloadIds)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleGetMangaDownloadsList
//
//	@summary displays the list of downloaded manga.
//	@desc This analyzes the download folder and returns a well-formatted structure for displaying downloaded manga.
//	@desc It returns a list of manga.DownloadListItem where the media data might be nil if it's not in the AniList collection.
//	@route /api/v1/manga/downloads [GET]
//	@returns []manga.DownloadListItem
func (h *Handler) HandleGetMangaDownloadsList(c echo.Context) error {
	sessionID := c.Get("Seanime-Client-Id").(string)

	mangaCollection, err := h.App.GetMangaCollectionForSession(sessionID, false)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	res, err := h.App.MangaDownloader.NewDownloadList(&manga.NewDownloadListOptions{
		MangaCollection: mangaCollection,
		AnilistPlatform: h.App.AnilistPlatform,
	})
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, res)
}

// retryChapterQueue attempts to queue a chapter with exponential backoff retry (15s, 30s, 1min)
func (h *Handler) retryChapterQueue(options manga.DownloadChapterOptions, chapterName string) error {
	retryDelays := []time.Duration{
		15 * time.Second,  // First retry after 15 seconds
		30 * time.Second,  // Second retry after 30 seconds
		60 * time.Second,  // Third retry after 1 minute
	}

	// First attempt
	err := h.App.MangaDownloader.DownloadChapter(options)
	if err == nil {
		return nil
	}

	h.App.WSEventManager.SendEvent(events.InfoToast, fmt.Sprintf("Chapter %s failed to queue, retrying...", chapterName))

	// Retry attempts with exponential backoff
	for attempt, delay := range retryDelays {
		h.App.WSEventManager.SendEvent(events.InfoToast, fmt.Sprintf("Retrying chapter %s in %v (attempt %d/3)...", chapterName, delay, attempt+1))
		time.Sleep(delay)

		err = h.App.MangaDownloader.DownloadChapter(options)
		if err == nil {
			h.App.WSEventManager.SendEvent(events.SuccessToast, fmt.Sprintf("Chapter %s queued successfully after retry", chapterName))
			return nil
		}

		h.App.WSEventManager.SendEvent(events.InfoToast, fmt.Sprintf("Chapter %s retry %d failed: %v", chapterName, attempt+1, err))
	}

	// All retries failed
	return fmt.Errorf("failed after 3 retries: %w", err)
}

// getProviderChapterQueueRateLimit returns the rate limiting delay for queuing chapters from different providers
// This prevents 429 "Too Many Requests" errors when fetching chapter page lists
func getProviderChapterQueueRateLimit(provider string) time.Duration {
	switch provider {
	case "weebcentral":
		return 500 * time.Millisecond // WeebCentral has strict rate limits
	case "mangadx":
		return 300 * time.Millisecond // MangaDx has moderate rate limits
	case "comick":
		return 200 * time.Millisecond
	case "mangafire":
		return 250 * time.Millisecond
	case "manganato":
		return 150 * time.Millisecond
	case "mangapill":
		return 100 * time.Millisecond
	default:
		return 200 * time.Millisecond // Default rate limit
	}
}

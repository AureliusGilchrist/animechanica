package handlers

// TEMPORARILY DISABLED: Anime batch downloader backend has dependency issues
// TODO: Fix hibike anime extension imports and missing dependencies
/*
import (
	"seanime/internal/anime"
	"strconv"

	"github.com/labstack/echo/v4"
)

// HandleStartAnimeBatchDownload starts a batch anime download
//
//	@summary starts a batch anime download
//	@desc This starts a batch download operation for anime. All anime downloads are performed in batches and automatically linked.
//	@returns anime.BatchDownloadJob
//	@param type - string - true - "Batch download type (anime, season, year, genre)"
//	@route /api/v1/anime/batch-download [POST]
func (h *Handler) HandleStartAnimeBatchDownload(c echo.Context) error {
	type request struct {
		Type     string                         `json:"type"`
		Criteria map[string]interface{}         `json:"criteria"`
		Settings *anime.BatchDownloadSettings   `json:"settings"`
	}

	var req request
	if err := c.Bind(&req); err != nil {
		return h.RespondWithError(c, err)
	}

	// Ensure settings exist with defaults
	if req.Settings == nil {
		req.Settings = &anime.BatchDownloadSettings{
			Quality:             "1080p",
			Language:            "dual",
			MinSeeders:          5,
			ConcurrentDownloads: 3,
			AutoLink:            true, // Always auto-link
		}
	} else {
		// Force auto-link to always be true
		req.Settings.AutoLink = true
	}

	batchType := anime.BatchDownloadType(req.Type)
	job, err := h.App.AnimeEnMasseDownloader.StartBatchDownload(c.Request().Context(), batchType, req.Criteria, req.Settings)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, job)
}

// HandleDownloadSingleAnime downloads a single anime (using batch system)
//
//	@summary downloads a single anime
//	@desc This downloads a single anime using the batch system with automatic linking.
//	@returns anime.BatchDownloadJob
//	@param title - string - true - "Anime title to download"
//	@route /api/v1/anime/download [POST]
func (h *Handler) HandleDownloadSingleAnime(c echo.Context) error {
	type request struct {
		Title    string                         `json:"title"`
		Settings *anime.BatchDownloadSettings   `json:"settings"`
	}

	var req request
	if err := c.Bind(&req); err != nil {
		return h.RespondWithError(c, err)
	}

	if req.Title == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "title is required"))
	}

	// Ensure settings exist with defaults
	if req.Settings == nil {
		req.Settings = &anime.BatchDownloadSettings{
			Quality:             "1080p",
			Language:            "dual",
			MinSeeders:          5,
			ConcurrentDownloads: 1, // Single anime, so 1 concurrent
			AutoLink:            true, // Always auto-link
		}
	} else {
		// Force auto-link to always be true
		req.Settings.AutoLink = true
	}

	job, err := h.App.AnimeEnMasseDownloader.DownloadSingleAnime(c.Request().Context(), req.Title, req.Settings)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, job)
}

// HandleGetAnimeBatchJobs gets all active batch download jobs
//
//	@summary gets active anime batch download jobs
//	@desc This returns all currently active anime batch download jobs.
//	@returns []anime.BatchDownloadJob
//	@route /api/v1/anime/batch-download/jobs [GET]
func (h *Handler) HandleGetAnimeBatchJobs(c echo.Context) error {
	jobs := h.App.AnimeEnMasseDownloader.GetActiveBatchJobs()
	return h.RespondWithData(c, jobs)
}

// HandleGetAnimeBatchJob gets a specific batch download job
//
//	@summary gets a specific anime batch download job
//	@desc This returns details of a specific anime batch download job.
//	@returns anime.BatchDownloadJob
//	@param jobId - string - true - "Batch job ID"
//	@route /api/v1/anime/batch-download/jobs/{jobId} [GET]
func (h *Handler) HandleGetAnimeBatchJob(c echo.Context) error {
	jobID := c.Param("jobId")
	if jobID == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "jobId is required"))
	}

	job, exists := h.App.AnimeEnMasseDownloader.GetBatchJob(jobID)
	if !exists {
		return h.RespondWithError(c, echo.NewHTTPError(404, "batch job not found"))
	}

	return h.RespondWithData(c, job)
}

// HandleCancelAnimeBatchDownload cancels a batch download
//
//	@summary cancels an anime batch download
//	@desc This cancels an active anime batch download job.
//	@returns map[string]string
//	@param jobId - string - true - "Batch job ID"
//	@route /api/v1/anime/batch-download/jobs/{jobId}/cancel [POST]
func (h *Handler) HandleCancelAnimeBatchDownload(c echo.Context) error {
	jobID := c.Param("jobId")
	if jobID == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "jobId is required"))
	}

	err := h.App.AnimeEnMasseDownloader.CancelBatchDownload(jobID)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, map[string]string{"message": "batch download cancelled"})
}

// HandleSearchAnimeEpisodes searches for anime episodes
//
//	@summary searches for anime episodes
//	@desc This searches for episodes of an anime using the specified provider.
//	@returns anime.EpisodeContainer
//	@param mediaId - int - true - "AniList anime media ID"
//	@param provider - string - true - "Provider name"
//	@route /api/v1/anime/episodes/search [POST]
func (h *Handler) HandleSearchAnimeEpisodes(c echo.Context) error {
	type request struct {
		MediaID  int      `json:"mediaId"`
		Provider string   `json:"provider"`
		Titles   []string `json:"titles"`
	}

	var req request
	if err := c.Bind(&req); err != nil {
		return h.RespondWithError(c, err)
	}

	if req.MediaID == 0 {
		return h.RespondWithError(c, echo.NewHTTPError(400, "mediaId is required"))
	}

	if req.Provider == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "provider is required"))
	}

	if len(req.Titles) == 0 {
		return h.RespondWithError(c, echo.NewHTTPError(400, "titles are required"))
	}

	container, err := h.App.AnimeRepository.SearchEpisodes(c.Request().Context(), req.Provider, req.MediaID, req.Titles)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, container)
}

// HandleGetAnimeEpisodeStreamLinks gets stream links for an episode
//
//	@summary gets stream links for an anime episode
//	@desc This gets stream links and subtitles for a specific anime episode.
//	@returns anime.EpisodePageContainer
//	@param mediaId - int - true - "AniList anime media ID"
//	@param provider - string - true - "Provider name"
//	@param episodeId - string - true - "Episode ID"
//	@route /api/v1/anime/episodes/stream-links [POST]
func (h *Handler) HandleGetAnimeEpisodeStreamLinks(c echo.Context) error {
	type request struct {
		MediaID   int    `json:"mediaId"`
		Provider  string `json:"provider"`
		EpisodeID string `json:"episodeId"`
	}

	var req request
	if err := c.Bind(&req); err != nil {
		return h.RespondWithError(c, err)
	}

	if req.MediaID == 0 {
		return h.RespondWithError(c, echo.NewHTTPError(400, "mediaId is required"))
	}

	if req.Provider == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "provider is required"))
	}

	if req.EpisodeID == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "episodeId is required"))
	}

	container, err := h.App.AnimeRepository.GetEpisodeStreamLinks(c.Request().Context(), req.Provider, req.MediaID, req.EpisodeID)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, container)
}

// HandleGetAnimeProviders gets available anime providers
//
//	@summary gets available anime providers
//	@desc This returns a list of available anime providers.
//	@returns []string
//	@route /api/v1/anime/providers [GET]
func (h *Handler) HandleGetAnimeProviders(c echo.Context) error {
	providers := h.App.AnimeRepository.GetAvailableProviders()
	return h.RespondWithData(c, providers)
}

// HandleGetAnimeProviderInfo gets information about a specific provider
//
//	@summary gets anime provider information
//	@desc This returns information about a specific anime provider.
//	@returns map[string]interface{}
//	@param provider - string - true - "Provider name"
//	@route /api/v1/anime/providers/{provider} [GET]
func (h *Handler) HandleGetAnimeProviderInfo(c echo.Context) error {
	provider := c.Param("provider")
	if provider == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "provider is required"))
	}

	info := h.App.AnimeRepository.GetProviderInfo(provider)
	if info == nil {
		return h.RespondWithError(c, echo.NewHTTPError(404, "provider not found"))
	}

	return h.RespondWithData(c, info)
}

// HandleGetAnimeLatestEpisodeNumbers gets latest episode numbers for anime
//
//	@summary gets latest episode numbers for anime
//	@desc This returns the latest episode numbers for the specified anime from all providers.
//	@returns map[int][]anime.AnimeLatestEpisodeNumberItem
//	@route /api/v1/anime/latest-episodes [POST]
func (h *Handler) HandleGetAnimeLatestEpisodeNumbers(c echo.Context) error {
	type request struct {
		MediaIDs []int `json:"mediaIds"`
	}

	var req request
	if err := c.Bind(&req); err != nil {
		return h.RespondWithError(c, err)
	}

	if len(req.MediaIDs) == 0 {
		return h.RespondWithError(c, echo.NewHTTPError(400, "mediaIds are required"))
	}

	latestEpisodes := h.App.AnimeRepository.GetAnimeLatestEpisodeNumbersMap(req.MediaIDs)
	return h.RespondWithData(c, latestEpisodes)
}

// HandleRefreshAnimeLatestEpisodeNumbers refreshes latest episode data
//
//	@summary refreshes latest episode numbers for anime
//	@desc This refreshes the cached latest episode data for the specified anime.
//	@returns map[string]string
//	@route /api/v1/anime/latest-episodes/refresh [POST]
func (h *Handler) HandleRefreshAnimeLatestEpisodeNumbers(c echo.Context) error {
	type request struct {
		MediaIDs []int `json:"mediaIds"`
	}

	var req request
	if err := c.Bind(&req); err != nil {
		return h.RespondWithError(c, err)
	}

	if len(req.MediaIDs) == 0 {
		return h.RespondWithError(c, echo.NewHTTPError(400, "mediaIds are required"))
	}

	h.App.AnimeRepository.RefreshAnimeLatestEpisodeNumbers(req.MediaIDs)
	return h.RespondWithData(c, map[string]string{"message": "refresh initiated"})
}

// HandleGetAnimeDownloadTasks gets active download tasks
//
//	@summary gets active anime download tasks
//	@desc This returns all currently active anime download tasks.
//	@returns []anime.DownloadTask
//	@route /api/v1/anime/downloads [GET]
func (h *Handler) HandleGetAnimeDownloadTasks(c echo.Context) error {
	tasks := h.App.AnimeDownloadManager.GetActiveDownloads()
	return h.RespondWithData(c, tasks)
}

// HandleGetAnimeDownloadTask gets a specific download task
//
//	@summary gets a specific anime download task
//	@desc This returns details of a specific anime download task.
//	@returns anime.DownloadTask
//	@param taskId - string - true - "Download task ID"
//	@route /api/v1/anime/downloads/{taskId} [GET]
func (h *Handler) HandleGetAnimeDownloadTask(c echo.Context) error {
	taskID := c.Param("taskId")
	if taskID == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "taskId is required"))
	}

	task, exists := h.App.AnimeDownloadManager.GetDownloadTask(taskID)
	if !exists {
		return h.RespondWithError(c, echo.NewHTTPError(404, "download task not found"))
	}

	return h.RespondWithData(c, task)
}

// HandleCancelAnimeDownload cancels a download task
//
//	@summary cancels an anime download task
//	@desc This cancels an active anime download task.
//	@returns map[string]string
//	@param taskId - string - true - "Download task ID"
//	@route /api/v1/anime/downloads/{taskId}/cancel [POST]
func (h *Handler) HandleCancelAnimeDownload(c echo.Context) error {
	taskID := c.Param("taskId")
	if taskID == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "taskId is required"))
	}

	err := h.App.AnimeDownloadManager.CancelDownload(taskID)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, map[string]string{"message": "download cancelled"})
}
*/

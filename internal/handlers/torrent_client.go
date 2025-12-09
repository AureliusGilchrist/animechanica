package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"seanime/internal/api/anilist"
	"seanime/internal/database/db_bridge"
	"seanime/internal/events"
	hibiketorrent "seanime/internal/extension/hibike/torrent"
	"seanime/internal/torrent_clients/torrent_client"
	"seanime/internal/util"

	"github.com/labstack/echo/v4"
)

// HandleGetActiveTorrentList
//
//	@summary returns all active torrents.
//	@desc This handler is used by the client to display the active torrents.
//
//	@route /api/v1/torrent-client/list [GET]
//	@returns []torrent_client.Torrent
func (h *Handler) HandleGetActiveTorrentList(c echo.Context) error {

	// Get torrent list
	res, err := h.App.TorrentClientRepository.GetActiveTorrents()
	// If an error occurred, try to start the torrent client and get the list again
	// DEVNOTE: We try to get the list first because this route is called repeatedly by the client.
	if err != nil {
		ok := h.App.TorrentClientRepository.Start()
		if !ok {
			return h.RespondWithError(c, errors.New("could not start torrent client, verify your settings"))
		}
		res, err = h.App.TorrentClientRepository.GetActiveTorrents()
	}

	return h.RespondWithData(c, res)

}

// HandleTorrentClientAction
//
//	@summary performs an action on a torrent.
//	@desc This handler is used to pause, resume or remove a torrent.
//	@route /api/v1/torrent-client/action [POST]
//	@returns bool
func (h *Handler) HandleTorrentClientAction(c echo.Context) error {

	type body struct {
		Hash   string `json:"hash"`
		Action string `json:"action"`
		Dir    string `json:"dir"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	if b.Hash == "" || b.Action == "" {
		return h.RespondWithError(c, errors.New("missing arguments"))
	}

	switch b.Action {
	case "pause":
		err := h.App.TorrentClientRepository.PauseTorrents([]string{b.Hash})
		if err != nil {
			return h.RespondWithError(c, err)
		}
	case "resume":
		err := h.App.TorrentClientRepository.ResumeTorrents([]string{b.Hash})
		if err != nil {
			return h.RespondWithError(c, err)
		}
	case "remove":
		err := h.App.TorrentClientRepository.RemoveTorrents([]string{b.Hash})
		if err != nil {
			return h.RespondWithError(c, err)
		}
	case "open":
		if b.Dir == "" {
			return h.RespondWithError(c, errors.New("directory not found"))
		}
		OpenDirInExplorer(b.Dir)
	}

	return h.RespondWithData(c, true)

}

// HandleTorrentClientGetFiles
//
//	@summary gets the files of a torrent.
//	@desc This handler is used to get the files of a torrent.
//	@route /api/v1/torrent-client/get-files [POST]
//	@returns []string
func (h *Handler) HandleTorrentClientGetFiles(c echo.Context) error {

	type body struct {
		Torrent  *hibiketorrent.AnimeTorrent `json:"torrent"`
		Provider string                      `json:"provider"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	if b.Torrent == nil || b.Torrent.InfoHash == "" {
		return h.RespondWithError(c, errors.New("missing arguments"))
	}

	tempDir, err := os.MkdirTemp("", "torrent-")
	if err != nil {
		return h.RespondWithError(c, err)
	}
	defer os.RemoveAll(tempDir)

	// Get the torrent's provider extension
	providerExtension, ok := h.App.TorrentRepository.GetAnimeProviderExtension(b.Provider)
	if !ok {
		return h.RespondWithError(c, errors.New("provider extension not found for torrent"))
	}
	// Get the magnet
	magnet, err := providerExtension.GetProvider().GetTorrentMagnetLink(b.Torrent)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	exists := h.App.TorrentClientRepository.TorrentExists(b.Torrent.InfoHash)

	if !exists {
		h.App.Logger.Info().Msgf("torrent client: Torrent %s does not exist, adding", b.Torrent.InfoHash)
		// Add the torrent
		err = h.App.TorrentClientRepository.AddMagnets([]string{magnet}, tempDir)
		if err != nil {
			return err
		}
	}

	h.App.Logger.Info().Msgf("torrent client: Getting files for %s", b.Torrent.InfoHash)
	files, err := h.App.TorrentClientRepository.GetFiles(b.Torrent.InfoHash)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	if !exists {
		h.App.Logger.Info().Msgf("torrent client: Removing torrent %s", b.Torrent.InfoHash)
		_ = h.App.TorrentClientRepository.RemoveTorrents([]string{b.Torrent.InfoHash})
	}

	return h.RespondWithData(c, files)
}

// HandleTorrentClientDownload
//
//	@summary adds torrents to the torrent client.
//	@desc It fetches the magnets from the provided URLs and adds them to the torrent client.
//	@desc If smart select is enabled, it will try to select the best torrent based on the missing episodes.
//	@route /api/v1/torrent-client/download [POST]
//	@returns bool
func (h *Handler) HandleTorrentClientDownload(c echo.Context) error {

	type body struct {
		Torrents    []hibiketorrent.AnimeTorrent `json:"torrents"`
		Destination string                       `json:"destination"`
		SmartSelect struct {
			Enabled               bool  `json:"enabled"`
			MissingEpisodeNumbers []int `json:"missingEpisodeNumbers"`
		} `json:"smartSelect"`
		Deselect struct {
			Enabled bool  `json:"enabled"`
			Indices []int `json:"indices"`
		} `json:"deselect,omitempty"`
		Media *anilist.BaseAnime `json:"media"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	if b.Destination == "" {
		return h.RespondWithError(c, errors.New("destination not found"))
	}

	if !filepath.IsAbs(b.Destination) {
		return h.RespondWithError(c, errors.New("destination path must be absolute"))
	}

	// Check that the destination path is a library path
	//libraryPaths, err := h.App.Database.GetAllLibraryPathsFromSettings()
	//if err != nil {
	//	return h.RespondWithError(c, err)
	//}
	//isInLibrary := util.IsSubdirectoryOfAny(libraryPaths, b.Destination)
	//if !isInLibrary {
	//	return h.RespondWithError(c, errors.New("destination path is not a library path"))
	//}

	// try to start torrent client if it's not running
	ok := h.App.TorrentClientRepository.Start()
	if !ok {
		return h.RespondWithError(c, errors.New("could not contact torrent client, verify your settings or make sure it's running"))
	}

	var completeAnime *anilist.CompleteAnime
	var err error
	completeAnime, err = h.App.AnilistPlatformRef.Get().GetAnimeWithRelations(c.Request().Context(), b.Media.ID)
	if err != nil {
		completeAnime = b.Media.ToCompleteAnime()
	}

	if b.SmartSelect.Enabled {
		if len(b.Torrents) > 1 {
			return h.RespondWithError(c, errors.New("smart select is not supported for multiple torrents"))
		}

		// smart select
		err = h.App.TorrentClientRepository.SmartSelect(&torrent_client.SmartSelectParams{
			Torrent:          &b.Torrents[0],
			EpisodeNumbers:   b.SmartSelect.MissingEpisodeNumbers,
			Media:            completeAnime,
			Destination:      b.Destination,
			PlatformRef:      h.App.AnilistPlatformRef,
			ShouldAddTorrent: true,
		})
		if err != nil {
			return h.RespondWithError(c, err)
		}
	}

	if b.Deselect.Enabled {
		err = h.App.TorrentClientRepository.DeselectAndDownload(&torrent_client.DeselectAndDownloadParams{
			Torrent:          &b.Torrents[0],
			FileIndices:      b.Deselect.Indices,
			Destination:      b.Destination,
			ShouldAddTorrent: true,
		})
		if err != nil {
			return h.RespondWithError(c, err)
		}
	} else {

		// Get magnets
		magnets := make([]string, 0)
		for _, t := range b.Torrents {
			// Get the torrent's provider extension
			providerExtension, ok := h.App.TorrentRepository.GetAnimeProviderExtension(t.Provider)
			if !ok {
				return h.RespondWithError(c, errors.New("provider extension not found for torrent"))
			}
			// Get the torrent magnet link
			magnet, err := providerExtension.GetProvider().GetTorrentMagnetLink(&t)
			if err != nil {
				return h.RespondWithError(c, err)
			}

			magnets = append(magnets, magnet)
		}

		// try to add torrents to client, on error return error
		err = h.App.TorrentClientRepository.AddMagnets(magnets, b.Destination)
		if err != nil {
			return h.RespondWithError(c, err)
		}
	}

	// Save pre-match association so the scanner can directly match files to this anime
	// This avoids false positives from fuzzy title matching
	if b.Media != nil && b.Media.ID > 0 {
		err = h.App.Database.SaveTorrentPreMatch(b.Destination, b.Media.ID)
		if err != nil {
			h.App.Logger.Warn().Err(err).Msg("torrent client: Failed to save torrent pre-match")
		} else {
			h.App.Logger.Info().
				Int("mediaId", b.Media.ID).
				Str("destination", b.Destination).
				Msg("torrent client: Saved torrent pre-match for accurate file matching")
		}
	}

	// Add the media to the collection (if it wasn't already)
	go func() {
		defer util.HandlePanicInModuleThen("handlers/HandleTorrentClientDownload", func() {})
		if b.Media != nil {
			// Check if the media is already in the collection
			animeCollection, err := h.App.GetAnimeCollection(false)
			if err != nil {
				return
			}
			_, found := animeCollection.FindAnime(b.Media.ID)
			if found {
				return
			}
			// Add the media to the collection
			err = h.App.AnilistPlatformRef.Get().AddMediaToCollection(c.Request().Context(), []int{b.Media.ID})
			if err != nil {
				h.App.Logger.Error().Err(err).Msg("anilist: Failed to add media to collection")
			}
			ac, _ := h.App.RefreshAnimeCollection()
			h.App.WSEventManager.SendEvent(events.RefreshedAnilistAnimeCollection, ac)
		}
	}()

	return h.RespondWithData(c, true)

}

// HandleTorrentClientAddMagnetFromRule
//
//	@summary adds magnets to the torrent client based on the AutoDownloader item.
//	@desc This is used to download torrents that were queued by the AutoDownloader.
//	@desc The item will be removed from the queue if the magnet was added successfully.
//	@desc The AutoDownloader items should be re-fetched after this.
//	@route /api/v1/torrent-client/rule-magnet [POST]
//	@returns bool
func (h *Handler) HandleTorrentClientAddMagnetFromRule(c echo.Context) error {

	type body struct {
		MagnetUrl    string `json:"magnetUrl"`
		RuleId       uint   `json:"ruleId"`
		QueuedItemId uint   `json:"queuedItemId"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	if b.MagnetUrl == "" || b.RuleId == 0 {
		return h.RespondWithError(c, errors.New("missing parameters"))
	}

	// Get rule from database
	rule, err := db_bridge.GetAutoDownloaderRule(h.App.Database, b.RuleId)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// try to start torrent client if it's not running
	ok := h.App.TorrentClientRepository.Start()
	if !ok {
		return h.RespondWithError(c, errors.New("could not start torrent client, verify your settings"))
	}

	// try to add torrents to client, on error return error
	err = h.App.TorrentClientRepository.AddMagnets([]string{b.MagnetUrl}, rule.Destination)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	if b.QueuedItemId > 0 {
		// the magnet was added successfully, remove the item from the queue
		err = h.App.Database.DeleteAutoDownloaderItem(b.QueuedItemId)
	}

	return h.RespondWithData(c, true)

}

// MediaDownloadStatus represents the download status of a media item
type MediaDownloadStatus struct {
	MediaId  int                          `json:"mediaId"`
	Status   torrent_client.TorrentStatus `json:"status"`
	Progress float64                      `json:"progress"`
}

// HandleClearTorrentPreMatches
//
//	@summary clears all torrent pre-match entries from the database.
//	@desc This handler removes all stored associations between torrent destinations and media IDs.
//	@route /api/v1/torrent-client/clear-pre-matches [POST]
//	@returns bool
func (h *Handler) HandleClearTorrentPreMatches(c echo.Context) error {
	err := h.App.Database.ClearAllTorrentPreMatches()
	if err != nil {
		return h.RespondWithError(c, err)
	}
	h.App.Logger.Info().Msg("torrent client: Cleared all torrent pre-matches")
	return h.RespondWithData(c, true)
}

// HandleGetMediaDownloadingStatus
//
//	@summary returns the download status of media items that are currently downloading.
//	@desc This handler returns a map of media IDs to their download status based on active torrents.
//	@route /api/v1/torrent-client/media-downloading-status [GET]
//	@returns []MediaDownloadStatus
func (h *Handler) HandleGetMediaDownloadingStatus(c echo.Context) error {
	result := make([]MediaDownloadStatus, 0)

	// Get active torrents
	torrents, err := h.App.TorrentClientRepository.GetActiveTorrents()
	if err != nil {
		// Return empty result if torrent client is not available
		return h.RespondWithData(c, result)
	}

	// Get all pre-matches
	preMatches, err := h.App.Database.GetAllTorrentPreMatches()
	if err != nil {
		return h.RespondWithData(c, result)
	}

	// Create a map of destination paths to media IDs
	destToMediaId := make(map[string]int)
	for _, pm := range preMatches {
		destToMediaId[util.NormalizePath(pm.Destination)] = pm.MediaId
	}

	// Track which media IDs we've already added (to avoid duplicates)
	addedMediaIds := make(map[int]bool)

	// Match torrents to media IDs based on content path
	for _, torrent := range torrents {
		contentPath := util.NormalizePath(torrent.ContentPath)

		// Check if the torrent's content path matches any pre-match destination
		for destPath, mediaId := range destToMediaId {
			// Check if content path starts with or equals the destination path
			if len(contentPath) >= len(destPath) && contentPath[:len(destPath)] == destPath {
				if !addedMediaIds[mediaId] {
					result = append(result, MediaDownloadStatus{
						MediaId:  mediaId,
						Status:   torrent.Status,
						Progress: torrent.Progress,
					})
					addedMediaIds[mediaId] = true
				}
				break
			}
		}
	}

	return h.RespondWithData(c, result)
}

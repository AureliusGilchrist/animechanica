package handlers

import (
	"errors"
	"seanime/internal/database/db_bridge"
	"seanime/internal/library/scanner"
	"seanime/internal/library/summary"

	"github.com/labstack/echo/v4"
)

// HandleScanLocalFiles
//
//	@summary scans the user's library.
//	@desc This will scan the user's library.
//	@desc The response is ignored, the client should re-fetch the library after this.
//	@route /api/v1/library/scan [POST]
//	@returns []anime.LocalFile
func (h *Handler) HandleScanLocalFiles(c echo.Context) error {

	type body struct {
		Enhanced         bool `json:"enhanced"`
		SkipLockedFiles  bool `json:"skipLockedFiles"`
		SkipIgnoredFiles bool `json:"skipIgnoredFiles"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	// Retrieve the user's library path
	libraryPath, err := h.App.Database.GetLibraryPathFromSettings()
	if err != nil {
		return h.RespondWithError(c, err)
	}
	additionalLibraryPaths, err := h.App.Database.GetAdditionalLibraryPathsFromSettings()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Get the latest local files
	existingLfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// +---------------------+
	// |       Scanner       |
	// +---------------------+

	// Create scan summary logger
	scanSummaryLogger := summary.NewScanSummaryLogger()

	// Create a new scan logger
	scanLogger, err := scanner.NewScanLogger(h.App.Config.Logs.Dir)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	defer scanLogger.Done()

	// Create a new scanner
	sc := scanner.Scanner{
		DirPath:            libraryPath,
		OtherDirPaths:      additionalLibraryPaths,
		Enhanced:           b.Enhanced,
		Platform:           h.App.AnilistPlatform,
		Logger:             h.App.Logger,
		WSEventManager:     h.App.WSEventManager,
		ExistingLocalFiles: existingLfs,
		SkipLockedFiles:    b.SkipLockedFiles,
		SkipIgnoredFiles:   b.SkipIgnoredFiles,
		ScanSummaryLogger:  scanSummaryLogger,
		ScanLogger:         scanLogger,
		MetadataProvider:   h.App.MetadataProvider,
		MatchingAlgorithm:  h.App.Settings.GetLibrary().ScannerMatchingAlgorithm,
		MatchingThreshold:  h.App.Settings.GetLibrary().ScannerMatchingThreshold,
	}

	// Scan the library
	allLfs, err := sc.Scan(c.Request().Context())
	if err != nil {
		if errors.Is(err, scanner.ErrNoLocalFiles) {
			return h.RespondWithData(c, []interface{}{})
		} else {
			return h.RespondWithError(c, err)
		}
	}

	// Insert the local files
	lfs, err := db_bridge.InsertLocalFiles(h.App.Database, allLfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Save the scan summary
	_ = db_bridge.InsertScanSummary(h.App.Database, scanSummaryLogger.GenerateSummary())

	go h.App.AutoDownloader.CleanUpDownloadedItems()

	return h.RespondWithData(c, lfs)
}

// HandleRematchAllAnimeLinks
//
//	@summary bulk unmatches all anime-to-AniList links, then runs a full scan to rematch.
//	@desc This clears MediaId, unlocks, and unignores all local files, then triggers a scan using the configured matching algorithm (Romaji-prioritized) to rebuild links.
//	@route /api/v1/library/rematch-anime-links [POST]
//	@returns []anime.LocalFile
func (h *Handler) HandleRematchAllAnimeLinks(c echo.Context) error {

	// Step 1: Load local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Step 2: Clear all links without touching series data
	for _, lf := range lfs {
		lf.MediaId = 0
		lf.Locked = false
		lf.Ignored = false
	}

	// Step 3: Save cleared local files
	if _, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs); err != nil {
		return h.RespondWithError(c, err)
	}

	// Step 4: Prepare scan (reuse logic from HandleScanLocalFiles)
	libraryPath, err := h.App.Database.GetLibraryPathFromSettings()
	if err != nil {
		return h.RespondWithError(c, err)
	}
	additionalLibraryPaths, err := h.App.Database.GetAdditionalLibraryPathsFromSettings()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	existingLfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Create scan summary and logger
	scanSummaryLogger := summary.NewScanSummaryLogger()
	scanLogger, err := scanner.NewScanLogger(h.App.Config.Logs.Dir)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	defer scanLogger.Done()

	// Configure scanner. Use Enhanced=true for best matching and do not skip any files.
	sc := scanner.Scanner{
		DirPath:            libraryPath,
		OtherDirPaths:      additionalLibraryPaths,
		Enhanced:           true,
		Platform:           h.App.AnilistPlatform,
		Logger:             h.App.Logger,
		WSEventManager:     h.App.WSEventManager,
		ExistingLocalFiles: existingLfs,
		SkipLockedFiles:    false,
		SkipIgnoredFiles:   false,
		ScanSummaryLogger:  scanSummaryLogger,
		ScanLogger:         scanLogger,
		MetadataProvider:   h.App.MetadataProvider,
		MatchingAlgorithm:  h.App.Settings.GetLibrary().ScannerMatchingAlgorithm,
		MatchingThreshold:  h.App.Settings.GetLibrary().ScannerMatchingThreshold,
	}

	// Step 5: Run scan
	allLfs, err := sc.Scan(c.Request().Context())
	if err != nil {
		if errors.Is(err, scanner.ErrNoLocalFiles) {
			return h.RespondWithData(c, []interface{}{})
		} else {
			return h.RespondWithError(c, err)
		}
	}

	// Step 6: Persist scan results and summary
	ret, err := db_bridge.InsertLocalFiles(h.App.Database, allLfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	_ = db_bridge.InsertScanSummary(h.App.Database, scanSummaryLogger.GenerateSummary())

	go h.App.AutoDownloader.CleanUpDownloadedItems()

	// Step 7: Return updated local files
	return h.RespondWithData(c, ret)
}

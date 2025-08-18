package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"seanime/internal/database/db_bridge"
	"seanime/internal/events"
	"seanime/internal/library/anime"
	"seanime/internal/library/filesystem"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
)

// HandleGetLocalFiles
//
//	@summary returns all local files.
//	@desc Reminder that local files are scanned from the library path.
//	@route /api/v1/library/local-files [GET]
//	@returns []anime.LocalFile
func (h *Handler) HandleGetLocalFiles(c echo.Context) error {

	lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	return h.RespondWithData(c, lfs)
}

// HandleMoveAndRenameAnimeSeries
//
//	@summary Moves and renames all episodes of a linked series to "{SERIESNAME} - {XXX} - {EPISODENAME}{ext}" in the series directory.
//	@desc Given a mediaId, this moves files from their release folder(s) into the parent series folder with a consistent naming scheme.
//	@desc After successful moves, optionally deletes the original release folder(s) when confirmDelete is true.
//	@route /api/v1/library/anime-entry/move-rename [POST]
//	@returns { moved: number, skipped: number, deletedFolders: []string }
func (h *Handler) HandleMoveAndRenameAnimeSeries(c echo.Context) error {
	type body struct {
		MediaId        int     `json:"mediaId"`
		ConfirmDelete  bool    `json:"confirmDelete"`
		DryRun         bool    `json:"dryRun,omitempty"`
		IncludeNonMain *bool   `json:"includeNonMain,omitempty"` // nil => default true
		TargetDir      string  `json:"targetDir,omitempty"`      // optional override for destination directory
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}
	if b.MediaId == 0 {
		return h.RespondWithError(c, errors.New("mediaId is required"))
	}

	// Load local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Determine default for IncludeNonMain (default true if not provided)
	includeNonMain := true
	if b.IncludeNonMain != nil {
		includeNonMain = *b.IncludeNonMain
	}

	// Filter files for this media
	files := lo.Filter(lfs, func(lf *anime.LocalFile, _ int) bool {
		if lf.MediaId != b.MediaId {
			return false
		}
		if includeNonMain {
			// Include common video extensions even if type isn't Main
			name := strings.ToLower(lf.GetPath())
			return strings.HasSuffix(name, ".mkv") || strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".avi") || strings.HasSuffix(name, ".mov")
		}
		return lf.GetType() == anime.LocalFileTypeMain
	})
	if len(files) == 0 {
		return h.RespondWithError(c, errors.New("no linked episodes found for mediaId"))
	}

	// Determine series directory robustly to support both:
	// - Episodes inside a release subfolder: /Series/Title/[Release]/Ep1.mkv -> seriesDir=/Series/Title
	// - Files directly under the series folder (e.g., movies): /Series/Title/Movie.mkv -> seriesDir=/Series/Title
	sourceDirs := lo.Uniq(lo.Map(files, func(lf *anime.LocalFile, _ int) string { return filepath.Dir(lf.GetPath()) }))

	// Heuristic to detect release-like folder names
	releaseDirRe := regexp.MustCompile(`(?i)(\[.*\])|(bluray|web|bd|1080|720|480|x265|x264|hevc|aac|h264|10bit|multi|dual)`) // matches common release markers

	var seriesDir string
	if strings.TrimSpace(b.TargetDir) != "" {
		seriesDir = b.TargetDir
	} else {
		if len(sourceDirs) == 1 {
			src := sourceDirs[0]
			base := strings.ToLower(filepath.Base(src))
			if releaseDirRe.MatchString(base) {
				seriesDir = filepath.Dir(src)
			} else {
				// Files are already directly under the series directory
				seriesDir = src
			}
		} else {
			// Compute nearest common ancestor directory across all sourceDirs
			split := func(p string) []string {
				// Normalize to slash-separated for comparison
				return strings.Split(filepath.ToSlash(filepath.Clean(p)), "/")
			}
			parts := make([][]string, len(sourceDirs))
			for i, d := range sourceDirs {
				parts[i] = split(d)
			}
			// Find common prefix parts
			var common []string
			for idx := 0; ; idx++ {
				var cur string
				ok := true
				for i := range parts {
					if idx >= len(parts[i]) {
						ok = false
						break
					}
					if i == 0 {
						cur = parts[i][idx]
					} else if parts[i][idx] != cur {
						ok = false
						break
					}
				}
				if !ok {
					break
				}
				common = append(common, cur)
			}
			if len(common) == 0 {
				// Fallback to parent of first sourceDir
				seriesDir = filepath.Dir(sourceDirs[0])
			} else {
				seriesDir = filepath.FromSlash(strings.Join(common, "/"))
			}
		}
	}

	seriesName := filepath.Base(seriesDir)

	// If there's only one source directory and it's a direct child of seriesDir,
	// treat this as a single-recursion case: keep files inside that child folder
	// and name them after that folder instead of moving them up to seriesDir.
	singleRecursion := false
	singleRecursionDir := ""
	if len(sourceDirs) == 1 {
		candidate := sourceDirs[0]
		if filepath.Dir(candidate) == seriesDir {
			singleRecursion = true
			singleRecursionDir = candidate
		}
	}

	// Ensure all files share the same series directory structure; collect sourceDirs for deletion
	sourceDirs = lo.Uniq(lo.Map(files, func(lf *anime.LocalFile, _ int) string { return filepath.Dir(lf.GetPath()) }))

	// Prepare operations
	sanitize := func(name string) string {
		// Remove/replace characters illegal in filenames
		// Allow basic punctuation, replace others with space
		invalidRe := regexp.MustCompile(`[\\/:*?"<>|]`)
		trimmed := invalidRe.ReplaceAllString(name, " ")
		// Collapse multiple spaces
		spaceRe := regexp.MustCompile(`\s+`)
		return spaceRe.ReplaceAllString(trimmed, " ")
	}

	type moveResult struct {
		moved          int
		skipped        int
		errors         []string
		deletedFolders []string
	}
	res := &moveResult{}

	// Move/rename files
	for _, lf := range files {
		oldPath := lf.GetPath()
		ext := filepath.Ext(oldPath)
		ep := lf.GetEpisodeNumber()
		if ep < 0 {
			ep = 0
		}
		epTitle := lf.GetParsedEpisodeTitle()
		if len(epTitle) == 0 {
			epTitle = fmt.Sprintf("Episode %03d", ep)
		}
		// Decide destination directory and series name for naming based on recursion rule
		destDir := seriesDir
		nameSeries := seriesName
		if singleRecursion {
			destDir = singleRecursionDir
			nameSeries = filepath.Base(singleRecursionDir)
		}

		newName := fmt.Sprintf("%s - %03d - %s%s", sanitize(nameSeries), ep, sanitize(epTitle), ext)
		newPath := filepath.Join(destDir, newName)

		// If target exists, add a numeric suffix to avoid collision
		if !b.DryRun {
			try := 1
			base := strings.TrimSuffix(newName, ext)
			for {
				if _, statErr := os.Stat(newPath); statErr == nil {
					// exists, try next suffix
					suffixed := fmt.Sprintf("%s (%d)%s", base, try, ext)
					newPath = filepath.Join(destDir, suffixed)
					try++
					continue
				}
				break
			}
		}

		if oldPath == newPath {
			res.skipped++
			continue
		}

		if b.DryRun {
			// Simulate
			h.App.Logger.Info().Str("from", oldPath).Str("to", newPath).Msg("dry-run: move/rename")
			res.moved++
			continue
		}

		// Ensure destination directory exists
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			res.errors = append(res.errors, fmt.Sprintf("mkdir failed for %s: %v", destDir, err))
			continue
		}

		// Destination collision already handled above; just proceed

		if err := os.Rename(oldPath, newPath); err != nil {
			res.errors = append(res.errors, fmt.Sprintf("failed to move %s -> %s: %v", oldPath, newPath, err))
			continue
		}
		res.moved++

		// Update in-memory LocalFile so DB can be saved without rescan
		lf.Path = newPath
		lf.Name = filepath.Base(newPath)
	}

	// Optionally delete original release folders if moves succeeded for all files in those folders
	if !b.DryRun && b.ConfirmDelete {
		for _, src := range sourceDirs {
			// Attempt to remove dir if empty
			entries, readErr := os.ReadDir(src)
			if readErr != nil {
				res.errors = append(res.errors, fmt.Sprintf("readdir failed for %s: %v", src, readErr))
				continue
			}
			// If only leftover non-media files remain, we still attempt removal; try Remove and ignore not empty
			if err := os.Remove(src); err != nil {
				// Try recursive remove only if truly empty (double-check)
				if len(entries) == 0 {
					if rerr := os.RemoveAll(src); rerr == nil {
						res.deletedFolders = append(res.deletedFolders, src)
					} else {
						res.errors = append(res.errors, fmt.Sprintf("failed to remove %s: %v", src, rerr))
					}
				}
			} else {
				res.deletedFolders = append(res.deletedFolders, src)
			}
		}
	}

	// Persist changes to database and notify clients (when not a dry run)
	if !b.DryRun {
		if _, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs); err != nil {
			return h.RespondWithError(c, err)
		}
		// Invalidate relevant queries so UI updates immediately
		h.App.WSEventManager.SendEvent(events.InvalidateQueries, []string{
			events.GetLocalFilesEndpoint,
			events.GetAnimeEntryEndpoint,
			events.GetLibraryCollectionEndpoint,
			events.GetMissingEpisodesEndpoint,
		})
	}

	type response struct {
		Moved          int      `json:"moved"`
		Skipped        int      `json:"skipped"`
		DeletedFolders []string `json:"deletedFolders"`
		Errors         []string `json:"errors,omitempty"`
	}

	return h.RespondWithData(c, response{
		Moved:          res.moved,
		Skipped:        res.skipped,
		DeletedFolders: res.deletedFolders,
		Errors:         res.errors,
	})
}

// HandleListLinkedAnimeFiles
//
//	@summary lists linked anime files with filters and pagination.
//	@desc Query params:
//	@desc - source=auto|manual|any (default any)
//	@desc - blocked=true|false|any (default any)
//	@desc - mediaId=<int> (optional, filters to a single media)
//	@desc - page=<int> (1-based, default 1)
//	@desc - pageSize=<int> (default 20)
//	@desc - hidden=include|exclude|only (default exclude)
//	@route /api/v1/library/anime-linked [GET]
//	@returns { items: []anime.LocalFile, total: number, page: number, pageSize: number }
func (h *Handler) HandleListLinkedAnimeFiles(c echo.Context) error {

    // Parse query params
    source := c.QueryParam("source") // auto|manual|any
    if source == "" {
        source = "any"
    }
    blockedParam := c.QueryParam("blocked") // true|false|any
    if blockedParam == "" {
        blockedParam = "any"
    }
    mediaIdParam := c.QueryParam("mediaId")
    pageParam := c.QueryParam("page")
    pageSizeParam := c.QueryParam("pageSize")
    hiddenParam := c.QueryParam("hidden") // include|exclude|only
    if hiddenParam == "" {
        hiddenParam = "exclude"
    }

    // Defaults
    page := 1
    pageSize := 5000
    var mediaId int

    if pageParam != "" {
        fmt.Sscanf(pageParam, "%d", &page)
        if page <= 0 {
            page = 1
        }
    }
    if pageSizeParam != "" {
        fmt.Sscanf(pageSizeParam, "%d", &pageSize)
        // Allow large page sizes; clamp to 5000 if out of range
        if pageSize <= 0 || pageSize > 5000 {
            pageSize = 5000
        }
    }
    if mediaIdParam != "" {
        fmt.Sscanf(mediaIdParam, "%d", &mediaId)
    }

    // Load local files
    lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
    if err != nil {
        return h.RespondWithError(c, err)
    }

    // Filter linked files
    filtered := lo.Filter(lfs, func(lf *anime.LocalFile, _ int) bool {
        if lf.MediaId == 0 {
            return false
        }
        if mediaId != 0 && lf.MediaId != mediaId {
            return false
        }
        // source filter
        switch source {
        case "auto":
            if lf.LinkSource != "auto" {
                return false
            }
        case "manual":
            if lf.LinkSource != "manual" {
                return false
            }
        }
        // blocked filter
        switch blockedParam {
        case "true":
            if !lf.AutoMatchBlocked {
                return false
            }
        case "false":
            if lf.AutoMatchBlocked {
                return false
            }
        }
        // hidden filter
        switch hiddenParam {
        case "only":
            if !lf.Hidden {
                return false
            }
        case "exclude":
            if lf.Hidden {
                return false
            }
        }
        return true
    })

    total := len(filtered)
    // Pagination window
    start := (page - 1) * pageSize
    if start > total {
        start = total
    }
    end := start + pageSize
    if end > total {
        end = total
    }
    items := filtered[start:end]

	// Response structure
	type resp struct {
		Items    []*anime.LocalFile `json:"items"`
		Total    int                `json:"total"`
		Page     int                `json:"page"`
		PageSize int                `json:"pageSize"`
	}

	return h.RespondWithData(c, resp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// HandleUnmatchAnimeEntry
//
//	@summary unmatches either all files for a mediaId or specific paths.
//	@route /api/v1/library/anime-entry/unmatch [POST]
//	@returns bool
func (h *Handler) HandleUnmatchAnimeEntry(c echo.Context) error {
	type body struct {
		MediaId int      `json:"mediaId"`
		Paths   []string `json:"paths"`
		// When true, and if the current link was created automatically, block future auto-rematching
		BlockAutoRematch bool `json:"blockAutoRematch,omitempty"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}
	if b.MediaId == 0 && len(b.Paths) == 0 {
		return h.RespondWithError(c, errors.New("mediaId or paths required"))
	}

	// Load
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// If paths provided, unmatch those paths only
	if len(b.Paths) > 0 {
		for _, path := range b.Paths {
			lf, found := lo.Find(lfs, func(i *anime.LocalFile) bool { return i.HasSamePath(path) })
			if !found {
				continue
			}
			prevSource := lf.LinkSource
			lf.MediaId = 0
			lf.Locked = false
			lf.Ignored = false
			lf.LinkSource = ""
			if b.BlockAutoRematch && prevSource != "manual" {
				lf.AutoMatchBlocked = true
			}
		}
	} else {
		// Unmatch all for mediaId
		for _, lf := range lfs {
			if lf.MediaId == b.MediaId {
				prevSource := lf.LinkSource
				lf.PreviousMediaId = lf.MediaId
				lf.ResolvedState = "unmatched"
				lf.MediaId = 0
				lf.Locked = false
				lf.Ignored = false
				lf.LinkSource = ""
				if b.BlockAutoRematch && prevSource != "manual" {
					lf.AutoMatchBlocked = true
				}
			}
		}
	}

	// Save
	_, err = db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	return h.RespondWithData(c, true)
}

// HandleGetLinkedFilesByMedia
//
//	@summary returns local files linked to a given mediaId.
//	@route /api/v1/library/anime-entry/linked-files/:id [GET]
//	@returns []anime.LocalFile
func (h *Handler) HandleGetLinkedFilesByMedia(c echo.Context) error {
	idStr := c.Param("id")
	if idStr == "" {
		return h.RespondWithError(c, errors.New("missing id"))
	}
	// parse int
	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return h.RespondWithError(c, errors.New("invalid id"))
	}

	lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	linked := lo.Filter(lfs, func(i *anime.LocalFile, _ int) bool { return i.MediaId == id })
	return h.RespondWithData(c, linked)
}

func (h *Handler) HandleDumpLocalFilesToFile(c echo.Context) error {

	lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	filename := fmt.Sprintf("seanime-localfiles-%s.json", time.Now().Format("2006-01-02_15-04-05"))

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Response().Header().Set("Content-Type", "application/json")

	jsonData, err := json.MarshalIndent(lfs, "", "  ")
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return c.Blob(200, "application/json", jsonData)
}

// HandleImportLocalFiles
//
//	@summary imports local files from the given path.
//	@desc This will import local files from the given path.
//	@desc The response is ignored, the client should refetch the entire library collection and media entry.
//	@route /api/v1/library/local-files/import [POST]
func (h *Handler) HandleImportLocalFiles(c echo.Context) error {
	type body struct {
		DataFilePath string `json:"dataFilePath"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	contentB, err := os.ReadFile(b.DataFilePath)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	var lfs []*anime.LocalFile
	if err := json.Unmarshal(contentB, &lfs); err != nil {
		return h.RespondWithError(c, err)
	}

	if len(lfs) == 0 {
		return h.RespondWithError(c, errors.New("no local files found"))
	}

	_, err = db_bridge.InsertLocalFiles(h.App.Database, lfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	h.App.Database.TrimLocalFileEntries()

	return h.RespondWithData(c, true)
}

// HandleLocalFileBulkAction
//
//	@summary performs an action on all local files.
//	@desc This will perform the given action on all local files.
//	@desc The response is ignored, the client should refetch the entire library collection and media entry.
//	@route /api/v1/library/local-files [POST]
//	@returns []anime.LocalFile
func (h *Handler) HandleLocalFileBulkAction(c echo.Context) error {

	type body struct {
		Action string `json:"action"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	// Get all the local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	switch b.Action {
	case "lock":
		for _, lf := range lfs {
			// Note: Don't lock local files that are not associated with a media.
			// Else refreshing the library will ignore them.
			if lf.MediaId != 0 {
				lf.Locked = true
			}
		}
	case "unlock":
		for _, lf := range lfs {
			lf.Locked = false
		}
	case "unmatch-all":
		// Reset all anime-to-AniList links without modifying series data.
		// This clears MediaId and unlocks/unignores every local file to prepare for rematching.
		for _, lf := range lfs {
			lf.MediaId = 0
			lf.Locked = false
			lf.Ignored = false
			// Preserve AutoMatchBlocked, clear link source
			lf.LinkSource = ""
		}
	}

	// Save the local files
	retLfs, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, retLfs)
}

// HandleUpdateLocalFileData
//
//	@summary updates the local file with the given path.
//	@desc This will update the local file with the given path.
//	@desc The response is ignored, the client should refetch the entire library collection and media entry.
//	@route /api/v1/library/local-file [PATCH]
//	@returns []anime.LocalFile
func (h *Handler) HandleUpdateLocalFileData(c echo.Context) error {

	type body struct {
		Path     string                   `json:"path"`
		Metadata *anime.LocalFileMetadata `json:"metadata"`
		Locked   bool                     `json:"locked"`
		Ignored  bool                     `json:"ignored"`
		MediaId  int                      `json:"mediaId"`
		// Optional: set link source explicitly when updating a single file
		LinkSource       string `json:"linkSource,omitempty"`
		AutoMatchBlocked *bool  `json:"autoMatchBlocked,omitempty"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	// Get all the local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	lf, found := lo.Find(lfs, func(i *anime.LocalFile) bool {
		return i.HasSamePath(b.Path)
	})
	if !found {
		return h.RespondWithError(c, errors.New("local file not found"))
	}
	lf.Metadata = b.Metadata
	lf.Locked = b.Locked
	lf.Ignored = b.Ignored
	lf.MediaId = b.MediaId
	if b.LinkSource != "" {
		lf.LinkSource = b.LinkSource
	}
	if b.AutoMatchBlocked != nil {
		lf.AutoMatchBlocked = *b.AutoMatchBlocked
	}

	// Save the local files
	retLfs, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, retLfs)
}

// HandleUpdateLocalFiles
//
//	@summary updates local files with the given paths.
//	@desc The client should refetch the entire library collection and media entry.
//	@route /api/v1/library/local-files [PATCH]
//	@returns bool
func (h *Handler) HandleUpdateLocalFiles(c echo.Context) error {

	type body struct {
		Paths   []string `json:"paths"`
		Action  string   `json:"action"`
		MediaId int      `json:"mediaId,omitempty"`
		// Only used for unmatch action; when true and the link was automatic, block future auto-rematching
		BlockAutoRematch bool `json:"blockAutoRematch,omitempty"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	// Get all the local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Update the files
	for _, path := range b.Paths {
		lf, found := lo.Find(lfs, func(i *anime.LocalFile) bool {
			return i.HasSamePath(path)
		})
		if !found {
			continue
		}
		switch b.Action {
		case "lock":
			lf.Locked = true
		case "unlock":
			lf.Locked = false
		case "ignore":
			lf.MediaId = 0
			lf.Ignored = true
			lf.Locked = false
			lf.LinkSource = ""
		case "unignore":
			lf.Ignored = false
			lf.Locked = false
		case "unmatch":
			prevSource := lf.LinkSource
			lf.MediaId = 0
			lf.Locked = false
			lf.Ignored = false
			lf.LinkSource = ""
			if b.BlockAutoRematch && prevSource != "manual" {
				lf.AutoMatchBlocked = true
			}
		case "match":
			lf.MediaId = b.MediaId
			lf.Locked = true
			lf.Ignored = false
			// Mark as manual match and clear any block
			lf.LinkSource = "manual"
			lf.AutoMatchBlocked = false
		case "unblock-auto":
			lf.AutoMatchBlocked = false
		}
	}

	// Save the local files
	_, err = db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleDeleteLocalFiles
//
//	@summary deletes local files with the given paths.
//	@desc This will delete the local files with the given paths.
//	@desc The client should refetch the entire library collection and media entry.
//	@route /api/v1/library/local-files [DELETE]
//	@returns bool
func (h *Handler) HandleDeleteLocalFiles(c echo.Context) error {

	type body struct {
		Paths []string `json:"paths"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	// Get all the local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Delete the files
	p := pool.New().WithErrors()
	for _, path := range b.Paths {
		path := path
		p.Go(func() error {
			err := os.Remove(path)
			if err != nil {
				return err
			}
			return nil
		})
	}
	if err := p.Wait(); err != nil {
		return h.RespondWithError(c, err)
	}

	// Remove the files from the list
	lfs = lo.Filter(lfs, func(i *anime.LocalFile, _ int) bool {
		return !lo.Contains(b.Paths, i.Path)
	})

	// Save the local files
	_, err = db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleRemoveEmptyDirectories
//
//	@summary removes empty directories.
//	@desc This will remove empty directories in the library path.
//	@route /api/v1/library/empty-directories [DELETE]
//	@returns bool
func (h *Handler) HandleRemoveEmptyDirectories(c echo.Context) error {

	libraryPaths, err := h.App.Database.GetAllLibraryPathsFromSettings()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	for _, path := range libraryPaths {
		filesystem.RemoveEmptyDirectories(path, h.App.Logger)
	}

	return h.RespondWithData(c, true)
}

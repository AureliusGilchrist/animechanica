package handlers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"seanime/internal/api/anilist"
	"seanime/internal/database/db_bridge"
	"seanime/internal/hook"
	"seanime/internal/library/anime"
	"seanime/internal/library/scanner"
	"seanime/internal/library/summary"
	"seanime/internal/util"
	"seanime/internal/util/limiter"
	"seanime/internal/util/result"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"gorm.io/gorm"
)

// HandleGetAnimeEntry
//
//	@summary return a media entry for the given AniList anime media id.
//	@desc This is used by the anime media entry pages to get all the data about the anime.
//	@desc This includes episodes and metadata (if any), AniList list data, download info...
//	@route /api/v1/library/anime-entry/{id} [GET]
//	@param id - int - true - "AniList anime media ID"
//	@returns anime.Entry
func (h *Handler) HandleGetAnimeEntry(c echo.Context) error {

	mId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Get all the local files
	lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Get the host anime library files
	nakamaLfs, hydratedFromNakama := h.App.NakamaManager.GetHostAnimeLibraryFiles(mId)
	if hydratedFromNakama && nakamaLfs != nil {
		lfs = nakamaLfs
	}

	// Get the user's anilist collection
	animeCollection, err := h.App.GetAnimeCollection(false)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	if animeCollection == nil {
		return h.RespondWithError(c, errors.New("anime collection not found"))
	}

	// Create a new media entry
	entry, err := anime.NewEntry(c.Request().Context(), &anime.NewEntryOptions{
		MediaId:          mId,
		LocalFiles:       lfs,
		AnimeCollection:  animeCollection,
		Platform:         h.App.AnilistPlatform,
		MetadataProvider: h.App.MetadataProvider,
		IsSimulated:      h.App.GetUser().IsSimulated,
	})
	if err != nil {
		return h.RespondWithError(c, err)
	}

	fillerEvent := new(anime.AnimeEntryFillerHydrationEvent)
	fillerEvent.Entry = entry
	err = hook.GlobalHookManager.OnAnimeEntryFillerHydration().Trigger(fillerEvent)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	entry = fillerEvent.Entry

	if !fillerEvent.DefaultPrevented {
		h.App.FillerManager.HydrateFillerData(fillerEvent.Entry)
	}

	if hydratedFromNakama {
		entry.IsNakamaEntry = true
		for _, ep := range entry.Episodes {
			ep.IsNakamaEpisode = true
		}
	}

	return h.RespondWithData(c, entry)
}

//----------------------------------------------------------------------------------------------------------------------

// HandleCheckAnimeSeriesDirExists
//
//	@summary checks if a romaji-named series directory exists in any configured library path.
//	@desc Uses AniList to fetch the romaji title for the given media ID, sanitizes it with sanitizePathName,
//	@desc and then checks for a directory of that name under all library paths (primary and additional).
//	@route /api/v1/library/anime-entry/dir-exists/{id} [GET]
//	@param id - int - true - "AniList anime media ID"
//	@returns map[string]any  // { exists: bool, name: string }
func (h *Handler) HandleCheckAnimeSeriesDirExists(c echo.Context) error {

	// Parse media ID
	mId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Fetch anime to get romaji title
	media, err := h.App.AnilistPlatform.GetAnime(c.Request().Context(), mId)
	if err != nil || media == nil || media.Title == nil {
		return h.RespondWithError(c, fmt.Errorf("failed to fetch anime details: %w", err))
	}

	romaji := strings.TrimSpace(lo.FromPtr(media.Title.Romaji))
	english := strings.TrimSpace(lo.FromPtr(media.Title.English))
	native := strings.TrimSpace(lo.FromPtr(media.Title.Native))
	userPreferred := strings.TrimSpace(lo.FromPtr(media.Title.UserPreferred))

	// Build candidate names using the same sanitizer used when making folders
	rawCandidates := []string{
		romaji,
		english,
		userPreferred,
		native,
	}
	// Always include a fallback label to avoid empty
	if firstNonEmpty(rawCandidates...) == "" {
		rawCandidates = append(rawCandidates, fmt.Sprintf("Anime %d", mId))
	}
	// Sanitize and de-duplicate
	candidateSet := map[string]struct{}{}
	candidates := make([]string, 0, len(rawCandidates))
	for _, t := range rawCandidates {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		s := sanitizePathName(t)
		if s == "" {
			continue
		}
		if _, ok := candidateSet[s]; !ok {
			candidateSet[s] = struct{}{}
			candidates = append(candidates, s)
		}
	}
	// Keep the first candidate as the primary name for response
	sanitized := ""
	if len(candidates) > 0 {
		sanitized = candidates[0]
	}

	// Get all library paths from settings
	libPaths, err := h.App.Database.GetAllLibraryPathsFromSettings()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Also consider the theater/completed path where in-progress moves may exist
	// Keep this aligned with the destination used by HandleMoveAndRenameAnimeSeries
	const theaterCompletedBase = "/aeternae/theater/anime/completed"

	// Optional: restrict to only the theater path when explicitly requested by the client
	onlyTheater := c.QueryParam("only_theater") == "1"
	// Optional: return debug info (candidates, basesChecked)
	debug := c.QueryParam("debug") == "1"

	// Build list of bases to check (unique, non-empty)
	basesToCheck := make([]string, 0, len(libPaths)+1)
	seen := map[string]struct{}{}
	if onlyTheater {
		basesToCheck = append(basesToCheck, theaterCompletedBase)
	} else {
		for _, p := range libPaths {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				basesToCheck = append(basesToCheck, p)
			}
		}
		if strings.TrimSpace(theaterCompletedBase) != "" {
			if _, ok := seen[theaterCompletedBase]; !ok {
				seen[theaterCompletedBase] = struct{}{}
				basesToCheck = append(basesToCheck, theaterCompletedBase)
			}
		}
	}

	// Check each path for existence (case-insensitive on name for portability)
	exists := false
	for _, base := range basesToCheck {
		if strings.TrimSpace(base) == "" {
			continue
		}
		// Fast path: try direct stat for each candidate name
		for _, name := range candidates {
			p := filepath.Join(base, name)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				exists = true
				break
			}
		}
		if exists {
			break
		}
		// Fallback: scan top-level directory entries once and compare against candidates
		if entries, err := os.ReadDir(base); err == nil {
			entryNames := make(map[string]struct{}, len(entries))
			for _, de := range entries {
				if !de.IsDir() {
					continue
				}
				entryNames[de.Name()] = struct{}{}
			}
			for name := range entryNames {
				for _, cand := range candidates {
					if strings.EqualFold(name, cand) {
						exists = true
						break
					}
				}
				if exists {
					break
				}
			}
			if exists {
				break
			}
		}
	}

	if debug {
		return h.RespondWithData(c, map[string]any{
			"exists":        exists,
			"name":          sanitized,
			"candidates":    candidates,
			"basesChecked":  basesToCheck,
		})
	}
	return h.RespondWithData(c, map[string]any{
		"exists": exists,
		"name":   sanitized,
	})
}

//----------------------------------------------------------------------------------------------------------------------

// HandleRenameAnimeEntryFiles
//
//	@summary preview or execute standardized rename+move for a media's local files.
//	@desc Moves all episodes to a single root folder named after the anime and renames to
//	@desc "{ANIMENAME} - {XXX} - {EPISODETITLE}{ext}" (XXX is zero-padded to 3). Skips already-standardized files unless force=true.
//	@route /api/v1/library/anime-entry/rename-files [POST]
//	@returns []map[string]string
func (h *Handler) HandleRenameAnimeEntryFiles(c echo.Context) error {
	type body struct {
		MediaId int  `json:"mediaId"`
		DryRun  bool `json:"dryRun"`
		Force   bool `json:"force"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}
	if p.MediaId == 0 {
		return h.RespondWithError(c, errors.New("mediaId is required"))
	}

	// Fetch local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Filter to media
	files := lo.Filter(lfs, func(lf *anime.LocalFile, _ int) bool { return lf.MediaId == p.MediaId })
	if len(files) == 0 {
		return h.RespondWithError(c, errors.New("no local files for this media"))
	}

	// Resolve anime name from AniList (romaji preferred)
	media, err := h.App.AnilistPlatform.GetAnime(c.Request().Context(), p.MediaId)
	if err != nil || media == nil || media.Title == nil {
		return h.RespondWithError(c, fmt.Errorf("failed to fetch anime details: %w", err))
	}
	animeName := firstNonEmpty(lo.FromPtr(media.Title.Romaji), lo.FromPtr(media.Title.English), lo.FromPtr(media.Title.Native))
	if animeName == "" {
		animeName = fmt.Sprintf("Anime %d", p.MediaId)
	}
	cleanAnimeName := sanitizePathName(animeName)
	// Detect movies: name files without episode numbers or titles
	isMovie := media.Format != nil && lo.FromPtr(media.Format) == anilist.MediaFormatMovie

	// Compute common parent directory across all files, then target dir under it
	commonParent := commonDirOf(files)
	if commonParent == "" {
		// fallback to first file's dir
		commonParent = filepath.Dir(files[0].GetNormalizedPath())
	}
	targetDir := filepath.Join(commonParent, cleanAnimeName)

	// Build plan
	plan := make([]map[string]string, 0, len(files))
	// if movie has multiple files, we will suffix with part index deterministically
	// Create a stable order for determinism
	sorted := append([]*anime.LocalFile(nil), files...)
	slices.SortFunc(sorted, func(a, b *anime.LocalFile) int { return strings.Compare(a.GetNormalizedPath(), b.GetNormalizedPath()) })
	partIndexByPath := map[string]int{}
	if isMovie {
		for i, f := range sorted {
			partIndexByPath[f.GetNormalizedPath()] = i + 1
		}
	}
	for _, f := range files {
		from := f.GetNormalizedPath()
		ext := filepath.Ext(from)
		// Skip NCOP/NCED and other extra credit/opening files
		if isExtraEpisode(from, f.ParsedData.EpisodeTitle) {
			plan = append(plan, map[string]string{"from": from, "to": "", "reason": "filtered NCOP/NCED/extra"})
			continue
		}
		epiNum := f.Metadata.Episode
		if epiNum == 0 {
			// try parsed episode
			if n, err := strconv.Atoi(strings.TrimSpace(f.ParsedData.Episode)); err == nil {
				epiNum = n
			}
		}
		// Build destination filename
		var name string
		if isMovie {
			// Single-file movie: just the series name. Multi-file: append part index.
			name = cleanAnimeName
			if len(files) > 1 {
				idx := partIndexByPath[from]
				name = fmt.Sprintf("%s - Part %02d", cleanAnimeName, idx)
			}
		} else {
			if epiNum == 0 && !p.Force {
				plan = append(plan, map[string]string{"from": from, "to": "", "reason": "missing episode number, skipped"})
				continue
			}
			epiTitle := strings.TrimSpace(f.ParsedData.EpisodeTitle)
			name = fmt.Sprintf("%s - %03d", cleanAnimeName, epiNum)
			if epiTitle != "" {
				name += " - " + sanitizeFileName(epiTitle)
			}
		}
		to := filepath.Join(targetDir, name+ext)
		// Skip if already standardized (same dir and same basename), unless force
		if !p.Force && sameDirAndName(from, to) {
			plan = append(plan, map[string]string{"from": from, "to": to, "reason": "already standardized"})
			continue
		}
		plan = append(plan, map[string]string{"from": from, "to": to})
	}

	if p.DryRun {
		return h.RespondWithData(c, plan)
	}

	// ... (rest of the code remains the same)
	// Execute
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return h.RespondWithError(c, err)
	}

	for i, step := range plan {
		from, to := step["from"], step["to"]
		if to == "" {
			continue
		}
		// Ensure parent dir
		_ = os.MkdirAll(filepath.Dir(to), 0o755)
		finalTo := to
		// Avoid overwrite
		if fileExists(finalTo) {
			base := strings.TrimSuffix(filepath.Base(to), filepath.Ext(to))
			ext := filepath.Ext(to)
			j := 1
			for {
				candidate := filepath.Join(filepath.Dir(to), fmt.Sprintf("%s (%d)%s", base, j, ext))
				if !fileExists(candidate) {
					finalTo = candidate
					break
				}
				j++
			}
		}
		if err := os.Rename(from, finalTo); err != nil {
			plan[i]["error"] = err.Error()
			continue
		}
		// Update in-memory path
		if lf, ok := lo.Find(lfs, func(x *anime.LocalFile) bool { return x.GetNormalizedPath() == from }); ok {
			lf.Path = finalTo
		}
		plan[i]["to"] = finalTo
	}

	// Persist updated local files slice
	if _, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs); err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, plan)
}

//----------------------------------------------------------------------------------------------------------------------

// HandleListRenameCandidates
//
//	@summary lists series that likely need standardization and those manually resolved.
//	@route /api/v1/library/anime/rename-candidates [GET]
//	@returns map
func (h *Handler) HandleListRenameCandidates(c echo.Context) error {
	lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Group by MediaId
	byMedia := lo.GroupBy(lfs, func(lf *anime.LocalFile) int { return lf.MediaId })
	unresolved := make([]int, 0)
	manualSet := map[int]struct{}{}
	ignoredSet := map[int]struct{}{}

	for mid, files := range byMedia {
		if mid == 0 || len(files) == 0 {
			continue
		}
		// manually resolved heuristic: any locked file
		if lo.SomeBy(files, func(f *anime.LocalFile) bool { return f.Locked }) {
			manualSet[mid] = struct{}{}
		}
		// ignored heuristic: any file marked ignored
		if lo.SomeBy(files, func(f *anime.LocalFile) bool { return f.Ignored }) {
			ignoredSet[mid] = struct{}{}
		}
		// unresolved heuristic: files span multiple dirs or names not following pattern
		if needsStandardization(files) {
			unresolved = append(unresolved, mid)
		}
	}

	manual := make([]int, 0, len(manualSet))
	for k := range manualSet {
		manual = append(manual, k)
	}
	ignored := make([]int, 0, len(ignoredSet))
	for k := range ignoredSet {
		ignored = append(ignored, k)
	}

	res := map[string]interface{}{
		"unresolved":        unresolved,
		"manuallyResolved":  manual,
		"manuallyResolvedN": len(manual),
		"ignored":           ignored,
		"ignoredN":          len(ignored),
	}
	return h.RespondWithData(c, res)
}

// ----------------- helpers -----------------

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func fileExists(p string) bool {
	if _, err := os.Stat(p); err == nil {
		return true
	} else if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return false
}

func isSymlink(p string) bool {
	info, err := os.Lstat(p)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func removeSymlinkAndTarget(p string) {
	info, err := os.Lstat(p)
	if err != nil {
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// resolve target relative to link dir if needed
		target, err := os.Readlink(p)
		if err == nil {
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(p), target)
			}
			_ = os.Remove(p)
			_ = os.Remove(target)
			return
		}
		_ = os.Remove(p)
		return
	}
	_ = os.Remove(p)
}

func sanitizePathName(s string) string {
	s = strings.TrimSpace(s)
	// forbid path separators and control chars
	s = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' {
			return '-'
		}
		if r == 0 || r == utf8.RuneError || unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func sanitizeFileName(s string) string { return sanitizePathName(s) }

func sameDirAndName(from, to string) bool {
	return filepath.Dir(from) == filepath.Dir(to) && filepath.Base(from) == filepath.Base(to)
}

func commonDirOf(files []*anime.LocalFile) string {
	if len(files) == 0 {
		return ""
	}
	dirs := lo.Map(files, func(f *anime.LocalFile, _ int) string { return filepath.Dir(f.GetNormalizedPath()) })
	base := dirs[0]
	for _, d := range dirs[1:] {
		for !strings.HasPrefix(d, base) && base != string(filepath.Separator) {
			base = filepath.Dir(base)
		}
	}
	return base
}

func needsStandardization(files []*anime.LocalFile) bool {
	if len(files) == 0 {
		return false
	}
	// multiple dirs?
	dirs := lo.Uniq(lo.Map(files, func(f *anime.LocalFile, _ int) string { return filepath.Dir(f.GetNormalizedPath()) }))
	if len(dirs) > 1 {
		return true
	}
	// simple pattern: presence of non-standard names
	standardLike := func(name string) bool {
		name = strings.TrimSuffix(name, filepath.Ext(name))
		// contains zero-padded 3-digit pattern ' - ddd' as a proxy
		return strings.Contains(name, " - ") && has3DigitsSegment(name)
	}
	return lo.SomeBy(files, func(f *anime.LocalFile) bool { return !standardLike(filepath.Base(f.GetNormalizedPath())) })
}

func has3DigitsSegment(s string) bool {
	// very lightweight check for 3 consecutive digits
	for i := 0; i+2 < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' && s[i+1] >= '0' && s[i+1] <= '9' && s[i+2] >= '0' && s[i+2] <= '9' {
			return true
		}
	}
	return false
}

func isExtraEpisode(path string, episodeTitle string) bool {
	episodeTitle = strings.ToLower(episodeTitle)
	base := strings.ToLower(filepath.Base(path))
	// Creditless openings/endings and common extras
	if strings.Contains(episodeTitle, "ncop") || strings.Contains(episodeTitle, "nced") || strings.Contains(episodeTitle, "creditless") {
		return true
	}
	if strings.Contains(base, "ncop") || strings.Contains(base, "nced") || strings.Contains(base, "creditless") {
		return true
	}
	// Explicit extras we don't want to rename as episodes
	if strings.Contains(episodeTitle, "ncop") || strings.Contains(episodeTitle, "nced") || strings.Contains(episodeTitle, "opening") || strings.Contains(episodeTitle, "ending") {
		return true
	}
	if strings.Contains(base, "opening") || strings.Contains(base, "ending") {
		return true
	}
	return false
}

func deriveBestSuggestionTitle(files []*anime.LocalFile) string {
	if len(files) == 0 {
		return ""
	}
	// Prefer parsed title
	title := strings.TrimSpace(files[0].GetParsedTitle())
	title = stripCommonTags(title)
	if title != "" && !isGenericFolderName(title) {
		return title
	}
	// Fallback to common parent folder name
	baseDir := filepath.Base(commonDirOf(files))
	baseDir = stripCommonTags(baseDir)
	if baseDir != "" && !isGenericFolderName(baseDir) {
		return baseDir
	}
	return title
}

func stripCommonTags(s string) string {
	s = strings.TrimSpace(s)
	// Remove bracketed groups like [SubsGroup], (1080p), {BD}
	for {
		i := strings.IndexAny(s, "[{")
		j := strings.IndexAny(s, "]}")
		if i >= 0 && j > i {
			s = strings.TrimSpace(s[:i] + s[j+1:])
			continue
		}
		break
	}
	// Remove common tags and episode labels
	lowers := []string{"- episode", "episode", "- ep", " ep ", "- epsiode", "- ova", " ova ", "- special", " special ", "- complete", " complete "}
	ls := strings.ToLower(s)
	for _, tag := range lowers {
		if idx := strings.Index(ls, tag); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
			ls = strings.ToLower(s)
		}
	}
	// Trim trailing dashes
	s = strings.TrimSpace(strings.TrimSuffix(s, "-"))
	return s
}

func isGenericFolderName(s string) bool {
	ls := strings.ToLower(strings.TrimSpace(s))
	switch ls {
	case "complete", "completed", "season", "season 1", "season 2", "s1", "s2", "tv", "bd", "bluray":
		return true
	}
	return false
}

// HandleAnimeEntryBulkAction
//
//	@summary perform given action on all the local files for the given media id.
//	@desc This is used to unmatch or toggle the lock status of all the local files for a specific media entry
//	@desc The response is not used in the frontend. The client should just refetch the entire media entry data.
//	@route /api/v1/library/anime-entry/bulk-action [PATCH]
//	@returns []anime.LocalFile
func (h *Handler) HandleAnimeEntryBulkAction(c echo.Context) error {

	type body struct {
		MediaId int    `json:"mediaId"`
		Action  string `json:"action"` // "unmatch", "toggle-lock", "unlink", or "delete-files"
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	// Get all the local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Group local files by media id
	groupedLfs := anime.GroupLocalFilesByMediaID(lfs)

	selectLfs, ok := groupedLfs[p.MediaId]
	if !ok {
		return h.RespondWithError(c, errors.New("no local files found for media id"))
	}

	switch p.Action {
	case "unmatch":
		lfs = lop.Map(lfs, func(item *anime.LocalFile, _ int) *anime.LocalFile {
			if item.MediaId == p.MediaId && p.MediaId != 0 {
				// Record prior linkage for resolved-unmatched view
				item.PreviousMediaId = item.MediaId
				item.ResolvedState = "unmatched"
				item.MediaId = 0
				item.Locked = false
				item.Ignored = false
			}
			return item
		})
	case "toggle-lock":
		// Flip the locked status of all the local files for the given media
		allLocked := lo.EveryBy(selectLfs, func(item *anime.LocalFile) bool { return item.Locked })
		lfs = lop.Map(lfs, func(item *anime.LocalFile, _ int) *anime.LocalFile {
			if item.MediaId == p.MediaId && p.MediaId != 0 {
				item.Locked = !allLocked
			}
			return item
		})
	case "unlink":
		// Remove symlinked files only, and unmatch everything
		kept := make([]*anime.LocalFile, 0, len(lfs))
		for _, item := range lfs {
			if item.MediaId != p.MediaId || p.MediaId == 0 {
				kept = append(kept, item)
				continue
			}
			// If file is a symlink, remove it and drop from list
			if isSymlink(item.GetNormalizedPath()) {
				_ = os.Remove(item.GetNormalizedPath())
				continue // drop entry for deleted link
			}
			// Unmatch non-symlink files without deleting
			item.PreviousMediaId = item.MediaId
			item.ResolvedState = "unmatched"
			item.MediaId = 0
			item.Locked = false
			item.Ignored = false
			kept = append(kept, item)
		}
		lfs = kept
	case "delete-files":
		// Delete files from disk (links and regular files) and remove entries
		kept := make([]*anime.LocalFile, 0, len(lfs))
		for _, item := range lfs {
			if item.MediaId != p.MediaId || p.MediaId == 0 {
				kept = append(kept, item)
				continue
			}
			removeSymlinkAndTarget(item.GetNormalizedPath())
			// do not append: entry deleted
		}
		lfs = kept
	}

	// Save the local files
	retLfs, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, retLfs)

}

//----------------------------------------------------------------------------------------------------------------------

// HandleOpenAnimeEntryInExplorer
//
//	@summary opens the directory of a media entry in the file explorer.
//	@desc This finds a common directory for all media entry local files and opens it in the file explorer.
//	@desc Returns 'true' whether the operation was successful or not, errors are ignored.
//	@route /api/v1/library/anime-entry/open-in-explorer [POST]
//	@returns bool
func (h *Handler) HandleOpenAnimeEntryInExplorer(c echo.Context) error {

	type body struct {
		MediaId int `json:"mediaId"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	// Get all the local files
	lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	lf, found := lo.Find(lfs, func(i *anime.LocalFile) bool {
		return i.MediaId == p.MediaId
	})
	if !found {
		return h.RespondWithError(c, errors.New("local file not found"))
	}

	dir := filepath.Dir(lf.GetNormalizedPath())
	cmd := ""
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "explorer"
		wPath := strings.ReplaceAll(strings.ToLower(dir), "/", "\\")
		args = []string{wPath}
	case "darwin":
		cmd = "open"
		args = []string{dir}
	case "linux":
		cmd = "xdg-open"
		args = []string{dir}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	cmdObj := util.NewCmd(cmd, args...)
	cmdObj.Stdout = os.Stdout
	cmdObj.Stderr = os.Stderr
	_ = cmdObj.Run()

	return h.RespondWithData(c, true)

}

//----------------------------------------------------------------------------------------------------------------------

var (
	entriesSuggestionsCache = result.NewCache[string, []*anilist.BaseAnime]()
)

// HandleFetchAnimeEntrySuggestions
//
//	@summary returns a list of media suggestions for files in the given directory.
//	@desc This is used by the "Resolve unmatched media" feature to suggest media entries for the local files in the given directory.
//	@desc If some matches files are found in the directory, it will ignore them and base the suggestions on the remaining files.
//	@route /api/v1/library/anime-entry/suggestions [POST]
//	@returns []anilist.BaseAnime
func (h *Handler) HandleFetchAnimeEntrySuggestions(c echo.Context) error {

	type body struct {
		Dir string `json:"dir"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	b.Dir = strings.ToLower(b.Dir)

	suggestions, found := entriesSuggestionsCache.Get(b.Dir)
	if found {
		return h.RespondWithData(c, suggestions)
	}

	// Retrieve local files
	lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Group local files by dir
	groupedLfs := lop.GroupBy(lfs, func(item *anime.LocalFile) string {
		return filepath.Dir(item.GetNormalizedPath())
	})

	selectedLfs, found := groupedLfs[b.Dir]
	if !found {
		return h.RespondWithError(c, errors.New("no local files found for selected directory"))
	}

	// Filter out local files that are already matched
	selectedLfs = lo.Filter(selectedLfs, func(item *anime.LocalFile, _ int) bool {
		return item.MediaId == 0
	})

	// If nothing left after filtering, return empty suggestions to avoid panic
	if len(selectedLfs) == 0 {
		entriesSuggestionsCache.Set(b.Dir, []*anilist.BaseAnime{})
		return h.RespondWithData(c, []*anilist.BaseAnime{})
	}

	// Derive a robust title for suggestions
	title := deriveBestSuggestionTitle(selectedLfs)
	if strings.TrimSpace(title) == "" {
		entriesSuggestionsCache.Set(b.Dir, []*anilist.BaseAnime{})
		return h.RespondWithData(c, []*anilist.BaseAnime{})
	}

	h.App.Logger.Info().Str("title", title).Msg("handlers: Fetching anime suggestions")

	res, err := anilist.ListAnimeM(
		lo.ToPtr(1),
		&title,
		lo.ToPtr(8),
		nil,
		[]*anilist.MediaStatus{lo.ToPtr(anilist.MediaStatusFinished), lo.ToPtr(anilist.MediaStatusReleasing), lo.ToPtr(anilist.MediaStatusCancelled), lo.ToPtr(anilist.MediaStatusHiatus)},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		h.App.Logger,
		h.App.GetUserAnilistToken(),
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Cache the results
	entriesSuggestionsCache.Set(b.Dir, res.GetPage().GetMedia())

	return h.RespondWithData(c, res.GetPage().GetMedia())

}

//----------------------------------------------------------------------------------------------------------------------

// HandleAnimeEntryManualMatch
//
//	@summary matches un-matched local files in the given directory to the given media.
//	@desc It is used by the "Resolve unmatched media" feature to manually match local files to a specific media entry.
//	@desc Matching involves the use of scanner.FileHydrator. It will also lock the files.
//	@desc The response is not used in the frontend. The client should just refetch the entire library collection.
//	@route /api/v1/library/anime-entry/manual-match [POST]
//	@returns []anime.LocalFile
func (h *Handler) HandleAnimeEntryManualMatch(c echo.Context) error {

	type body struct {
		Paths   []string `json:"paths"`
		MediaId int      `json:"mediaId"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	animeCollectionWithRelations, err := h.App.AnilistPlatform.GetAnimeCollectionWithRelations(c.Request().Context())
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Retrieve local files
	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	compPaths := make(map[string]struct{})
	for _, p := range b.Paths {
		compPaths[util.NormalizePath(p)] = struct{}{}
	}

	selectedLfs := lo.Filter(lfs, func(item *anime.LocalFile, _ int) bool {
		_, found := compPaths[item.GetNormalizedPath()]
		return found && item.MediaId == 0
	})

	// Add the media id to the selected local files
	// Also, lock the files
	selectedLfs = lop.Map(selectedLfs, func(item *anime.LocalFile, _ int) *anime.LocalFile {
		item.MediaId = b.MediaId
		item.Locked = true
		item.Ignored = false
		return item
	})

	// Get the media
	media, err := h.App.AnilistPlatform.GetAnime(c.Request().Context(), b.MediaId)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Create a slice of normalized media
	normalizedMedia := []*anime.NormalizedMedia{
		anime.NewNormalizedMedia(media),
	}

	scanLogger, err := scanner.NewScanLogger(h.App.Config.Logs.Dir)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Create scan summary logger
	scanSummaryLogger := summary.NewScanSummaryLogger()

	fh := scanner.FileHydrator{
		LocalFiles:         selectedLfs,
		CompleteAnimeCache: anilist.NewCompleteAnimeCache(),
		Platform:           h.App.AnilistPlatform,
		MetadataProvider:   h.App.MetadataProvider,
		AnilistRateLimiter: limiter.NewAnilistLimiter(),
		Logger:             h.App.Logger,
		ScanLogger:         scanLogger,
		ScanSummaryLogger:  scanSummaryLogger,
		AllMedia:           normalizedMedia,
		ForceMediaId:       media.GetID(),
	}

	fh.HydrateMetadata()

	// Hydrate the summary logger before merging files
	fh.ScanSummaryLogger.HydrateData(selectedLfs, normalizedMedia, animeCollectionWithRelations)

	// Save the scan summary
	go func() {
		err = db_bridge.InsertScanSummary(h.App.Database, scanSummaryLogger.GenerateSummary())
	}()

	// Remove select local files from the database slice, we will add them (hydrated) later
	selectedPaths := lop.Map(selectedLfs, func(item *anime.LocalFile, _ int) string { return item.GetNormalizedPath() })
	lfs = lo.Filter(lfs, func(item *anime.LocalFile, _ int) bool {
		if slices.Contains(selectedPaths, item.GetNormalizedPath()) {
			return false
		}
		return true
	})

	// Event
	event := new(anime.AnimeEntryManualMatchBeforeSaveEvent)
	event.MediaId = b.MediaId
	event.Paths = b.Paths
	event.MatchedLocalFiles = selectedLfs
	err = hook.GlobalHookManager.OnAnimeEntryManualMatchBeforeSave().Trigger(event)
	if err != nil {
		return h.RespondWithError(c, fmt.Errorf("OnAnimeEntryManualMatchBeforeSave: %w", err))
	}

	// Default prevented, do not save the local files
	if event.DefaultPrevented {
		return h.RespondWithData(c, lfs)
	}

	// Add the hydrated local files to the slice
	lfs = append(lfs, event.MatchedLocalFiles...)

	// Update the local files
	retLfs, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, retLfs)
}

//----------------------------------------------------------------------------------------------------------------------

//var missingEpisodesMap = result.NewResultMap[string, *anime.MissingEpisodes]()

// HandleGetMissingEpisodes
//
//	@summary returns a list of episodes missing from the user's library collection
//	@desc It detects missing episodes by comparing the user's AniList collection 'next airing' data with the local files.
//	@desc This route can be called multiple times, as it does not bypass the cache.
//	@route /api/v1/library/missing-episodes [GET]
//	@returns anime.MissingEpisodes
func (h *Handler) HandleGetMissingEpisodes(c echo.Context) error {

	// Get the user's anilist collection
	// Do not bypass the cache, since this handler might be called multiple times, and we don't want to spam the API
	// A cron job will refresh the cache every 10 minutes
	animeCollection, err := h.App.GetAnimeCollection(false)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	lfs, _, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Get the silenced media ids
	silencedMediaIds, _ := h.App.Database.GetSilencedMediaEntryIds()

	missingEps := anime.NewMissingEpisodes(&anime.NewMissingEpisodesOptions{
		AnimeCollection:  animeCollection,
		LocalFiles:       lfs,
		SilencedMediaIds: silencedMediaIds,
		MetadataProvider: h.App.MetadataProvider,
	})

	event := new(anime.MissingEpisodesEvent)
	event.MissingEpisodes = missingEps
	err = hook.GlobalHookManager.OnMissingEpisodes().Trigger(event)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, event.MissingEpisodes)
}

//----------------------------------------------------------------------------------------------------------------------

// HandleGetAnimeEntrySilenceStatus
//
//	@summary returns the silence status of a media entry.
//	@param id - int - true - "The ID of the media entry."
//	@route /api/v1/library/anime-entry/silence/{id} [GET]
//	@returns models.SilencedMediaEntry
func (h *Handler) HandleGetAnimeEntrySilenceStatus(c echo.Context) error {
	mId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return h.RespondWithError(c, errors.New("invalid id"))
	}

	animeEntry, err := h.App.Database.GetSilencedMediaEntry(uint(mId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return h.RespondWithData(c, false)
		} else {
			return h.RespondWithError(c, err)
		}
	}

	return h.RespondWithData(c, animeEntry)
}

// HandleToggleAnimeEntrySilenceStatus
//
//	@summary toggles the silence status of a media entry.
//	@desc The missing episodes should be re-fetched after this.
//	@route /api/v1/library/anime-entry/silence [POST]
//	@returns bool
func (h *Handler) HandleToggleAnimeEntrySilenceStatus(c echo.Context) error {

	type body struct {
		MediaId int `json:"mediaId"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	animeEntry, err := h.App.Database.GetSilencedMediaEntry(uint(b.MediaId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = h.App.Database.InsertSilencedMediaEntry(uint(b.MediaId))
			if err != nil {
				return h.RespondWithError(c, err)
			}
			return h.RespondWithData(c, true)
		} else {
			return h.RespondWithError(c, err)
		}
	}

	err = h.App.Database.DeleteSilencedMediaEntry(animeEntry.ID)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleHideAnimeEntry
//
//	@summary hides a media entry (all its local files) from resolved views.
//	@route /api/v1/library/anime-entry/hide [POST]
//	@returns bool
func (h *Handler) HandleHideAnimeEntry(c echo.Context) error {

	type body struct {
		MediaId int `json:"mediaId"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}
	if b.MediaId == 0 {
		return h.RespondWithError(c, errors.New("mediaId is required"))
	}

	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Set Hidden=true for all local files of this media
	for _, lf := range lfs {
		if lf.MediaId == b.MediaId {
			lf.Hidden = true
		}
	}

	if _, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs); err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleUnhideAnimeEntry
//
//	@summary unhides a media entry (all its local files) so it appears in resolved views again.
//	@route /api/v1/library/anime-entry/unhide [POST]
//	@returns bool
func (h *Handler) HandleUnhideAnimeEntry(c echo.Context) error {

	type body struct {
		MediaId int `json:"mediaId"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}
	if b.MediaId == 0 {
		return h.RespondWithError(c, errors.New("mediaId is required"))
	}

	lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Set Hidden=false for all local files of this media
	for _, lf := range lfs {
		if lf.MediaId == b.MediaId {
			lf.Hidden = false
		}
	}

	if _, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs); err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleUpdateAnimeEntryProgress
//
//	@summary update the progress of the given anime media entry.
//	@desc This is used to update the progress of the given anime media entry on AniList.
//	@desc The response is not used in the frontend, the client should just refetch the entire media entry data.
//	@desc NOTE: This is currently only used by the 'Online streaming' feature since anime progress updates are handled by the Playback Manager.
//	@route /api/v1/library/anime-entry/update-progress [POST]
//	@returns bool
func (h *Handler) HandleUpdateAnimeEntryProgress(c echo.Context) error {

	type body struct {
		MediaId       int `json:"mediaId"`
		MalId         int `json:"malId,omitempty"`
		EpisodeNumber int `json:"episodeNumber"`
		TotalEpisodes int `json:"totalEpisodes"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	// Update the progress on AniList
	err := h.App.AnilistPlatform.UpdateEntryProgress(
		c.Request().Context(),
		b.MediaId,
		b.EpisodeNumber,
		&b.TotalEpisodes,
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	_, _ = h.App.RefreshAnimeCollection() // Refresh the AniList collection

	return h.RespondWithData(c, true)
}

//-----------------------------------------------------------------------------------------------------------------------------

// HandleUpdateAnimeEntryRepeat
//
//	@summary update the repeat value of the given anime media entry.
//	@desc This is used to update the repeat value of the given anime media entry on AniList.
//	@desc The response is not used in the frontend, the client should just refetch the entire media entry data.
//	@route /api/v1/library/anime-entry/update-repeat [POST]
//	@returns bool
func (h *Handler) HandleUpdateAnimeEntryRepeat(c echo.Context) error {

	type body struct {
		MediaId int `json:"mediaId"`
		Repeat  int `json:"repeat"`
	}

	b := new(body)
	if err := c.Bind(b); err != nil {
		return h.RespondWithError(c, err)
	}

	err := h.App.AnilistPlatform.UpdateEntryRepeat(
		c.Request().Context(),
		b.MediaId,
		b.Repeat,
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	//_, _ = h.App.RefreshAnimeCollection() // Refresh the AniList collection

	return h.RespondWithData(c, true)
}

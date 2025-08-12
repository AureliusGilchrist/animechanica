package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	hibiketorrent "seanime/internal/extension/hibike/torrent"
	"seanime/internal/torrent_clients/torrent_client"
	"seanime/internal/torrents/torrent"

	"github.com/labstack/echo/v4"
)

// note: provider-level rate limiting is applied in torrent repository

// Temporary structures for anime batch downloader until full integration
type AnimeOfflineEntry struct {
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Episodes int      `json:"episodes"`
	Status   string   `json:"status"`
	Synonyms []string `json:"synonyms"`
	Year     int      `json:"year"`
	Tags     []string `json:"tags"`
}

// snapshotJob creates a safe, read-only copy of the job for JSON responses.
// It copies slices/maps to avoid concurrent mutation during marshal.
func snapshotJob(src *BatchDownloadJob) *BatchDownloadJob {
	if src == nil {
		return nil
	}
	dst := &BatchDownloadJob{}
	src.mu.RLock()
	defer src.mu.RUnlock()

	// Shallow copy simple fields
	dst.ID = src.ID
	dst.Status = src.Status
	dst.Progress = src.Progress
	dst.TotalAnime = src.TotalAnime
	dst.CompletedAnime = src.CompletedAnime
	dst.FailedAnime = src.FailedAnime
	dst.ActiveBatches = src.ActiveBatches
	dst.StartTime = src.StartTime
	if src.EndTime != nil {
		t := *src.EndTime
		dst.EndTime = &t
	}
	dst.Settings = src.Settings

	// Copy Errors slice
	if len(src.Errors) > 0 {
		dst.Errors = make([]string, len(src.Errors))
		copy(dst.Errors, src.Errors)
	}

	// Copy Logs slice
	if len(src.Logs) > 0 {
		dst.Logs = make([]DownloadLogEntry, len(src.Logs))
		copy(dst.Logs, src.Logs)
	}

	// Copy CurrentAnime
	if src.CurrentAnime != nil {
		ca := *src.CurrentAnime
		dst.CurrentAnime = &ca
	}

	// Copy Statistics
	if src.Statistics != nil {
		st := *src.Statistics
		dst.Statistics = &st
	}

	return dst
}

// DownloadLogEntry captures per-anime attempts and failures
type DownloadLogEntry struct {
	AnimeTitle string    `json:"animeTitle"`
	Query      string    `json:"query"`
	Status     string    `json:"status"` // failed|success|info
	Message    string    `json:"message"`
	Time       time.Time `json:"time"`
}

type AnimeOfflineDatabase struct {
	Data []AnimeOfflineEntry `json:"data"`
}

type AllAnimeDownloadSettings struct {
	PreferDualAudio      bool     `json:"preferDualAudio"`
	PreferBluray         bool     `json:"preferBluray"`
	PreferHighestRes     bool     `json:"preferHighestRes"`
	MinSeeders           int      `json:"minSeeders"`
	MaxConcurrentBatches int      `json:"maxConcurrentBatches"`
	SkipOva              bool     `json:"skipOva"`
	SkipSpecials         bool     `json:"skipSpecials"`
	MinYear              int      `json:"minYear"`
	MaxYear              int      `json:"maxYear"`
	IncludeGenres        []string `json:"includeGenres"`
	ExcludeGenres        []string `json:"excludeGenres"`
}

// BatchDownloadJob represents a batch download operation
type BatchDownloadJob struct {
	ID             string                   `json:"id"`
	Status         string                   `json:"status"`
	Progress       float64                  `json:"progress"`
	TotalAnime     int                      `json:"totalAnime"`
	CompletedAnime int                      `json:"completedAnime"`
	FailedAnime    int                      `json:"failedAnime"`
	ActiveBatches  int                      `json:"activeBatches"`
	StartTime      time.Time                `json:"startTime"`
	EndTime        *time.Time               `json:"endTime"`
	CurrentAnime   *AnimeOfflineEntry       `json:"currentAnime"`
	Errors         []string                 `json:"errors"`
	Logs           []DownloadLogEntry       `json:"logs"`
	Settings       AllAnimeDownloadSettings `json:"settings"`
	Statistics     *BatchDownloadStatistics `json:"statistics"`
	mu             sync.RWMutex
}

// BatchDownloadStatistics contains download statistics
type BatchDownloadStatistics struct {
	TotalSizeGB       float64 `json:"totalSizeGb"`
	DownloadedSizeGB  float64 `json:"downloadedSizeGb"`
	AverageSpeed      int64   `json:"averageSpeed"`
	EstimatedTimeLeft string  `json:"estimatedTimeLeft"`
	DualAudioCount    int     `json:"dualAudioCount"`
	BlurayCount       int     `json:"blurayCount"`
	HighResCount      int     `json:"highResCount"`
	TorrentsAdded     int     `json:"torrentsAdded"`
	QbittorrentActive int     `json:"qbittorrentActive"`
}

// Global variables for job management (in production, this would be in a proper job manager)
var (
	activeJob   *BatchDownloadJob
	activeJobMu sync.RWMutex
)

// Helper functions for batch download management
func (h *Handler) setActiveJob(job *BatchDownloadJob) {
	activeJobMu.Lock()
	defer activeJobMu.Unlock()
	activeJob = job
}

func (h *Handler) getActiveJob() *BatchDownloadJob {
	activeJobMu.RLock()
	defer activeJobMu.RUnlock()
	return activeJob
}

func (h *Handler) clearActiveJob() {
	activeJobMu.Lock()
	defer activeJobMu.Unlock()
	activeJob = nil
}

// loadAnimeDatabase loads the anime database from file
func (h *Handler) loadAnimeDatabase(databasePath string) ([]AnimeOfflineEntry, error) {
	file, err := os.Open(databasePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var database AnimeOfflineDatabase
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&database); err != nil {
		return nil, err
	}

	return database.Data, nil
}

// filterAnimeForDownload filters anime based on settings
func (h *Handler) filterAnimeForDownload(animeList []AnimeOfflineEntry, settings AllAnimeDownloadSettings) []AnimeOfflineEntry {
	var filtered []AnimeOfflineEntry

	// Normalize year range
	minYear := settings.MinYear
	maxYear := settings.MaxYear
	if minYear > 0 && maxYear > 0 && minYear > maxYear {
		// Swap to be safe
		minYear, maxYear = maxYear, minYear
	}

	for _, anime := range animeList {
		// Skip OVA if requested
		if settings.SkipOva && anime.Type == "OVA" {
			continue
		}

		// Skip specials if requested
		if settings.SkipSpecials && anime.Type == "SPECIAL" {
			continue
		}

		// Year filter (allow unknown year 0 to pass through)
		if anime.Year != 0 {
			if minYear > 0 && anime.Year < minYear {
				continue
			}
			if maxYear > 0 && anime.Year > maxYear {
				continue
			}
		}

		// Genre filters (case-insensitive)
		// Include: if specified, require at least one match
		if len(settings.IncludeGenres) > 0 {
			hasIncluded := false
			for _, tag := range anime.Tags {
				for _, include := range settings.IncludeGenres {
					if strings.EqualFold(tag, include) {
						hasIncluded = true
						break
					}
				}
				if hasIncluded {
					break
				}
			}
			if !hasIncluded {
				continue
			}
		}

		// Exclude: if any tag matches excluded list, skip
		if len(settings.ExcludeGenres) > 0 {
			skip := false
			for _, excludeGenre := range settings.ExcludeGenres {
				for _, tag := range anime.Tags {
					if strings.EqualFold(tag, excludeGenre) {
						skip = true
						break
					}
				}
				if skip {
					break
				}
			}
			if skip {
				continue
			}
		}

		filtered = append(filtered, anime)
	}

	return filtered
}

// processBatchDownload processes the batch download in background
func (h *Handler) processBatchDownload(job *BatchDownloadJob, animeList []AnimeOfflineEntry) {
	defer func() {
		job.mu.Lock()
		if job.Status == "running" {
			job.Status = "completed"
		}
		job.EndTime = &[]time.Time{time.Now()}[0]
		job.mu.Unlock()

		// Optional success log
		job.mu.Lock()
		job.Logs = append(job.Logs, DownloadLogEntry{
			AnimeTitle: "Batch",
			Query:      "",
			Status:     "success",
			Message:    "Batch download completed",
			Time:       time.Now(),
		})
		job.mu.Unlock()
		h.emitJobUpdate(job)
		// Clear the active job reference once finished or cancelled
		h.clearActiveJob()
	}()

	batchSize := job.Settings.MaxConcurrentBatches
	if batchSize <= 0 {
		batchSize = 3 // Default batch size
	}

	for i := 0; i < len(animeList); i += batchSize {
		// Check if job was cancelled
		job.mu.RLock()
		if job.Status == "cancelled" {
			job.mu.RUnlock()
			return
		}
		job.mu.RUnlock()

		// Process batch
		end := i + batchSize
		if end > len(animeList) {
			end = len(animeList)
		}

		batch := animeList[i:end]
		h.processBatch(job, batch)

		// Update progress
		job.mu.Lock()
		job.Progress = float64(i+len(batch)) / float64(len(animeList)) * 100
		job.mu.Unlock()

		h.emitJobUpdate(job)

		// Small delay between batches
		time.Sleep(2 * time.Second)
	}
}

// processBatch processes a batch of anime
func (h *Handler) processBatch(job *BatchDownloadJob, batch []AnimeOfflineEntry) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, job.Settings.MaxConcurrentBatches)

	for _, anime := range batch {
		wg.Add(1)
		go func(a AnimeOfflineEntry) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			h.processAnime(job, a)
		}(anime)
	}

	wg.Wait()
}

// processAnime processes a single anime
func (h *Handler) processAnime(job *BatchDownloadJob, anime AnimeOfflineEntry) {
	job.mu.Lock()
	job.CurrentAnime = &anime
	job.ActiveBatches++
	job.mu.Unlock()

	defer func() {
		job.mu.Lock()
		job.ActiveBatches--
		job.mu.Unlock()
	}()

	// Check cancellation early
	job.mu.RLock()
	if job.Status == "cancelled" {
		job.mu.RUnlock()
		return
	}
	job.mu.RUnlock()

	// Ensure torrent client is running
	if ok := h.App.TorrentClientRepository.Start(); !ok {
		job.mu.Lock()
		job.FailedAnime++
		job.Errors = append(job.Errors, fmt.Sprintf("Torrent client not available for: %s", anime.Title))
		job.Logs = append(job.Logs, DownloadLogEntry{
			AnimeTitle: anime.Title,
			Query:      "",
			Status:     "failed",
			Message:    "Torrent client not available",
			Time:       time.Now(),
		})
		job.mu.Unlock()
		return
	}

	// Build destination folder under anime download dir
	// The torrent client will create subdirectories based on the torrent content
	destRoot := h.App.Config.Anime.DownloadDir
	_ = os.MkdirAll(destRoot, 0o755)

	// Use the root anime download directory as destination
	// The torrent client will extract files into this directory
	dest := destRoot

	// Construct search query and media for AnimeTosho direct provider search
	parts := []string{anime.Title, "batch"}
	if job.Settings.PreferBluray {
		parts = append(parts, "bluray", "bd")
	}
	if job.Settings.PreferDualAudio {
		parts = append(parts, "dual audio")
	}
	query := strings.Join(parts, " ")

	// Prepare media with cleaned synonyms
	cleanedSyns := cleanSynonyms(anime.Synonyms, anime.Title)
	media := hibiketorrent.Media{
		RomajiTitle:  anime.Title,
		EnglishTitle: nil,
		EpisodeCount: anime.Episodes,
		Format:       strings.ToLower(anime.Type), // best-effort mapping
		Synonyms:     cleanedSyns,
	}

	// Build and log OR-name array (base title + cleaned synonyms), deduped case-insensitively
	buildOrNames := func(base string, syns []string) []string {
		merged := make([]string, 0, 1+len(syns))
		merged = append(merged, base)
		merged = append(merged, syns...)
		seen := make(map[string]struct{}, len(merged))
		out := make([]string, 0, len(merged))
		for _, s := range merged {
			v := strings.TrimSpace(s)
			if v == "" {
				continue
			}
			l := strings.ToLower(v)
			if _, ok := seen[l]; ok {
				continue
			}
			seen[l] = struct{}{}
			out = append(out, v)
		}
		return out
	}
	orNames := buildOrNames(anime.Title, cleanedSyns)
	if b, err := json.Marshal(orNames); err == nil {
		job.mu.Lock()
		job.Logs = append(job.Logs, DownloadLogEntry{
			AnimeTitle: anime.Title,
			Query:      query,
			Status:     "info",
			Message:    fmt.Sprintf("OR names: %s", string(b)),
			Time:       time.Now(),
		})
		job.mu.Unlock()
		h.emitJobUpdate(job)
	}

	// Get AnimeTosho provider from torrent repository
	providerExt, ok := h.App.TorrentRepository.GetAnimeProviderExtension(torrent.ProviderAnimeTosho)
	if !ok {
		job.mu.Lock()
		job.FailedAnime++
		job.Errors = append(job.Errors, fmt.Sprintf("AnimeTosho provider not available for: %s", anime.Title))
		job.Logs = append(job.Logs, DownloadLogEntry{
			AnimeTitle: anime.Title,
			Query:      query,
			Status:     "failed",
			Message:    "provider not available",
			Time:       time.Now(),
		})
		job.mu.Unlock()
		h.emitJobUpdate(job)
		return
	}

	// Prefer batch smart search first with rate limiting and retries
	var torrents []*hibiketorrent.AnimeTorrent
	var err error
	// For movies/single-episode, do not force batch searches
	wantBatch := anime.Episodes > 1 && !strings.EqualFold(anime.Type, "movie")
	for attempt := 0; attempt < 3; attempt++ {
		torrents, err = providerExt.GetProvider().SmartSearch(hibiketorrent.AnimeSmartSearchOptions{
			Media:      media,
			Query:      query,
			Batch:      wantBatch,
			Resolution: "",
		})
		if err == nil && len(torrents) > 0 {
			break
		}
		// Exponential backoff between attempts
		time.Sleep(time.Duration(400*(attempt+1)) * time.Millisecond)
	}
	if len(torrents) == 0 {
		// Fallback to simple search with pacing and retries
		for attempt := 0; attempt < 2; attempt++ {
			var res []*hibiketorrent.AnimeTorrent
			res, err = providerExt.GetProvider().Search(hibiketorrent.AnimeSearchOptions{Media: media, Query: query})
			if err == nil && len(res) > 0 {
				torrents = res
				break
			}
			time.Sleep(time.Duration(400*(attempt+1)) * time.Millisecond)
		}
	}

	if len(torrents) == 0 {
		// Provide a clear reason instead of showing <nil>
		reason := "no results"
		if err != nil {
			reason = err.Error()
		}

		job.mu.Lock()
		job.FailedAnime++
		job.Errors = append(job.Errors, fmt.Sprintf("No torrents found on AnimeTosho for: %s (%s)", anime.Title, reason))
		job.Logs = append(job.Logs, DownloadLogEntry{
			AnimeTitle: anime.Title,
			Query:      query,
			Status:     "failed",
			Message:    fmt.Sprintf("search failed: %s", reason),
			Time:       time.Now(),
		})
		job.mu.Unlock()
		h.emitJobUpdate(job)
		return
	}

	// Select best torrent using biased scoring
	best, candidates := selectBestAnimeToshoTorrent(torrents, job.Settings.MinSeeders)
	if best == nil {
		// Build detailed message showing top 5 rejected candidates
		var candidateDetails []string
		rejectedCount := 0
		for _, c := range candidates {
			if c.Reason != "qualified" {
				rejectedCount++
			}
		}

		// Sort candidates by score descending, then by seeders
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].Score != candidates[j].Score {
				return candidates[i].Score > candidates[j].Score
			}
			return candidates[i].Torrent.Seeders > candidates[j].Torrent.Seeders
		})

		// Show top 5 candidates with details
		for i, c := range candidates {
			if i >= 5 {
				break
			}
			features := []string{}
			if c.HasBatch {
				features = append(features, "Batch")
			}
			if c.HasDual {
				features = append(features, "Dual")
			}
			if c.HasRes {
				features = append(features, "HD")
			}
			if c.HasBD {
				features = append(features, "BD")
			}

			featureStr := strings.Join(features, "+")
			if featureStr == "" {
				featureStr = "None"
			}

			candidateDetails = append(candidateDetails, fmt.Sprintf(
				"%s (%.1f%%, %d seeds, %s) - %s",
				c.Torrent.Name,
				c.MatchPct,
				c.Torrent.Seeders,
				featureStr,
				c.Reason,
			))
		}

		message := fmt.Sprintf("no candidate met threshold (%d rejected)", rejectedCount)
		if len(candidateDetails) > 0 {
			message += ":\n" + strings.Join(candidateDetails, "\n")
		}

		job.mu.Lock()
		job.FailedAnime++
		job.Errors = append(job.Errors, fmt.Sprintf("No suitable torrent candidate met threshold for: %s (%d candidates)", anime.Title, len(candidates)))
		job.Logs = append(job.Logs, DownloadLogEntry{
			AnimeTitle: anime.Title,
			Query:      query,
			Status:     "failed",
			Message:    message,
			Time:       time.Now(),
		})
		job.mu.Unlock()
		h.emitJobUpdate(job)
		return
	}

	magnet := best.MagnetLink
	if magnet == "" {
		// Try provider method if not set
		if m, e := providerExt.GetProvider().GetTorrentMagnetLink(best); e == nil {
			magnet = m
		}
	}
	if magnet == "" {
		job.mu.Lock()
		job.FailedAnime++
		job.Errors = append(job.Errors, fmt.Sprintf("Failed to resolve magnet for: %s", anime.Title))
		job.Logs = append(job.Logs, DownloadLogEntry{
			AnimeTitle: anime.Title,
			Query:      query,
			Status:     "failed",
			Message:    "empty magnet link",
			Time:       time.Now(),
		})
		job.mu.Unlock()
		h.emitJobUpdate(job)
		return
	}

	if err := h.App.TorrentClientRepository.AddMagnets([]string{magnet}, dest); err != nil {
		job.mu.Lock()
		job.FailedAnime++
		job.Errors = append(job.Errors, fmt.Sprintf("Failed to add magnet for: %s (%v)", anime.Title, err))
		job.Logs = append(job.Logs, DownloadLogEntry{
			AnimeTitle: anime.Title,
			Query:      query,
			Status:     "failed",
			Message:    fmt.Sprintf("add magnet failed: %v", err),
			Time:       time.Now(),
		})
		job.mu.Unlock()
		h.emitJobUpdate(job)
		return
	}

	// Success log after adding magnet
	job.mu.Lock()
	job.Logs = append(job.Logs, DownloadLogEntry{
		AnimeTitle: anime.Title,
		Query:      query,
		Status:     "success",
		Message:    "magnet added to client",
		Time:       time.Now(),
	})
	job.mu.Unlock()
	h.emitJobUpdate(job)

	// Success: update stats
	job.mu.Lock()
	job.CompletedAnime++
	if job.Statistics == nil {
		job.Statistics = &BatchDownloadStatistics{}
	}
	job.Statistics.TorrentsAdded++
	if job.Settings.PreferDualAudio {
		job.Statistics.DualAudioCount++
	}
	if job.Settings.PreferBluray {
		job.Statistics.BlurayCount++
	}
	if job.Settings.PreferHighestRes {
		job.Statistics.HighResCount++
	}
	job.mu.Unlock()

	// Refresh active torrent counts
	var ac torrent_client.ActiveCount
	h.App.TorrentClientRepository.GetActiveCount(&ac)
	job.mu.Lock()
	if job.Statistics == nil {
		job.Statistics = &BatchDownloadStatistics{}
	}
	job.Statistics.QbittorrentActive = ac.Downloading + ac.Seeding + ac.Paused
	job.mu.Unlock()
}

// TorrentCandidate holds information about a torrent candidate and why it was rejected
type TorrentCandidate struct {
	Torrent  *hibiketorrent.AnimeTorrent
	Score    int
	MatchPct float64
	Reason   string
	HasBatch bool
	HasDual  bool
	HasRes   bool
	HasBD    bool
}

// selectBestAnimeToshoTorrent applies biased scoring over AnimeTosho results.
// Bias: Batch > Dual Audio > Resolution > BD. A candidate must meet minSeeders and >=50% match across features,
// unless it is batch (always allowed). Tie-breakers: resolution, seeders, size.
func selectBestAnimeToshoTorrent(ts []*hibiketorrent.AnimeTorrent, minSeeders int) (*hibiketorrent.AnimeTorrent, []TorrentCandidate) {
	bestIdx := -1
	bestScore := -1
	var bestRes int
	var bestSeed int
	var bestSize int64
	var candidates []TorrentCandidate

	for i, t := range ts {
		if t == nil {
			continue
		}

		title := t.Name
		hasBatch := t.IsBatch || t.IsBestRelease || isBatchTitleLite(title)
		dual := dualAudioScoreLite(title) > 0
		res := resolutionScoreLite(title)
		bd := blurayScoreLite(title)

		matches := 0
		if hasBatch {
			matches++
		}
		if dual {
			matches++
		}
		if res > 0 {
			matches++
		}
		if bd > 0 {
			matches++
		}

		matchPct := float64(matches) / 4.0 * 100

		candidate := TorrentCandidate{
			Torrent:  t,
			MatchPct: matchPct,
			HasBatch: hasBatch,
			HasDual:  dual,
			HasRes:   res > 0,
			HasBD:    bd > 0,
		}

		if t.Seeders < minSeeders {
			candidate.Reason = fmt.Sprintf("too few seeders (%d < %d)", t.Seeders, minSeeders)
			candidates = append(candidates, candidate)
			continue
		}

		if !hasBatch && matchPct < 50.0 {
			candidate.Reason = fmt.Sprintf("quality too low (%.1f%% < 50%%)", matchPct)
			candidates = append(candidates, candidate)
			continue
		}

		score := 0
		if hasBatch {
			score += 100
		}
		if dual {
			score += 50
		}
		if res > 0 {
			score += 20 + res
		}
		if bd > 0 {
			score += 10 + bd
		}

		candidate.Score = score
		candidate.Reason = "qualified"
		candidates = append(candidates, candidate)

		size := int64(t.Size)
		if score > bestScore ||
			(score == bestScore && res > bestRes) ||
			(score == bestScore && res == bestRes && t.Seeders > bestSeed) ||
			(score == bestScore && res == bestRes && t.Seeders == bestSeed && size > bestSize) {
			bestScore = score
			bestIdx = i
			bestRes = res
			bestSeed = t.Seeders
			bestSize = size
		}
	}

	if bestIdx < 0 {
		return nil, candidates
	}
	return ts[bestIdx], candidates
}

// Lightweight helpers (duplicated from torrent_client scoring logic)
func isBatchTitleLite(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	if t == "" {
		return false
	}
	if strings.Contains(t, "complete") || strings.Contains(t, "batch") || strings.Contains(t, "全集") || strings.Contains(t, "全巻") {
		return true
	}
	// simple range check like 1-12
	if strings.Contains(t, "-") && (strings.Contains(t, "1-") || strings.Contains(t, "01-")) {
		return true
	}
	// Season tag like S1
	if strings.Contains(t, " s1") || strings.Contains(t, " s2") || strings.Contains(t, " s3") {
		return true
	}
	// Heuristic: if provider flagged batch
	return false
}

func resolutionScoreLite(title string) int {
	t := strings.ToLower(title)
	if strings.Contains(t, "2160p") || strings.Contains(t, "4k") {
		return 4
	}
	if strings.Contains(t, "1080p") {
		return 3
	}
	if strings.Contains(t, "720p") {
		return 2
	}
	if strings.Contains(t, "480p") || strings.Contains(t, "576p") {
		return 1
	}
	return 0
}

func dualAudioScoreLite(title string) int {
	t := strings.ToLower(title)
	if strings.Contains(t, "dual audio") || strings.Contains(t, "dual-audio") || strings.Contains(t, "dual") {
		return 1
	}
	return 0
}

func blurayScoreLite(title string) int {
	t := strings.ToLower(title)
	if strings.Contains(t, "bluray") || strings.Contains(t, "bd") || strings.Contains(t, "bdrip") {
		return 1
	}
	return 0
}

// cleanSynonyms trims, deduplicates, removes empties and title duplicates, and limits list length
func cleanSynonyms(in []string, title string) []string {
	// Generate variations including symbol-removed versions and collapse spaces.
	// Cap to avoid overly long provider queries.
	const capMax = 12

	seen := make(map[string]struct{}, len(in)+4)
	out := make([]string, 0, len(in)+4)

	norm := func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return s
		}
		// collapse whitespace
		s = strings.Join(strings.Fields(s), " ")
		return s
	}

	stripSymbols := func(s string) string {
		if s == "" {
			return s
		}
		var b strings.Builder
		b.Grow(len(s))
		prevSpace := false
		for _, r := range s {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				b.WriteRune(r)
				prevSpace = false
				continue
			}
			// Treat all other runes as space separators
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		}
		return norm(b.String())
	}

	// remove only apostrophes to produce possessive-joined variants: JoJo's -> JoJos
	removeApostrophes := func(s string) string {
		if s == "" {
			return s
		}
		s = strings.ReplaceAll(s, "'", "")
		s = strings.ReplaceAll(s, "’", "")
		return norm(s)
	}

	base := norm(title)
	baseLower := strings.ToLower(base)
	// Include stripped variant of base title if distinct
	baseStripped := stripSymbols(base)
	if baseStripped != "" {
		l := strings.ToLower(baseStripped)
		if l != baseLower {
			if _, ok := seen[l]; !ok {
				seen[l] = struct{}{}
				out = append(out, baseStripped)
			}
		}
	}

	// Include apostrophe-removed variant of base title if distinct
	baseApos := removeApostrophes(base)
	if baseApos != "" {
		l := strings.ToLower(baseApos)
		if l != baseLower {
			if _, ok := seen[l]; !ok {
				seen[l] = struct{}{}
				out = append(out, baseApos)
			}
		}
	}

	// Process input synonyms with their stripped variants
	for _, s := range in {
		if len(out) >= capMax {
			break
		}
		v := norm(s)
		if v == "" {
			continue
		}
		lv := strings.ToLower(v)
		if lv == baseLower {
			continue
		}
		if _, ok := seen[lv]; !ok {
			seen[lv] = struct{}{}
			out = append(out, v)
		}
		if len(out) >= capMax {
			break
		}
		sv := stripSymbols(v)
		if sv != "" {
			lsv := strings.ToLower(sv)
			if lsv != baseLower {
				if _, ok := seen[lsv]; !ok {
					seen[lsv] = struct{}{}
					out = append(out, sv)
				}
			}
		}
		if len(out) >= capMax {
			break
		}
		av := removeApostrophes(v)
		if av != "" {
			lav := strings.ToLower(av)
			if lav != baseLower {
				if _, ok := seen[lav]; !ok {
					seen[lav] = struct{}{}
					out = append(out, av)
				}
			}
		}
	}

	// Ensure cap
	if len(out) > capMax {
		out = out[:capMax]
	}
	return out
}

// emitJobUpdate emits a job update event (mock implementation)
func (h *Handler) emitJobUpdate(job *BatchDownloadJob) {
	// In a real implementation, this would emit WebSocket events
	// For now, we'll just log the progress
	if job.Progress > 0 {
		fmt.Printf("Batch Download Progress: %.1f%% (%d/%d completed, %d failed)\n",
			job.Progress, job.CompletedAnime, job.TotalAnime, job.FailedAnime)
	}
}

// HandleGetAllAnimeDownloadStatus gets the status of the all-anime download
//
//	@summary gets the status of the all-anime download operation
//	@desc This returns the current status of the anime batch download job.
//	@returns map[string]interface{}
//	@route /api/v1/anime/download-all/status [GET]
func (h *Handler) HandleGetAllAnimeDownloadStatus(c echo.Context) error {
	// Get the database path and read actual anime count
	// Database is located in the manga download directory
	databasePath := filepath.Join("/aeternae/theater/anime/completed", "anime-offline-database-minified.json")

	// Get database statistics to show real anime count
	stats, err := h.getAnimeDatabaseStats(databasePath)
	if err != nil {
		// If database can't be read, show error but don't fail completely
		return h.RespondWithData(c, map[string]interface{}{
			"job":             nil,
			"status":          "idle",
			"message":         fmt.Sprintf("Database error: %v", err),
			"totalAnime":      0,
			"estimatedSizeGB": 0,
		})
	}

	// Calculate estimated size (rough estimate: 500MB per anime)
	totalAnime := stats["totalAnime"].(int)
	estimatedSizeGB := float64(totalAnime) * 0.5 // 500MB per anime

	// Check for active job
	activeJob := h.getActiveJob()
	if activeJob != nil {
		// Return active job status
		snap := snapshotJob(activeJob)
		jobStatus := snap.Status
		jobProgress := snap.Progress
		jobMessage := fmt.Sprintf("Processing %d/%d anime (%.1f%% complete)", snap.CompletedAnime, snap.TotalAnime, jobProgress)

		return h.RespondWithData(c, map[string]interface{}{
			"job":             snap,
			"status":          jobStatus,
			"message":         jobMessage,
			"totalAnime":      totalAnime,
			"estimatedSizeGB": estimatedSizeGB,
			"databaseStats":   stats,
		})
	}

	return h.RespondWithData(c, map[string]interface{}{
		"job":             nil,
		"status":          "idle",
		"message":         "Ready to start anime batch download",
		"totalAnime":      totalAnime,
		"estimatedSizeGB": estimatedSizeGB,
		"databaseStats":   stats,
	})
}

// HandleLoadAnimeDatabase loads the anime offline database
//
//	@summary loads the anime offline database
//	@desc This loads the anime offline database and returns statistics.
//	@returns map[string]interface{}
//	@route /api/v1/anime/database/load [POST]
func (h *Handler) HandleLoadAnimeDatabase(c echo.Context) error {
	// Get the database path from the manga download directory
	databasePath := filepath.Join("/aeternae/library/manga/seanime", "anime-offline-database-minified.json")

	// Check if database file exists
	if _, err := os.Stat(databasePath); os.IsNotExist(err) {
		return h.RespondWithData(c, map[string]interface{}{
			"loaded":  false,
			"error":   "Anime database file not found",
			"path":    databasePath,
			"message": "Please ensure anime-offline-database-minified.json is in the app data directory",
		})
	}

	// Get basic statistics by reading the file
	stats, err := h.getAnimeDatabaseStats(databasePath)
	if err != nil {
		return h.RespondWithData(c, map[string]interface{}{
			"loaded": false,
			"error":  fmt.Sprintf("Failed to read database: %v", err),
		})
	}

	return h.RespondWithData(c, map[string]interface{}{
		"loaded": true,
		"stats":  stats,
	})
}

// HandleStartAllAnimeDownload starts downloading ALL anime from the database
//
//	@summary starts downloading ALL anime
//	@desc This starts the massive anime batch download operation.
//	@returns map[string]interface{}
//	@route /api/v1/anime/download-all [POST]
func (h *Handler) HandleStartAllAnimeDownload(c echo.Context) error {
	var settings AllAnimeDownloadSettings
	if err := c.Bind(&settings); err != nil {
		return h.RespondWithData(c, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	// If a job is already active (running/pending/paused), return it to make the endpoint idempotent
	if existing := h.getActiveJob(); existing != nil {
		existing.mu.RLock()
		status := existing.Status
		existing.mu.RUnlock()
		if status == "running" || status == "pending" || status == "paused" {
			return h.RespondWithData(c, map[string]interface{}{
				"success": true,
				"job":     existing,
				"message": "An all-anime batch download is already in progress",
			})
		}
	}

	// Load and filter anime database
	databasePath := filepath.Join("/aeternae/library/manga/seanime", "anime-offline-database-minified.json")
	animeList, err := h.loadAnimeDatabase(databasePath)
	if err != nil {
		return h.RespondWithData(c, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to load anime database: %v", err),
		})
	}

	// Filter anime based on settings
	filteredAnime := h.filterAnimeForDownload(animeList, settings)
	if len(filteredAnime) == 0 {
		return h.RespondWithData(c, map[string]interface{}{
			"success": false,
			"error":   "No anime match the specified criteria",
		})
	}

	// Create job and start processing
	jobID := fmt.Sprintf("batch_%d", time.Now().Unix())
	job := &BatchDownloadJob{
		ID:             jobID,
		Status:         "running",
		Progress:       0.0,
		TotalAnime:     len(filteredAnime),
		CompletedAnime: 0,
		FailedAnime:    0,
		ActiveBatches:  0,
		StartTime:      time.Now(),
		Settings:       settings,
		Errors:         make([]string, 0),
	}

	// Store the active job (in a real implementation, this would be in a proper job manager)
	h.setActiveJob(job)

	// Start processing in background
	go h.processBatchDownload(job, filteredAnime)

	return h.RespondWithData(c, map[string]interface{}{
		"success": true,
		"job":     job,
		"message": fmt.Sprintf("Started batch download for %d anime", len(filteredAnime)),
	})
}

// HandleCancelAllAnimeDownload cancels the all-anime download
//
//	@summary cancels the all-anime download operation
//	@desc This cancels the currently running anime batch download.
//	@returns map[string]string
//	@route /api/v1/anime/download-all/cancel [POST]
func (h *Handler) HandleCancelAllAnimeDownload(c echo.Context) error {
	active := h.getActiveJob()
	if active == nil {
		return h.RespondWithData(c, map[string]interface{}{
			"success": false,
			"status":  "idle",
			"message": "No active anime batch download to cancel",
		})
	}

	// Mark job as cancelled; background loop checks this flag and exits
	active.mu.Lock()
	if active.Status == "running" || active.Status == "pending" || active.Status == "paused" {
		active.Status = "cancelled"
	}
	active.mu.Unlock()

	h.emitJobUpdate(active)

	return h.RespondWithData(c, map[string]interface{}{
		"success": true,
		"status":  "cancelled",
		"job":     active,
		"message": "Cancellation requested; job will stop shortly",
	})
}

// HandleGetAnimeDatabaseStats gets anime database statistics
//
//	@summary gets anime database statistics
//	@desc This returns statistics about the loaded anime database.
//	@returns map[string]interface{}
//	@route /api/v1/anime/database/stats [GET]
func (h *Handler) HandleGetAnimeDatabaseStats(c echo.Context) error {
	databasePath := filepath.Join("/aeternae/library/manga/seanime", "anime-offline-database-minified.json")
	stats, err := h.getAnimeDatabaseStats(databasePath)
	if err != nil {
		return h.RespondWithData(c, map[string]interface{}{
			"error": fmt.Sprintf("Failed to get database stats: %v", err),
		})
	}

	return h.RespondWithData(c, stats)
}

// HandlePreviewAllAnimeDownload previews what would be downloaded
//
//	@summary previews the all-anime download
//	@desc This returns a preview of what would be downloaded based on settings.
//	@returns map[string]interface{}
//	@route /api/v1/anime/download-all/preview [POST]
func (h *Handler) HandlePreviewAllAnimeDownload(c echo.Context) error {
	var settings AllAnimeDownloadSettings
	if err := c.Bind(&settings); err != nil {
		return h.RespondWithData(c, map[string]interface{}{
			"error": fmt.Sprintf("Invalid request body: %v", err),
		})
	}

	databasePath := filepath.Join("/aeternae/library/manga/seanime", "anime-offline-database-minified.json")
	preview, err := h.generateAnimeDownloadPreview(databasePath, &settings)
	if err != nil {
		return h.RespondWithData(c, map[string]interface{}{
			"error": fmt.Sprintf("Failed to generate preview: %v", err),
		})
	}

	return h.RespondWithData(c, preview)
}

// Helper function to get anime database statistics
func (h *Handler) getAnimeDatabaseStats(databasePath string) (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(databasePath); os.IsNotExist(err) {
		return map[string]interface{}{
			"totalAnime": 0,
			"loaded":     false,
			"error":      "Database file not found",
			"path":       databasePath,
		}, nil
	}

	// Read and parse the database file
	data, err := os.ReadFile(databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read database file: %w", err)
	}

	var database AnimeOfflineDatabase
	if err := json.Unmarshal(data, &database); err != nil {
		return nil, fmt.Errorf("failed to parse database JSON: %w", err)
	}

	entries := database.Data

	// Calculate basic statistics
	totalAnime := len(entries)
	typeCount := make(map[string]int)
	tagCount := make(map[string]int)
	statusCount := make(map[string]int)

	for _, entry := range entries {
		typeCount[entry.Type]++
		statusCount[entry.Status]++
		for _, tag := range entry.Tags {
			tagCount[tag]++
		}
	}

	return map[string]interface{}{
		"totalAnime":  totalAnime,
		"loaded":      true,
		"typeCount":   typeCount,
		"tagCount":    tagCount,
		"statusCount": statusCount,
		"path":        databasePath,
	}, nil
}

// Helper function to generate anime download preview
func (h *Handler) generateAnimeDownloadPreview(databasePath string, settings *AllAnimeDownloadSettings) (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(databasePath); os.IsNotExist(err) {
		return map[string]interface{}{
			"error": "Database file not found",
			"path":  databasePath,
		}, nil
	}

	// Read and parse the database file
	data, err := os.ReadFile(databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read database file: %w", err)
	}

	var entries []AnimeOfflineEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse database JSON: %w", err)
	}

	// Apply filters based on settings
	filteredEntries := h.filterAnimeEntriesForPreview(entries, settings)

	return map[string]interface{}{
		"totalAnime":    len(entries),
		"filteredAnime": len(filteredEntries),
		"settings":      settings,
		"sampleEntries": h.getSampleEntries(filteredEntries, 10),
		"message":       "Preview generated successfully",
	}, nil
}

// Helper function to filter anime entries for preview
func (h *Handler) filterAnimeEntriesForPreview(entries []AnimeOfflineEntry, settings *AllAnimeDownloadSettings) []AnimeOfflineEntry {
	var filtered []AnimeOfflineEntry

	for _, entry := range entries {
		// Year filtering not available in current database structure
		// Skip year-based filters for now

		// Apply type filters
		if settings.SkipOva && (entry.Type == "OVA" || entry.Type == "ONA") {
			continue
		}
		if settings.SkipSpecials && entry.Type == "SPECIAL" {
			continue
		}

		// Apply tag filters (using tags instead of genres)
		if len(settings.IncludeGenres) > 0 {
			hasIncludedTag := false
			for _, tag := range entry.Tags {
				for _, includeGenre := range settings.IncludeGenres {
					if strings.EqualFold(tag, includeGenre) {
						hasIncludedTag = true
						break
					}
				}
				if hasIncludedTag {
					break
				}
			}
			if !hasIncludedTag {
				continue
			}
		}

		if len(settings.ExcludeGenres) > 0 {
			hasExcludedTag := false
			for _, tag := range entry.Tags {
				for _, excludeGenre := range settings.ExcludeGenres {
					if strings.EqualFold(tag, excludeGenre) {
						hasExcludedTag = true
						break
					}
				}
				if hasExcludedTag {
					break
				}
			}
			if hasExcludedTag {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	return filtered
}

// Helper function to get sample entries for preview
func (h *Handler) getSampleEntries(entries []AnimeOfflineEntry, limit int) []AnimeOfflineEntry {
	if len(entries) <= limit {
		return entries
	}
	return entries[:limit]
}

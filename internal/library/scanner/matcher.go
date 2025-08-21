package scanner

import (
	"errors"
	"fmt"
	"math"
	"seanime/internal/api/anilist"
	"seanime/internal/hook"
	"seanime/internal/library/anime"
	"seanime/internal/library/summary"
	"seanime/internal/util"
	"seanime/internal/util/comparison"
	"strings"
	"time"

	"github.com/adrg/strutil/metrics"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"github.com/sourcegraph/conc/pool"
)

type Matcher struct {
	LocalFiles         []*anime.LocalFile
	MediaContainer     *MediaContainer
	CompleteAnimeCache *anilist.CompleteAnimeCache
	Logger             *zerolog.Logger
	ScanLogger         *ScanLogger
	ScanSummaryLogger  *summary.ScanSummaryLogger // optional
	Algorithm          string
	Threshold          float64
}

// containsNC detects NCOP/NCED markers in file or folder titles as a fallback when metadata type isn't set.
func containsNC(lf *anime.LocalFile) bool {
	check := func(s string) bool {
		u := strings.ToUpper(s)
		return strings.Contains(u, "NCOP") || strings.Contains(u, "NCED")
	}
	if lf == nil {
		return false
	}
	if check(lf.Name) {
		return true
	}
	if lf.ParsedData != nil {
		if check(lf.ParsedData.Original) || check(lf.ParsedData.Title) || check(lf.ParsedData.EpisodeTitle) {
			return true
		}
	}
	if t := lf.GetFolderTitle(); check(t) {
		return true
	}
	// Also examine parsed folder data entries
	for _, fpd := range lf.ParsedFolderData {
		if fpd == nil {
			continue
		}
		if check(fpd.Original) || check(fpd.Title) {
			return true
		}
	}
	return false
}

var (
	ErrNoLocalFiles = errors.New("[matcher] no local files")
)

// MatchLocalFilesWithMedia will match each anime.LocalFile with a specific anilist.BaseAnime and modify the LocalFile's `mediaId`
func (m *Matcher) MatchLocalFilesWithMedia() error {

	if m.Threshold == 0 {
		m.Threshold = 0.5
	}

	start := time.Now()

	if len(m.LocalFiles) == 0 {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.WarnLevel).Msg("No local files")
		}
		return ErrNoLocalFiles
	}
	if len(m.MediaContainer.allMedia) == 0 {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.WarnLevel).Msg("No media fed into the matcher")
		}
		return errors.New("[matcher] no media fed into the matcher")
	}

	m.Logger.Debug().Msg("matcher: Starting matching process")

	// Invoke ScanMatchingStarted hook
	event := &ScanMatchingStartedEvent{
		LocalFiles:      m.LocalFiles,
		NormalizedMedia: m.MediaContainer.NormalizedMedia,
		Algorithm:       m.Algorithm,
		Threshold:       m.Threshold,
	}
	_ = hook.GlobalHookManager.OnScanMatchingStarted().Trigger(event)
	m.LocalFiles = event.LocalFiles
	m.MediaContainer.NormalizedMedia = event.NormalizedMedia
	m.Algorithm = event.Algorithm
	m.Threshold = event.Threshold

	if event.DefaultPrevented {
		m.Logger.Debug().Msg("matcher: Match stopped by hook")
		return nil
	}

	// Parallelize the matching process
	lop.ForEach(m.LocalFiles, func(localFile *anime.LocalFile, _ int) {
		m.matchLocalFileWithMedia(localFile)
	})

	// m.validateMatches()

	// Invoke ScanMatchingCompleted hook
	completedEvent := &ScanMatchingCompletedEvent{
		LocalFiles: m.LocalFiles,
	}
	_ = hook.GlobalHookManager.OnScanMatchingCompleted().Trigger(completedEvent)
	m.LocalFiles = completedEvent.LocalFiles

	if m.ScanLogger != nil {
		m.ScanLogger.LogMatcher(zerolog.InfoLevel).
			Int64("ms", time.Since(start).Milliseconds()).
			Int("files", len(m.LocalFiles)).
			Int("unmatched", lo.CountBy(m.LocalFiles, func(localFile *anime.LocalFile) bool {
				return localFile.MediaId == 0
			})).
			Msg("Finished matching process")
	}

	return nil
}

// matchLocalFileWithMedia finds the best match for the local file
// If the best match is above a certain threshold, set the local file's mediaId to the best match's id
// If the best match is below a certain threshold, leave the local file's mediaId to 0
func (m *Matcher) matchLocalFileWithMedia(lf *anime.LocalFile) {
	defer util.HandlePanicInModuleThenS("scanner/matcher/matchLocalFileWithMedia", func(stackTrace string) {
		lf.MediaId = 0
		/*Log*/
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.ErrorLevel).
				Str("filename", lf.Name).
				Msg("Panic occurred, file un-matched")
		}
		m.ScanSummaryLogger.LogPanic(lf, stackTrace)
	})

	// Check if the local file has already been matched
	if lf.MediaId != 0 {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.DebugLevel).
				Str("filename", lf.Name).
				Msg("File already matched")
		}
		m.ScanSummaryLogger.LogFileNotMatched(lf, "Already matched")
		return
	}
	// Respect auto-match block flag: do not automatically match this file
	if lf.AutoMatchBlocked {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.DebugLevel).
				Str("filename", lf.Name).
				Msg("Auto-match blocked, skipping")
		}
		m.ScanSummaryLogger.LogFileNotMatched(lf, "Auto-match blocked")
		return
	}
	// Check if the local file has a title
	if lf.GetParsedTitle() == "" {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.WarnLevel).
				Str("filename", lf.Name).
				Msg("File has no parsed title")
		}
		m.ScanSummaryLogger.LogFileNotMatched(lf, "No parsed title found")
		return
	}

	// Exclude NC tracks (NCOP/NCED) from auto-matching
	if lf.GetType() == anime.LocalFileTypeNC || containsNC(lf) {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.DebugLevel).
				Str("filename", lf.Name).
				Msg("NC track (NCOP/NCED) excluded from auto-matching")
		}
		m.ScanSummaryLogger.LogFileNotMatched(lf, "NC track (NCOP/NCED) excluded")
		return
	}

	// Create title variations
	// Check cache for title variation

	titleVariations := lf.GetTitleVariations()

	if len(titleVariations) == 0 {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.WarnLevel).
				Str("filename", lf.Name).
				Msg("No titles found")
		}
		m.ScanSummaryLogger.LogFileNotMatched(lf, "No title variations found")
		return
	}

	if m.ScanLogger != nil {
		m.ScanLogger.LogMatcher(zerolog.DebugLevel).
			Str("filename", lf.Name).
			Interface("titleVariations", titleVariations).
			Msg("Matching local file")
	}
	m.ScanSummaryLogger.LogDebug(lf, util.InlineSpewT(titleVariations))

	//------------------

	// Helper: try to find a best match from folder context (series folder and its children)
	folderMediaMatch, folderRating, folderFound := m.getFolderContextBestMatch(lf)

	var levMatch *comparison.LevenshteinResult
	var sdMatch *comparison.SorensenDiceResult
	var jaccardMatch *comparison.JaccardResult

	if m.Algorithm == "jaccard" {
		// Using Jaccard
		// Get the matchs for each title variation
		compResults := lop.Map(titleVariations, func(title *string, _ int) *comparison.JaccardResult {
			var eng, rom, syn *comparison.JaccardResult
			if len(m.MediaContainer.romTitles) > 0 {
				if r, found := comparison.FindBestMatchWithJaccard(title, m.MediaContainer.romTitles); found {
					rom = r
				}
			}
			if len(m.MediaContainer.engTitles) > 0 {
				if e, found := comparison.FindBestMatchWithJaccard(title, m.MediaContainer.engTitles); found {
					eng = e
				}
			}
			if len(m.MediaContainer.synonyms) > 0 {
				if s, found := comparison.FindBestMatchWithJaccard(title, m.MediaContainer.synonyms); found {
					syn = s
				}
			}
			// Prefer Romaji unless another exceeds by more than epsilon
			epsilon := 0.02
			candidate := rom
			if candidate == nil {
				// No romaji; use the best available
				if eng != nil && (syn == nil || eng.Rating >= syn.Rating) {
					candidate = eng
				} else {
					candidate = syn
				}
			} else {
				if eng != nil && eng.Rating > candidate.Rating+epsilon {
					candidate = eng
				}
				if syn != nil && syn.Rating > candidate.Rating+epsilon {
					candidate = syn
				}
			}
			return candidate
		})

		// Retrieve the match from all the title variations results
		jaccardMatch = lo.Reduce(compResults, func(prev *comparison.JaccardResult, curr *comparison.JaccardResult, _ int) *comparison.JaccardResult {
			if prev.Rating > curr.Rating {
				return prev
			} else {
				return curr
			}
		}, compResults[0])

		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.DebugLevel).
				Str("filename", lf.Name).
				Interface("match", jaccardMatch).
				Interface("results", compResults).
				Msg("Jaccard match")
		}
		m.ScanSummaryLogger.LogComparison(lf, "Jaccard", *jaccardMatch.Value, "Rating", util.InlineSpewT(jaccardMatch.Rating))

	} else if m.Algorithm == "sorensen-dice" {
		// Using Sorensen-Dice
		// Get the matchs for each title variation
		compResults := lop.Map(titleVariations, func(title *string, _ int) *comparison.SorensenDiceResult {
			var eng, rom, syn *comparison.SorensenDiceResult
			if len(m.MediaContainer.romTitles) > 0 {
				if r, found := comparison.FindBestMatchWithSorensenDice(title, m.MediaContainer.romTitles); found {
					rom = r
				}
			}
			if len(m.MediaContainer.engTitles) > 0 {
				if e, found := comparison.FindBestMatchWithSorensenDice(title, m.MediaContainer.engTitles); found {
					eng = e
				}
			}
			if len(m.MediaContainer.synonyms) > 0 {
				if s, found := comparison.FindBestMatchWithSorensenDice(title, m.MediaContainer.synonyms); found {
					syn = s
				}
			}
			// Prefer Romaji unless another exceeds by more than epsilon
			epsilon := 0.02
			candidate := rom
			if candidate == nil {
				// No romaji; use the best available
				if eng != nil && (syn == nil || eng.Rating >= syn.Rating) {
					candidate = eng
				} else {
					candidate = syn
				}
			} else {
				if eng != nil && eng.Rating > candidate.Rating+epsilon {
					candidate = eng
				}
				if syn != nil && syn.Rating > candidate.Rating+epsilon {
					candidate = syn
				}
			}
			return candidate
		})

		// Retrieve the match from all the title variations results
		sdMatch = lo.Reduce(compResults, func(prev *comparison.SorensenDiceResult, curr *comparison.SorensenDiceResult, _ int) *comparison.SorensenDiceResult {
			if prev.Rating > curr.Rating {
				return prev
			} else {
				return curr
			}
		}, compResults[0])

		//util.Spew(compResults)

		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.DebugLevel).
				Str("filename", lf.Name).
				Interface("match", sdMatch).
				Interface("results", compResults).
				Msg("Sorensen-Dice match")
		}
		m.ScanSummaryLogger.LogComparison(lf, "Sorensen-Dice", *sdMatch.Value, "Rating", util.InlineSpewT(sdMatch.Rating))

	} else {
		// Using Levenshtein
		// Get the matches for each title variation
		levCompResults := lop.Map(titleVariations, func(title *string, _ int) *comparison.LevenshteinResult {
			var eng, rom, syn *comparison.LevenshteinResult
			if len(m.MediaContainer.romTitles) > 0 {
				if r, found := comparison.FindBestMatchWithLevenshtein(title, m.MediaContainer.romTitles); found {
					rom = r
				}
			}
			if len(m.MediaContainer.engTitles) > 0 {
				if e, found := comparison.FindBestMatchWithLevenshtein(title, m.MediaContainer.engTitles); found {
					eng = e
				}
			}
			if len(m.MediaContainer.synonyms) > 0 {
				if s, found := comparison.FindBestMatchWithLevenshtein(title, m.MediaContainer.synonyms); found {
					syn = s
				}
			}
			// For Levenshtein, lower distance is better. Prefer Romaji unless another beats it by more than epsilon.
			epsilon := 1 // distance margin
			candidate := rom
			if candidate == nil {
				// No romaji; pick the lowest distance among others
				if eng != nil && (syn == nil || eng.Distance <= syn.Distance) {
					candidate = eng
				} else {
					candidate = syn
				}
			} else {
				if eng != nil && eng.Distance+epsilon < candidate.Distance {
					candidate = eng
				}
				if syn != nil && syn.Distance+epsilon < candidate.Distance {
					candidate = syn
				}
			}
			return candidate
		})

		levMatch = lo.Reduce(levCompResults, func(prev *comparison.LevenshteinResult, curr *comparison.LevenshteinResult, _ int) *comparison.LevenshteinResult {
			if prev.Distance < curr.Distance {
				return prev
			} else {
				return curr
			}
		}, levCompResults[0])

		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.DebugLevel).
				Str("filename", lf.Name).
				Interface("match", levMatch).
				Interface("results", levCompResults).
				Int("distance", levMatch.Distance).
				Msg("Levenshtein match")
		}
		m.ScanSummaryLogger.LogComparison(lf, "Levenshtein", *levMatch.Value, "Distance", util.InlineSpewT(levMatch.Distance))
	}

	//------------------

	var mediaMatch *anime.NormalizedMedia
	var found bool
	finalRating := 0.0

	if sdMatch != nil {
		finalRating = sdMatch.Rating
		mediaMatch, found = m.MediaContainer.GetMediaFromTitleOrSynonym(sdMatch.Value)

	} else if jaccardMatch != nil {
		finalRating = jaccardMatch.Rating
		mediaMatch, found = m.MediaContainer.GetMediaFromTitleOrSynonym(jaccardMatch.Value)

	} else {
		dice := metrics.NewSorensenDice()
		dice.CaseSensitive = false
		dice.NgramSize = 1
		finalRating = dice.Compare(*levMatch.OriginalValue, *levMatch.Value)
		m.ScanSummaryLogger.LogComparison(lf, "Sorensen-Dice", *levMatch.Value, "Final rating", util.InlineSpewT(finalRating))
		mediaMatch, found = m.MediaContainer.GetMediaFromTitleOrSynonym(levMatch.Value)
	}

	// After setting the mediaId, add the hook invocation
	// Invoke ScanLocalFileMatched hook
	event := &ScanLocalFileMatchedEvent{
		LocalFile: lf,
		Score:     finalRating,
		Match:     mediaMatch,
		Found:     found,
	}
	hook.GlobalHookManager.OnScanLocalFileMatched().Trigger(event)
	lf = event.LocalFile
	mediaMatch = event.Match
	found = event.Found
	finalRating = event.Score

	// Folder-context enforcement and fallback
	if folderFound && folderRating >= m.Threshold {
		// If filename-based match disagrees with folder-based series, reject cross-series matches
		if found && mediaMatch != nil && mediaMatch.ID != folderMediaMatch.ID {
			if m.ScanLogger != nil {
				m.ScanLogger.LogMatcher(zerolog.WarnLevel).
					Str("filename", lf.Name).
					Str("folderBest", folderMediaMatch.GetTitleSafe()).
					Int("folderId", folderMediaMatch.ID).
					Str("fileBest", mediaMatch.GetTitleSafe()).
					Int("fileId", mediaMatch.ID).
					Msg("Rejected match: disagrees with series folder context")
			}
			m.ScanSummaryLogger.LogFileNotMatched(lf, fmt.Sprintf("Disagrees with folder context: folder '%s' (id %d) vs file best '%s' (id %d)", folderMediaMatch.GetTitleSafe(), folderMediaMatch.ID, mediaMatch.GetTitleSafe(), mediaMatch.ID))
			lf.MediaId = 0
			return
		}
		// If no solid filename match, fall back to folder match
		if !found || finalRating < m.Threshold || mediaMatch == nil {
			mediaMatch = folderMediaMatch
			found = true
			finalRating = folderRating
			if m.ScanLogger != nil {
				m.ScanLogger.LogMatcher(zerolog.DebugLevel).
					Str("filename", lf.Name).
					Str("title", mediaMatch.GetTitleSafe()).
					Int("id", mediaMatch.ID).
					Float64("rating", finalRating).
					Msg("Using folder-context match")
			}
		}
	}

	// Check if the hook overrode the match
	if event.DefaultPrevented {
		if m.ScanLogger != nil {
			if mediaMatch != nil {
				m.ScanLogger.LogMatcher(zerolog.DebugLevel).
					Str("filename", lf.Name).
					Int("id", mediaMatch.ID).
					Msg("Hook overrode match")
			} else {
				m.ScanLogger.LogMatcher(zerolog.DebugLevel).
					Str("filename", lf.Name).
					Msg("Hook overrode match, no match found")
			}
		}
		if mediaMatch != nil {
			lf.MediaId = mediaMatch.ID
		} else {
			lf.MediaId = 0
		}
		return
	}

	if !found {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.ErrorLevel).
				Str("filename", lf.Name).
				Msg("No media found from comparison result")
		}
		m.ScanSummaryLogger.LogFileNotMatched(lf, "No media found from comparison result")
		return
	}

	if m.ScanLogger != nil {
		m.ScanLogger.LogMatcher(zerolog.DebugLevel).
			Str("filename", lf.Name).
			Str("title", mediaMatch.GetTitleSafe()).
			Int("id", mediaMatch.ID).
			Msg("Best match found")
	}

	if finalRating < m.Threshold {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.DebugLevel).
				Str("filename", lf.Name).
				Float64("rating", finalRating).
				Float64("threshold", m.Threshold).
				Msg("Best match Sorensen-Dice rating too low, un-matching file")
		}
		m.ScanSummaryLogger.LogFailedMatch(lf, "Rating too low, threshold is "+fmt.Sprintf("%f", m.Threshold))
		return
	}

	if m.ScanLogger != nil {
		m.ScanLogger.LogMatcher(zerolog.DebugLevel).
			Str("filename", lf.Name).
			Float64("rating", finalRating).
			Float64("threshold", m.Threshold).
			Msg("Best match rating high enough, matching file")
	}
	m.ScanSummaryLogger.LogSuccessfullyMatched(lf, mediaMatch.ID)

	lf.MediaId = mediaMatch.ID
	// Mark as automatically linked by the scanner
	lf.LinkSource = "auto"
}

//----------------------------------------------------------------------------------------------------------------------

// getFolderContextBestMatch tries to pick a best media match using the parsed folder names
// (series folder and its children). It mirrors the main matching algorithm but only uses
// folder-derived title variations.
func (m *Matcher) getFolderContextBestMatch(lf *anime.LocalFile) (*anime.NormalizedMedia, float64, bool) {
	// Collect folder title variations (prefer parsed Title, then Original)
	folderTitles := make([]*string, 0)
	for _, p := range lf.ParsedFolderData {
		if p == nil {
			continue
		}
		if len(p.Title) > 0 {
			v := p.Title
			folderTitles = append(folderTitles, &v)
			continue
		}
		if len(p.Original) > 0 {
			v := p.Original
			folderTitles = append(folderTitles, &v)
		}
	}
	if len(folderTitles) == 0 {
		return nil, 0, false
	}

	var finalTitle *string
	var finalScore float64

	if m.Algorithm == "jaccard" {
		compResults := lop.Map(folderTitles, func(title *string, _ int) *comparison.JaccardResult {
			var eng, rom, syn *comparison.JaccardResult
			if len(m.MediaContainer.romTitles) > 0 {
				if r, found := comparison.FindBestMatchWithJaccard(title, m.MediaContainer.romTitles); found {
					rom = r
				}
			}
			if len(m.MediaContainer.engTitles) > 0 {
				if e, found := comparison.FindBestMatchWithJaccard(title, m.MediaContainer.engTitles); found {
					eng = e
				}
			}
			if len(m.MediaContainer.synonyms) > 0 {
				if s, found := comparison.FindBestMatchWithJaccard(title, m.MediaContainer.synonyms); found {
					syn = s
				}
			}
			epsilon := 0.02
			candidate := rom
			if candidate == nil {
				if eng != nil && (syn == nil || eng.Rating >= syn.Rating) {
					candidate = eng
				} else {
					candidate = syn
				}
			} else {
				if eng != nil && eng.Rating > candidate.Rating+epsilon {
					candidate = eng
				}
				if syn != nil && syn.Rating > candidate.Rating+epsilon {
					candidate = syn
				}
			}
			return candidate
		})
		best := lo.Reduce(compResults, func(prev, curr *comparison.JaccardResult, _ int) *comparison.JaccardResult {
			if prev.Rating > curr.Rating {
				return prev
			} else {
				return curr
			}
		}, compResults[0])
		finalTitle = best.Value
		finalScore = best.Rating
	} else if m.Algorithm == "sorensen-dice" {
		compResults := lop.Map(folderTitles, func(title *string, _ int) *comparison.SorensenDiceResult {
			var eng, rom, syn *comparison.SorensenDiceResult
			if len(m.MediaContainer.romTitles) > 0 {
				if r, found := comparison.FindBestMatchWithSorensenDice(title, m.MediaContainer.romTitles); found {
					rom = r
				}
			}
			if len(m.MediaContainer.engTitles) > 0 {
				if e, found := comparison.FindBestMatchWithSorensenDice(title, m.MediaContainer.engTitles); found {
					eng = e
				}
			}
			if len(m.MediaContainer.synonyms) > 0 {
				if s, found := comparison.FindBestMatchWithSorensenDice(title, m.MediaContainer.synonyms); found {
					syn = s
				}
			}
			epsilon := 0.02
			candidate := rom
			if candidate == nil {
				if eng != nil && (syn == nil || eng.Rating >= syn.Rating) {
					candidate = eng
				} else {
					candidate = syn
				}
			} else {
				if eng != nil && eng.Rating > candidate.Rating+epsilon {
					candidate = eng
				}
				if syn != nil && syn.Rating > candidate.Rating+epsilon {
					candidate = syn
				}
			}
			return candidate
		})
		best := lo.Reduce(compResults, func(prev, curr *comparison.SorensenDiceResult, _ int) *comparison.SorensenDiceResult {
			if prev.Rating > curr.Rating {
				return prev
			} else {
				return curr
			}
		}, compResults[0])
		finalTitle = best.Value
		finalScore = best.Rating
	} else {
		// Levenshtein path: compute Sorensen-Dice score on the chosen title for comparability
		levCompResults := lop.Map(folderTitles, func(title *string, _ int) *comparison.LevenshteinResult {
			var eng, rom, syn *comparison.LevenshteinResult
			if len(m.MediaContainer.romTitles) > 0 {
				if r, found := comparison.FindBestMatchWithLevenshtein(title, m.MediaContainer.romTitles); found {
					rom = r
				}
			}
			if len(m.MediaContainer.engTitles) > 0 {
				if e, found := comparison.FindBestMatchWithLevenshtein(title, m.MediaContainer.engTitles); found {
					eng = e
				}
			}
			if len(m.MediaContainer.synonyms) > 0 {
				if s, found := comparison.FindBestMatchWithLevenshtein(title, m.MediaContainer.synonyms); found {
					syn = s
				}
			}
			epsilon := 1
			candidate := rom
			if candidate == nil {
				if eng != nil && (syn == nil || eng.Distance <= syn.Distance) {
					candidate = eng
				} else {
					candidate = syn
				}
			} else {
				if eng != nil && eng.Distance+epsilon < candidate.Distance {
					candidate = eng
				}
				if syn != nil && syn.Distance+epsilon < candidate.Distance {
					candidate = syn
				}
			}
			return candidate
		})
		best := lo.Reduce(levCompResults, func(prev, curr *comparison.LevenshteinResult, _ int) *comparison.LevenshteinResult {
			if prev.Distance < curr.Distance {
				return prev
			} else {
				return curr
			}
		}, levCompResults[0])
		// compute comparable Sorensen-Dice rating for logging/threshold
		dice := metrics.NewSorensenDice()
		dice.CaseSensitive = false
		dice.NgramSize = 1
		finalTitle = best.Value
		finalScore = dice.Compare(*best.OriginalValue, *best.Value)
	}

	if finalTitle == nil {
		return nil, 0, false
	}
	media, ok := m.MediaContainer.GetMediaFromTitleOrSynonym(finalTitle)
	if !ok || media == nil {
		return nil, 0, false
	}
	return media, finalScore, true
}

// validateMatches compares groups of local files' titles with the media titles and un-matches the local files that have a lower rating than the highest rating.
func (m *Matcher) validateMatches() {

	if m.ScanLogger != nil {
		m.ScanLogger.LogMatcher(zerolog.InfoLevel).Msg("Validating matches")
	}

	// Group local files by media ID
	groups := lop.GroupBy(m.LocalFiles, func(localFile *anime.LocalFile) int {
		return localFile.MediaId
	})

	// Remove the group with unmatched media
	delete(groups, 0)

	// Un-match files with lower ratings
	p := pool.New()
	for mId, files := range groups {
		p.Go(func() {
			if len(files) > 0 {
				m.validateMatchGroup(mId, files)
			}
		})
	}
	p.Wait()

}

// validateMatchGroup compares the local files' titles under the same media
// with the media titles and un-matches the local files that have a lower rating.
// This is done to try and filter out wrong matches.
func (m *Matcher) validateMatchGroup(mediaId int, lfs []*anime.LocalFile) {

	media, found := m.MediaContainer.GetMediaFromId(mediaId)
	if !found {
		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.ErrorLevel).
				Int("mediaId", mediaId).
				Msg("Media not found in media container")
		}
		return
	}

	titles := media.GetAllTitles()

	// Compare all files' parsed title with the media title
	// Get the highest rating that will be used to un-match lower rated files
	p := pool.NewWithResults[float64]()
	for _, lf := range lfs {
		p.Go(func() float64 {
			t := lf.GetParsedTitle()
			if comparison.ValueContainsSpecial(lf.Name) || comparison.ValueContainsNC(lf.Name) {
				return 0
			}
			compRes, ok := comparison.FindBestMatchWithSorensenDice(&t, titles)
			if ok {
				return compRes.Rating
			}
			return 0
		})
	}
	fileRatings := p.Wait()

	if m.ScanLogger != nil {
		m.ScanLogger.LogMatcher(zerolog.DebugLevel).
			Int("mediaId", mediaId).
			Any("fileRatings", fileRatings).
			Msg("File ratings")
	}

	highestRating := lo.Reduce(fileRatings, func(prev float64, curr float64, _ int) float64 {
		if prev > curr {
			return prev
		} else {
			return curr
		}
	}, 0.0)

	// Un-match files that have a lower rating than the ceiling
	// UNLESS they are Special or NC
	lop.ForEach(lfs, func(lf *anime.LocalFile, _ int) {
		if !comparison.ValueContainsSpecial(lf.Name) && !comparison.ValueContainsNC(lf.Name) {
			t := lf.GetParsedTitle()
			if compRes, ok := comparison.FindBestMatchWithSorensenDice(&t, titles); ok {
				// If the local file's rating is lower, un-match it
				// Unless the difference is less than 0.7 (very lax since a lot of anime have very long names that can be truncated)
				if compRes.Rating < highestRating && math.Abs(compRes.Rating-highestRating) > 0.7 {
					lf.MediaId = 0

					if m.ScanLogger != nil {
						m.ScanLogger.LogMatcher(zerolog.WarnLevel).
							Int("mediaId", mediaId).
							Str("filename", lf.Name).
							Float64("rating", compRes.Rating).
							Float64("highestRating", highestRating).
							Msg("Rating does not match parameters, un-matching file")
					}
					m.ScanSummaryLogger.LogUnmatched(lf, fmt.Sprintf("Rating does not match parameters. File rating: %f, highest rating: %f", compRes.Rating, highestRating))

				} else {

					if m.ScanLogger != nil {
						m.ScanLogger.LogMatcher(zerolog.DebugLevel).
							Int("mediaId", mediaId).
							Str("filename", lf.Name).
							Float64("rating", compRes.Rating).
							Float64("highestRating", highestRating).
							Msg("Rating matches parameters, keeping file matched")
					}
					m.ScanSummaryLogger.LogMatchValidated(lf, mediaId)

				}
			}
		}
	})

}

package anime

import (
	"strings"
)

// IsVideoFile returns true for common video extensions
func IsVideoFile(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".mkv") || strings.HasSuffix(p, ".mp4") || strings.HasSuffix(p, ".avi") || strings.HasSuffix(p, ".mov") || strings.HasSuffix(p, ".m4v")
}

// ContainsNCHeuristic detects NCOP/NCED markers in file names and folders.
func ContainsNCHeuristic(lf *LocalFile) bool {
	if lf == nil {
		return false
	}
	check := func(s string) bool {
		u := strings.ToUpper(s)
		return strings.Contains(u, "NCOP") || strings.Contains(u, "NCED")
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
	for _, f := range lf.ParsedFolderData {
		if f == nil {
			continue
		}
		if check(f.Original) || check(f.Title) {
			return true
		}
	}
	return false
}

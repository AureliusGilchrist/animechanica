package manga

import (
	hibikemanga "seanime/internal/extension/hibike/manga"
)

// ChapterForDownload represents a chapter with its parsed number for sorting
type ChapterForDownload struct {
	ChapterDetails *hibikemanga.ChapterDetails
	ChapterNumber  float64
}

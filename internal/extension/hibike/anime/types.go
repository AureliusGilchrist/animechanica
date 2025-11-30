package hibikeanime

// Minimal set of types used by internal/anime/download.go
// This mirrors the expected structures from the upstream hibike project
// but only includes the fields we actually use here.

type SubtitleLink struct {
    URL      string `json:"url"`
    Language string `json:"language"`
}

type EpisodeStreamLink struct {
    URL     string `json:"url"`
    Quality string `json:"quality"`
}

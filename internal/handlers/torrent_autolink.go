package handlers

import (
    "os"
    "path/filepath"
    "time"

    "seanime/internal/database/db_bridge"
    "seanime/internal/events"
    "seanime/internal/library/anime"
    "seanime/internal/util"
)

// scheduleAutoLinkForDestination launches a background routine that periodically scans the
// provided destination directory for completed video files and auto-links them to mediaId.
// It runs for a limited duration and respects NC filtering and AutoMatchBlocked flags.
func (h *Handler) scheduleAutoLinkForDestination(dest string, mediaId int) {
    go func() {
        defer util.HandlePanicInModuleThen("handlers/scheduleAutoLinkForDestination", func() {})

        libraryPaths, err := h.App.Database.GetAllLibraryPathsFromSettings()
        if err != nil {
            libraryPaths = []string{}
        }

        const (
            maxIterations = 80 // ~20 minutes
            sleepSeconds  = 15
        )

        seen := make(map[string]struct{})

        for i := 0; i < maxIterations; i++ {
            var candidates []string
            _ = filepath.Walk(dest, func(path string, info os.FileInfo, err error) error {
                if err != nil || info == nil {
                    return nil
                }
                if info.IsDir() {
                    return nil
                }
                if path == dest {
                    return nil
                }
                if !anime.IsVideoFile(path) {
                    return nil
                }
                if _, ok := seen[path]; ok {
                    return nil
                }
                candidates = append(candidates, path)
                return nil
            })

            if len(candidates) > 0 {
                lfs, lfsId, err := db_bridge.GetLocalFiles(h.App.Database)
                if err == nil {
                    byPath := make(map[string]*anime.LocalFile, len(lfs))
                    for _, lf := range lfs {
                        byPath[lf.GetPath()] = lf
                    }
                    linked := 0
                    for _, p := range candidates {
                        st, err := os.Stat(p)
                        if err != nil || st.IsDir() {
                            continue
                        }
                        lf, ok := byPath[p]
                        if !ok {
                            lf = anime.NewLocalFileS(p, libraryPaths)
                            lfs = append(lfs, lf)
                            byPath[p] = lf
                        }
                        if lf.AutoMatchBlocked {
                            continue
                        }
                        if lf.GetType() == anime.LocalFileTypeNC || anime.ContainsNCHeuristic(lf) {
                            continue
                        }
                        if lf.MediaId == 0 {
                            lf.MediaId = mediaId
                            lf.LinkSource = "auto"
                            linked++
                        }
                    }
                    if linked > 0 {
                        if _, err := db_bridge.SaveLocalFiles(h.App.Database, lfsId, lfs); err == nil {
                            for _, p := range candidates {
                                seen[p] = struct{}{}
                            }
                            h.App.WSEventManager.SendEvent(events.InvalidateQueries, []string{
                                events.GetLocalFilesEndpoint,
                                events.GetAnimeEntryEndpoint,
                                events.GetLibraryCollectionEndpoint,
                                events.GetMissingEpisodesEndpoint,
                            })
                        }
                    }
                }
            }

            time.Sleep(sleepSeconds * time.Second)
        }
    }()
}

package torrent_client

import (
	"context"
	"errors"
	"github.com/hekmon/transmissionrpc/v3"
	"github.com/rs/zerolog"
	"path/filepath"
	"os"
	"regexp"
	"seanime/internal/api/metadata"
	"seanime/internal/events"
	"seanime/internal/torrent_clients/qbittorrent"
	qbittorrent_model "seanime/internal/torrent_clients/qbittorrent/model"
	"seanime/internal/torrent_clients/transmission"
	"seanime/internal/torrents/torrent"
	"strconv"
	"strings"
	"time"
)

// SearchBestMagnet performs a qBittorrent search and returns the best matching batch torrent magnet/URL.
// It enforces batch-only selection by filtering out single-episode results.
// plugins and categories can be nil to use all enabled providers.
func (r *Repository) SearchBestMagnet(ctx context.Context, query string, plugins, categories []string, minSeeders int) (string, error) {
    if r.provider != QbittorrentClient || r.qBittorrentClient == nil {
        return "", errors.New("torrent client: qBittorrent provider not available for search")
    }

    id, err := r.qBittorrentClient.Search.Start(query, plugins, categories)
    if err != nil {
        return "", err
    }
    // Ensure cleanup
    defer func() { _ = r.qBittorrentClient.Search.Delete(id) }()

    timeout := time.After(30 * time.Second)
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return "", ctx.Err()
        case <-timeout:
            return "", errors.New("torrent client: search timeout")
        case <-ticker.C:
            st, err := r.qBittorrentClient.Search.GetStatus(id)
            if err != nil {
                return "", err
            }
            if strings.EqualFold(st.Status, "Running") {
                continue
            }
            // Finished or stopped, retrieve results
            results, err := r.qBittorrentClient.Search.GetResults(id, 200, 0)
            if err != nil {
                return "", err
            }
            if results == nil || len(results.Results) == 0 {
                return "", errors.New("torrent client: no search results")
            }
            // Filter to batch-only and by seeders
            candidates := make([]qbittorrent_model.SearchResult, 0, len(results.Results))
            for _, r0 := range results.Results {
                if r0.NumSeeders < minSeeders {
                    continue
                }
                name := r0.FileName
                if isBatchTitle(name) {
                    candidates = append(candidates, r0)
                }
            }
            if len(candidates) == 0 {
                return "", errors.New("torrent client: no batch results above seeder threshold")
            }
            // Pick best: prioritize highest resolution, then (optionally) dual-audio and bluray when requested,
            // then seeders, then file size
            preferDual := prefersDualAudio(query)
            preferBluray := prefersBluray(query)
            best := candidates[0]
            for _, c := range candidates[1:] {
                br, cr := resolutionScore(best.FileName), resolutionScore(c.FileName)
                if cr > br {
                    best = c
                    continue
                }
                if cr == br {
                    // If user requested dual-audio, prefer titles that indicate dual audio
                    if preferDual {
                        bd, cd := dualAudioScore(best.FileName), dualAudioScore(c.FileName)
                        if cd > bd {
                            best = c
                            continue
                        }
                    }
                    // If user requested bluray, prefer titles that indicate BD/Bluray
                    if preferBluray {
                        bb, cb := blurayScore(best.FileName), blurayScore(c.FileName)
                        if cb > bb {
                            best = c
                            continue
                        }
                    }
                    if c.NumSeeders > best.NumSeeders {
                        best = c
                        continue
                    }
                    if c.NumSeeders == best.NumSeeders && c.FileSize > best.FileSize {
                        best = c
                    }
                }
            }
            return best.FileUrl, nil
        }
    }
}

var (
    reRange     = regexp.MustCompile(`(?i)\b(\d{1,2})\s*[-~–]\s*(\d{1,3})\b`)
    reSeasonTag = regexp.MustCompile(`(?i)\bS(\d{1,2})\b`)
    reComplete  = regexp.MustCompile(`(?i)\b(complete|batch|全集|全巻)\b`)
    reSingleEp  = regexp.MustCompile(`(?i)(?:\b(ep|e|episode)\s*\d{1,3}\b|\[(\d{1,3})\]|[-_\s]\d{1,3}(?:\D|$))`)
)

// isBatchTitle returns true if the given title likely represents a batch/pack torrent rather than a single episode.
func isBatchTitle(title string) bool {
    t := strings.TrimSpace(title)
    if t == "" {
        return false
    }
    // Positive signals: explicit batch words, range like 1-12, season tags
    if reComplete.MatchString(t) || reRange.MatchString(t) || reSeasonTag.MatchString(t) {
        return true
    }
    // Negative: obvious single-episode patterns
    if reSingleEp.MatchString(t) && !reRange.MatchString(t) {
        return false
    }
    // Default: require at least one positive indicator
    return false
}

// resolutionScore extracts a simple resolution score from a torrent title.
// Higher is better: 2160p/4K > 1080p > 720p > 480p > unknown(0)
func resolutionScore(title string) int {
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

// prefersDualAudio checks if the query indicates the user prefers dual-audio.
func prefersDualAudio(query string) bool {
    q := strings.ToLower(query)
    return strings.Contains(q, "dual audio") || strings.Contains(q, "dual-audio") || strings.Contains(q, "dual")
}

// prefersBluray checks if the query indicates the user prefers bluray/BD.
func prefersBluray(query string) bool {
    q := strings.ToLower(query)
    return strings.Contains(q, "bluray") || strings.Contains(q, "bd") || strings.Contains(q, "bdrip")
}

// dualAudioScore returns 1 if the title suggests dual audio, else 0.
func dualAudioScore(title string) int {
    t := strings.ToLower(title)
    if strings.Contains(t, "dual audio") || strings.Contains(t, "dual-audio") || strings.Contains(t, "dual") || strings.Contains(t, "eng+jpn") {
        return 1
    }
    return 0
}

// blurayScore returns 2 for strong bluray indicators, 1 for weaker BD indications, else 0.
func blurayScore(title string) int {
    t := strings.ToLower(title)
    if strings.Contains(t, "bluray") || strings.Contains(t, "bdrip") {
        return 2
    }
    if strings.Contains(t, "bd") {
        return 1
    }
    return 0
}

const (
	QbittorrentClient  = "qbittorrent"
	TransmissionClient = "transmission"
	NoneClient         = "none"
)

type (
	Repository struct {
		logger                      *zerolog.Logger
		qBittorrentClient           *qbittorrent.Client
		transmission                *transmission.Transmission
		torrentRepository           *torrent.Repository
		provider                    string
		metadataProvider            metadata.Provider
		activeTorrentCountCtxCancel context.CancelFunc
		activeTorrentCount          *ActiveCount
	}

	NewRepositoryOptions struct {
		Logger            *zerolog.Logger
		QbittorrentClient *qbittorrent.Client
		Transmission      *transmission.Transmission
		TorrentRepository *torrent.Repository
		Provider          string
		MetadataProvider  metadata.Provider
	}

	ActiveCount struct {
		Downloading int `json:"downloading"`
		Seeding     int `json:"seeding"`
		Paused      int `json:"paused"`
	}
)

func NewRepository(opts *NewRepositoryOptions) *Repository {
	if opts.Provider == "" {
		opts.Provider = QbittorrentClient
	}
	return &Repository{
		logger:             opts.Logger,
		qBittorrentClient:  opts.QbittorrentClient,
		transmission:       opts.Transmission,
		torrentRepository:  opts.TorrentRepository,
		provider:           opts.Provider,
		metadataProvider:   opts.MetadataProvider,
		activeTorrentCount: &ActiveCount{},
	}
}

func (r *Repository) Shutdown() {
	if r.activeTorrentCountCtxCancel != nil {
		r.activeTorrentCountCtxCancel()
		r.activeTorrentCountCtxCancel = nil
	}
}

func (r *Repository) InitActiveTorrentCount(enabled bool, wsEventManager events.WSEventManagerInterface) {
	if r.activeTorrentCountCtxCancel != nil {
		r.activeTorrentCountCtxCancel()
	}

	if !enabled {
		return
	}

	var ctx context.Context
	ctx, r.activeTorrentCountCtxCancel = context.WithCancel(context.Background())
	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Second * 5)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.GetActiveCount(r.activeTorrentCount)
				wsEventManager.SendEvent(events.ActiveTorrentCountUpdated, r.activeTorrentCount)
			}
		}
	}(ctx)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (r *Repository) GetProvider() string {
	return r.provider
}

func (r *Repository) Start() bool {
	switch r.provider {
	case QbittorrentClient:
		return r.qBittorrentClient.CheckStart()
	case TransmissionClient:
		return r.transmission.CheckStart()
	case NoneClient:
		return true
	default:
		return false
	}
}
func (r *Repository) TorrentExists(hash string) bool {
	switch r.provider {
	case QbittorrentClient:
		p, err := r.qBittorrentClient.Torrent.GetProperties(hash)
		return err == nil && p != nil
	case TransmissionClient:
		torrents, err := r.transmission.Client.TorrentGetAllForHashes(context.Background(), []string{hash})
		return err == nil && len(torrents) > 0
	default:
		return false
	}
}

// GetList will return all torrents from the torrent client.
func (r *Repository) GetList() ([]*Torrent, error) {
	switch r.provider {
	case QbittorrentClient:
		torrents, err := r.qBittorrentClient.Torrent.GetList(&qbittorrent_model.GetTorrentListOptions{Filter: "all"})
		if err != nil {
			r.logger.Err(err).Msg("torrent client: Error while getting torrent list (qBittorrent)")
			return nil, err
		}
		return r.FromQbitTorrents(torrents), nil
	case TransmissionClient:
		torrents, err := r.transmission.Client.TorrentGetAll(context.Background())
		if err != nil {
			r.logger.Err(err).Msg("torrent client: Error while getting torrent list (Transmission)")
			return nil, err
		}
		return r.FromTransmissionTorrents(torrents), nil
	default:
		return nil, errors.New("torrent client: No torrent client provider found")
	}
}

// GetActiveCount will return the count of active torrents (downloading, seeding, paused).
func (r *Repository) GetActiveCount(ret *ActiveCount) {
	ret.Seeding = 0
	ret.Downloading = 0
	ret.Paused = 0
	switch r.provider {
	case QbittorrentClient:
		torrents, err := r.qBittorrentClient.Torrent.GetList(&qbittorrent_model.GetTorrentListOptions{Filter: "downloading"})
		if err != nil {
			return
		}
		torrents2, err := r.qBittorrentClient.Torrent.GetList(&qbittorrent_model.GetTorrentListOptions{Filter: "seeding"})
		if err != nil {
			return
		}
		torrents = append(torrents, torrents2...)
		for _, t := range torrents {
			switch fromQbitTorrentStatus(t.State) {
			case TorrentStatusDownloading:
				ret.Downloading++
			case TorrentStatusSeeding:
				ret.Seeding++
			case TorrentStatusPaused:
				ret.Paused++
			}
		}
	case TransmissionClient:
		torrents, err := r.transmission.Client.TorrentGet(context.Background(), []string{"id", "status", "isFinished"}, nil)
		if err != nil {
			return
		}
		for _, t := range torrents {
			if t.Status == nil || t.IsFinished == nil {
				continue
			}
			switch fromTransmissionTorrentStatus(*t.Status, *t.IsFinished) {
			case TorrentStatusDownloading:
				ret.Downloading++
			case TorrentStatusSeeding:
				ret.Seeding++
			case TorrentStatusPaused:
				ret.Paused++
			}
		}
		return
	default:
		return
	}
}

// GetActiveTorrents will return all torrents that are currently downloading, paused or seeding.
func (r *Repository) GetActiveTorrents() ([]*Torrent, error) {
	torrents, err := r.GetList()
	if err != nil {
		return nil, err
	}
	var active []*Torrent
	for _, t := range torrents {
		if t.Status == TorrentStatusDownloading || t.Status == TorrentStatusSeeding || t.Status == TorrentStatusPaused {
			active = append(active, t)
		}
	}
	return active, nil
}

func (r *Repository) AddMagnets(magnets []string, dest string) error {
	r.logger.Trace().Any("magnets", magnets).Str("dest", dest).Msg("torrent client: Adding magnets")

	if len(magnets) == 0 {
		r.logger.Debug().Msg("torrent client: No magnets to add")
		return nil
	}

	var err error
	switch r.provider {
	case QbittorrentClient:
		// Important for correct save-path behavior:
		// - UseAutoTMM=false ensures qBittorrent does NOT override the provided savepath with category rules
		// - RootFolder="true" forces creating a root folder (like manual mode "Create subfolder")
		err = r.qBittorrentClient.Torrent.AddURLs(magnets, &qbittorrent_model.AddTorrentsOptions{
			Savepath:   dest,
			Tags:       r.qBittorrentClient.Tags,
			UseAutoTMM: false,
			RootFolder: "true",
		})
	case TransmissionClient:
		for _, magnet := range magnets {
			_, err = r.transmission.Client.TorrentAdd(context.Background(), transmissionrpc.TorrentAddPayload{
				Filename:    &magnet,
				DownloadDir: &dest,
			})
			if err != nil {
				r.logger.Err(err).Msg("torrent client: Error while adding magnets (Transmission)")
				break
			}
		}
	case NoneClient:
		return errors.New("torrent client: No torrent client selected")
	}

	if err != nil {
		r.logger.Err(err).Msg("torrent client: Error while adding magnets")
		return err
	}

	r.logger.Debug().Msg("torrent client: Added torrents")

	return nil
}

// AddMagnetsWithDirAndName adds magnets and, when supported, enforces the parent directory and the desired
// root folder name for the torrent contents. For qBittorrent this uses the "rename" option so that the root
// folder matches the provided name (similar to manually renaming the torrent on add). Transmission does not
// support renaming on add via API, so it only uses the parent directory.
func (r *Repository) AddMagnetsWithDirAndName(magnets []string, parentDir string, desiredRootName string) error {
	r.logger.Trace().Any("magnets", magnets).Str("parentDir", parentDir).Str("name", desiredRootName).Msg("torrent client: Adding magnets with explicit name")

	if len(magnets) == 0 {
		r.logger.Debug().Msg("torrent client: No magnets to add")
		return nil
	}

	// Create the destination folder immediately for visibility under the completed directory
	if parentDir != "" && desiredRootName != "" {
		absPath := filepath.Join(parentDir, desiredRootName)
		if err := os.MkdirAll(absPath, 0o775); err != nil {
			r.logger.Warn().Err(err).Str("path", absPath).Msg("torrent client: Could not pre-create destination folder")
		} else {
			r.logger.Debug().Str("path", absPath).Msg("torrent client: Pre-created destination folder")
		}
	}

	var err error
	switch r.provider {
	case QbittorrentClient:
		// Save to parentDir and force a root folder named desiredRootName
		err = r.qBittorrentClient.Torrent.AddURLs(magnets, &qbittorrent_model.AddTorrentsOptions{
			Savepath:   parentDir,
			Rename:     desiredRootName,
			Tags:       r.qBittorrentClient.Tags,
			UseAutoTMM: false,
			RootFolder: "true",
		})
	case TransmissionClient:
		for _, magnet := range magnets {
			_, err = r.transmission.Client.TorrentAdd(context.Background(), transmissionrpc.TorrentAddPayload{
				Filename:    &magnet,
				DownloadDir: &parentDir,
			})
			if err != nil {
				r.logger.Err(err).Msg("torrent client: Error while adding magnets (Transmission)")
				break
			}
		}
	case NoneClient:
		return errors.New("torrent client: No torrent client selected")
	}

	if err != nil {
		r.logger.Err(err).Msg("torrent client: Error while adding magnets with name")
		return err
	}

	r.logger.Debug().Msg("torrent client: Added torrents with explicit name")
	return nil
}

func (r *Repository) RemoveTorrents(hashes []string) error {
	r.logger.Trace().Msg("torrent client: Removing torrents")

	var err error
	switch r.provider {
	case QbittorrentClient:
		err = r.qBittorrentClient.Torrent.DeleteTorrents(hashes, true)
	case TransmissionClient:
		torrents, err := r.transmission.Client.TorrentGetAllForHashes(context.Background(), hashes)
		if err != nil {
			r.logger.Err(err).Msg("torrent client: Error while fetching torrents (Transmission)")
			return err
		}
		ids := make([]int64, len(torrents))
		for i, t := range torrents {
			ids[i] = *t.ID
		}
		err = r.transmission.Client.TorrentRemove(context.Background(), transmissionrpc.TorrentRemovePayload{
			IDs:             ids,
			DeleteLocalData: true,
		})
		if err != nil {
			r.logger.Err(err).Msg("torrent client: Error while removing torrents (Transmission)")
			return err
		}
	}
	if err != nil {
		r.logger.Err(err).Msg("torrent client: Error while removing torrents")
		return err
	}

	r.logger.Debug().Any("hashes", hashes).Msg("torrent client: Removed torrents")
	return nil
}

func (r *Repository) PauseTorrents(hashes []string) error {
	r.logger.Trace().Msg("torrent client: Pausing torrents")

	var err error
	switch r.provider {
	case QbittorrentClient:
		err = r.qBittorrentClient.Torrent.StopTorrents(hashes)
	case TransmissionClient:
		err = r.transmission.Client.TorrentStopHashes(context.Background(), hashes)
	}

	if err != nil {
		r.logger.Err(err).Msg("torrent client: Error while pausing torrents")
		return err
	}

	r.logger.Debug().Any("hashes", hashes).Msg("torrent client: Paused torrents")

	return nil
}

func (r *Repository) ResumeTorrents(hashes []string) error {
	r.logger.Trace().Msg("torrent client: Resuming torrents")

	var err error
	switch r.provider {
	case QbittorrentClient:
		err = r.qBittorrentClient.Torrent.ResumeTorrents(hashes)
	case TransmissionClient:
		err = r.transmission.Client.TorrentStartHashes(context.Background(), hashes)
	}

	if err != nil {
		r.logger.Err(err).Msg("torrent client: Error while resuming torrents")
		return err
	}

	r.logger.Debug().Any("hashes", hashes).Msg("torrent client: Resumed torrents")

	return nil
}

func (r *Repository) DeselectFiles(hash string, indices []int) error {

	var err error
	switch r.provider {
	case QbittorrentClient:
		strIndices := make([]string, len(indices), len(indices))
		for i, v := range indices {
			strIndices[i] = strconv.Itoa(v)
		}
		err = r.qBittorrentClient.Torrent.SetFilePriorities(hash, strIndices, 0)
	case TransmissionClient:
		torrents, err := r.transmission.Client.TorrentGetAllForHashes(context.Background(), []string{hash})
		if err != nil || torrents[0].ID == nil {
			r.logger.Err(err).Msg("torrent client: Error while deselecting files (Transmission)")
			return err
		}
		id := *torrents[0].ID
		ind := make([]int64, len(indices), len(indices))
		for i, v := range indices {
			ind[i] = int64(v)
		}
		err = r.transmission.Client.TorrentSet(context.Background(), transmissionrpc.TorrentSetPayload{
			FilesUnwanted: ind,
			IDs:           []int64{id},
		})
	}

	if err != nil {
		r.logger.Err(err).Msg("torrent client: Error while deselecting files")
		return err
	}

	r.logger.Debug().Str("hash", hash).Any("indices", indices).Msg("torrent client: Deselected torrent files")

	return nil
}

// GetFiles blocks until the files are retrieved, or until timeout.
func (r *Repository) GetFiles(hash string) (filenames []string, err error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	filenames = make([]string, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	done := make(chan struct{})

	go func() {
		r.logger.Debug().Str("hash", hash).Msg("torrent client: Getting torrent files")
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				err = errors.New("torrent client: Unable to retrieve torrent files (timeout)")
				return
			case <-ticker.C:
				switch r.provider {
				case QbittorrentClient:
					qbitFiles, err := r.qBittorrentClient.Torrent.GetContents(hash)
					if err == nil && qbitFiles != nil && len(qbitFiles) > 0 {
						r.logger.Debug().Str("hash", hash).Int("count", len(qbitFiles)).Msg("torrent client: Retrieved torrent files")
						for _, f := range qbitFiles {
							filenames = append(filenames, f.Name)
						}
						return
					}
				case TransmissionClient:
					torrents, err := r.transmission.Client.TorrentGetAllForHashes(context.Background(), []string{hash})
					if err == nil && len(torrents) > 0 && torrents[0].Files != nil && len(torrents[0].Files) > 0 {
						transmissionFiles := torrents[0].Files
						r.logger.Debug().Str("hash", hash).Int("count", len(transmissionFiles)).Msg("torrent client: Retrieved torrent files")
						for _, f := range transmissionFiles {
							filenames = append(filenames, f.Name)
						}
						return
					}
				}
			}
		}
	}()

	<-done // wait for the files to be retrieved

	return
}

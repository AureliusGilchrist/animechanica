//go:build disabled
// +build disabled

package anime

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"net/http"
	"seanime/internal/database/db"
	"seanime/internal/database/models"
	"seanime/internal/events"
	"seanime/internal/extension"
	"seanime/internal/util/filecache"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	_ "golang.org/x/image/bmp"  // Register BMP format
	_ "golang.org/x/image/tiff" // Register Tiff format
	_ "golang.org/x/image/webp" // Register WebP format
)

var (
	ErrNoResults            = errors.New("no results found for this media")
	ErrNoEpisodes           = errors.New("no anime episodes found")
	ErrEpisodeNotFound      = errors.New("episode not found")
	ErrEpisodeNotDownloaded = errors.New("episode not downloaded")
	ErrNoTitlesProvided     = errors.New("no titles provided")
)

type (
	Repository struct {
		logger                *zerolog.Logger
		fileCacher            *filecache.Cacher
		cacheDir              string
		providerExtensionBank *extension.UnifiedBank
		serverUri             string
		wsEventManager        events.WSEventManagerInterface
		mu                    sync.Mutex
		downloadDir           string
		db                    *db.Database

		settings *models.Settings
	}

	NewRepositoryOptions struct {
		Logger         *zerolog.Logger
		FileCacher     *filecache.Cacher
		CacheDir       string
		ServerUri      string
		WSEventManager events.WSEventManagerInterface
		DownloadDir    string
		Database       *db.Database
	}
)

func NewRepository(opts *NewRepositoryOptions) *Repository {
	return &Repository{
		logger:          opts.Logger,
		fileCacher:      opts.FileCacher,
		cacheDir:        opts.CacheDir,
		serverUri:       opts.ServerUri,
		wsEventManager:  opts.WSEventManager,
		downloadDir:     opts.DownloadDir,
		db:              opts.Database,
	}
}

func (r *Repository) SetSettings(settings *models.Settings) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings = settings
}

func (r *Repository) InitExtensionBank(bank *extension.UnifiedBank) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providerExtensionBank = bank
}

func (r *Repository) RemoveProvider(id string) {
	// Implementation for removing anime provider
}

func (r *Repository) GetProviderExtensionBank() *extension.UnifiedBank {
	return r.providerExtensionBank
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// File Cache
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type bucketType string

const (
	bucketTypeEpisodeKey                = "1"
	bucketTypeEpisode        bucketType = "episodes"
	bucketTypeVideo          bucketType = "videos"
	bucketTypeSubtitle       bucketType = "subtitles"
)

// getFcProviderBucket returns a bucket for the provider and mediaId.
//
//	e.g., anime_crunchyroll_episodes_123, anime_funimation_videos_456
//
// Note: Each bucket contains only 1 key-value pair.
func (r *Repository) getFcProviderBucket(provider string, mediaId int, bucketType bucketType) filecache.Bucket {
	return r.fileCacher.Bucket(
		"anime_" + provider + "_" + string(bucketType) + "_" + strconv.Itoa(mediaId),
	)
}

// EmptyAnimeCache deletes all anime buckets associated with the given mediaId.
func (r *Repository) EmptyAnimeCache(mediaId int) (err error) {
	buckets, err := r.fileCacher.GetBucketsWithSuffix("_" + strconv.Itoa(mediaId))
	if err != nil {
		return err
	}
	for _, bucket := range buckets {
		_ = bucket.Clear()
	}
	return nil
}

func ParseEpisodeContainerFileName(filename string) (provider string, bucketType bucketType, mediaId int, ok bool) {
	// anime_crunchyroll_episodes_123
	parts := strings.Split(filename, "_")
	if len(parts) != 4 {
		return "", "", 0, false
	}

	if parts[0] != "anime" {
		return "", "", 0, false
	}

	provider = parts[1]
	bucketType = bucketType(parts[2])
	mediaIdStr := parts[3]

	mediaId, err := strconv.Atoi(mediaIdStr)
	if err != nil {
		return "", "", 0, false
	}

	return provider, bucketType, mediaId, true
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func getVideoNaturalSize(url string) (int, int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return 0, 0, err
	}

	return getVideoNaturalSizeB(buf.Bytes())
}

func getVideoNaturalSizeB(data []byte) (int, int, error) {
	img, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return img.Width, img.Height, nil
}

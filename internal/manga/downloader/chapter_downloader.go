package chapter_downloader

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"seanime/internal/database/db"
	"seanime/internal/events"
	hibikemanga "seanime/internal/extension/hibike/manga"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/util"
	"strconv"
	"strings"
	"sync"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
	_ "golang.org/x/image/bmp"  // Register BMP format
	_ "golang.org/x/image/tiff" // Register Tiff format
)

// 📁 /aeternae/library/manga/seanime/
// └── 📁 {SERIES_TITLE}/                                    <- Sanitized manga title
//     └── 📁 {CHAPTER_NUMBER} - {CHAPTER_TITLE}/         <- Chapter folder
//         ├── 📄 registry.json						        <- Contains Registry with metadata
//         ├── 📄 1.jpg
//         ├── 📄 2.jpg
//         └── 📄 ...
//

type (
	// Downloader is used to download chapters from various manga providers.
	Downloader struct {
		logger         *zerolog.Logger
		wsEventManager events.WSEventManagerInterface
		database       *db.Database
		downloadDir    string
		mu             sync.Mutex
		downloadMu     sync.Mutex
		// cancelChannel is used to cancel some or all downloads.
		cancelChannels      map[DownloadID]chan struct{}
		queue               *Queue
		cancelCh            chan struct{}   // Close to cancel the download process
		runCh               chan *QueueInfo // Receives a signal to download the next item
		chapterDownloadedCh chan DownloadID // Sends a signal when a chapter has been downloaded
	}

	//+-------------------------------------------------------------------------------------------------------------------+

	DownloadID struct {
		Provider      string `json:"provider"`
		MediaId       int    `json:"mediaId"`
		ChapterId     string `json:"chapterId"`
		ChapterNumber string `json:"chapterNumber"`
		SeriesTitle   string `json:"seriesTitle"`
		ChapterTitle  string `json:"chapterTitle"`
		CoverImageUrl string `json:"coverImageUrl,omitempty"` // Cover image URL from search results
	}

	//+-------------------------------------------------------------------------------------------------------------------+

	// Registry stored in 📄 registry.json for each chapter download.
	// Now includes both page information and download metadata
	Registry struct {
		DownloadMetadata DownloadID       `json:"download_metadata"`
		Pages            map[int]PageInfo `json:"pages"`
	}

	PageInfo struct {
		Index       int    `json:"index"`
		Filename    string `json:"filename"`
		OriginalURL string `json:"original_url"`
		Size        int64  `json:"size"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
	}
)

type (
	NewDownloaderOptions struct {
		Logger         *zerolog.Logger
		WSEventManager events.WSEventManagerInterface
		DownloadDir    string
		Database       *db.Database
	}

	DownloadOptions struct {
		DownloadID
		Pages    []*hibikemanga.ChapterPage
		StartNow bool
	}
)

func NewDownloader(opts *NewDownloaderOptions) *Downloader {
	runCh := make(chan *QueueInfo, 1)

	d := &Downloader{
		logger:              opts.Logger,
		wsEventManager:      opts.WSEventManager,
		downloadDir:         opts.DownloadDir,
		cancelChannels:      make(map[DownloadID]chan struct{}),
		runCh:               runCh,
		queue:               NewQueue(opts.Database, opts.Logger, opts.WSEventManager, runCh),
		chapterDownloadedCh: make(chan DownloadID, 100),
	}

	return d
}

// Start spins up a goroutine that will listen to queue events.
func (cd *Downloader) Start() {
	go func() {
		for {
			select {
			// Listen for new queue items
			case queueInfo := <-cd.runCh:
				cd.logger.Debug().Msgf("chapter downloader: Received queue item to download: %s", queueInfo.ChapterId)
				cd.run(queueInfo)
			}
		}
	}()
}

func (cd *Downloader) ChapterDownloaded() <-chan DownloadID {
	return cd.chapterDownloadedCh
}

// AddToQueue adds a chapter to the download queue.
// If the chapter is already downloaded (i.e. a folder already exists), it will delete the previous data and re-download it.
func (cd *Downloader) AddToQueue(opts DownloadOptions) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	downloadId := opts.DownloadID

	// Check if chapter is already downloaded
	registryPath := cd.getChapterRegistryPath(downloadId)
	if _, err := os.Stat(registryPath); err == nil {
		cd.logger.Warn().Msg("chapter downloader: directory already exists, deleting")
		// Delete folder
		_ = os.RemoveAll(cd.getChapterDownloadDir(downloadId))
	}

	// Start download
	cd.logger.Debug().Msgf("chapter downloader: Adding chapter to download queue: %s", opts.ChapterId)
	// Add to queue
	return cd.queue.Add(downloadId, opts.Pages, opts.StartNow)
}

// DeleteChapter deletes a chapter directory from the download directory.
func (cd *Downloader) DeleteChapter(id DownloadID) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	cd.logger.Debug().Msgf("chapter downloader: Deleting chapter %s", id.ChapterId)

	_ = os.RemoveAll(cd.getChapterDownloadDir(id))
	cd.logger.Debug().Msgf("chapter downloader: Removed chapter %s", id.ChapterId)
	return nil
}

// Run starts the downloader if it's not already running.
func (cd *Downloader) Run() {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	cd.logger.Debug().Msg("chapter downloader: Starting queue")

	cd.cancelCh = make(chan struct{})

	cd.queue.Run()
}

// Stop cancels the download process and stops the queue from running.
func (cd *Downloader) Stop() {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			cd.logger.Error().Msgf("chapter downloader: cancelCh is already closed")
		}
	}()

	cd.cancelCh = make(chan struct{})

	close(cd.cancelCh) // Cancel download process

	cd.queue.Stop()
}

// run downloads the chapter based on the QueueInfo provided.
// This is called successively for each current item being processed.
// It invokes downloadChapterImages to download the chapter pages.
func (cd *Downloader) run(queueInfo *QueueInfo) {

	defer util.HandlePanicInModuleThen("internal/manga/downloader/runNext", func() {
		cd.logger.Error().Msg("chapter downloader: Panic in 'run'")
	})

	// Download chapter images
	if err := cd.downloadChapterImages(queueInfo); err != nil {
		return
	}

	cd.chapterDownloadedCh <- queueInfo.DownloadID
}

// downloadChapterImages creates a directory for the chapter and downloads each image to that directory.
// It also creates a Registry file that contains information about each image.
//
//	e.g.,
//	📁 {provider}_{mediaId}_{chapterId}_{chapterNumber}
//	   ├── 📄 registry.json
//	   ├── 📄 1.jpg
//	   ├── 📄 2.jpg
//	   └── 📄 ...
func (cd *Downloader) downloadChapterImages(queueInfo *QueueInfo) (err error) {
    // Ensure the queue progresses even if we hit an early return due to an error
    handledCompletion := false
    defer func() {
        if err != nil && !handledCompletion {
            queueInfo.Status = QueueStatusErrored
            cd.queue.HasCompleted(queueInfo)
        }
    }()
    // Use a temporary directory to avoid creating final series folder unless the download succeeds
    // Final destination: {DOWNLOAD_DIR}/{SERIES}/{CHAPTERNAME}
    destination := cd.getChapterDownloadDir(queueInfo.DownloadID)
    // Temporary destination: {DOWNLOAD_DIR}/.tmp/{SERIES}/{CHAPTERNAME}
    tmpRoot := filepath.Join(cd.downloadDir, ".tmp")
    tmpDestination := filepath.Join(tmpRoot, FormatNewChapterDirName(queueInfo.SeriesTitle, queueInfo.ChapterNumber, queueInfo.ChapterTitle))

    // Ensure any previous temp directory for this chapter is removed
    _ = os.RemoveAll(tmpDestination)

    // Create temporary download directory
    if err = os.MkdirAll(tmpDestination, os.ModePerm); err != nil {
        cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to create temp download directory for chapter %s", queueInfo.ChapterId)
        return err
    }

    cd.logger.Debug().Msgf("chapter downloader: Downloading chapter %s images to temp %s", queueInfo.ChapterId, tmpDestination)

	registry := Registry{
		DownloadMetadata: queueInfo.DownloadID,
		Pages:            make(map[int]PageInfo),
	}

	// calculateBatchSize calculates the batch size based on the number of URLs.
	calculateBatchSize := func(numURLs int) int {
		maxBatchSize := 5
		batchSize := numURLs / 10
		if batchSize < 1 {
			return 1
		} else if batchSize > maxBatchSize {
			return maxBatchSize
		}
		return batchSize
	}

	// Download images
	batchSize := calculateBatchSize(len(queueInfo.Pages))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, batchSize) // Semaphore to control concurrency
	for _, page := range queueInfo.Pages {
		semaphore <- struct{}{} // Acquire semaphore
		wg.Add(1)
		go func(page *hibikemanga.ChapterPage, registry *Registry) {
			defer func() {
				<-semaphore // Release semaphore
				wg.Done()
			}()
			select {
			case <-cd.cancelCh:
				//cd.logger.Warn().Msg("chapter downloader: Download goroutine canceled")
				return
			default:
				cd.downloadPage(page, tmpDestination, registry)
			}
		}(page, &registry)
	}
	wg.Wait()

	// Write the registry
	saveErr := registry.save(queueInfo, tmpDestination, cd.logger)

	handledCompletion = true
	cd.queue.HasCompleted(queueInfo)

	if queueInfo.Status == QueueStatusErrored || saveErr != nil {
		// On error, ensure temp directory is cleaned up
		_ = os.RemoveAll(tmpDestination)
		return fmt.Errorf("chapter downloader: Failed to download chapter %s", queueInfo.ChapterId)
	}

	// Success: move from temp to final destination. Ensure final parent exists.
	finalParent := filepath.Dir(destination)
	if err = os.MkdirAll(finalParent, os.ModePerm); err != nil {
		// If we fail to ensure final parent, clean up temp and return error
		_ = os.RemoveAll(tmpDestination)
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to create final parent directory for chapter %s", queueInfo.ChapterId)
		return err
	}

	// Remove any existing destination (shouldn't normally exist for fresh download)
	_ = os.RemoveAll(destination)
	if err = os.Rename(tmpDestination, destination); err != nil {
		// If rename fails, attempt to clean up temp
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to move chapter %s from temp to final destination", queueInfo.ChapterId)
		_ = os.RemoveAll(tmpDestination)
		return err
	}

	cd.logger.Info().Msgf("chapter downloader: Finished downloading chapter %s", queueInfo.ChapterId)
	return nil
}

// downloadPage downloads a single page from the URL and saves it to the destination directory.
// It also updates the Registry with the page information.
func (cd *Downloader) downloadPage(page *hibikemanga.ChapterPage, destination string, registry *Registry) {

	defer util.HandlePanicInModuleThen("manga/downloader/downloadImage", func() {
	})

	// Download image from URL

	imgID := fmt.Sprintf("%02d", page.Index+1)

	buf, err := manga_providers.GetImageByProxy(page.URL, page.Headers)
	if err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to get image from URL %s", page.URL)
		return
	}

	// Get the image format
	config, format, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to decode image format from URL %s", page.URL)
		return
	}

	filename := imgID + "." + format

	// Create the file
	filePath := filepath.Join(destination, filename)
	file, err := os.Create(filePath)
	if err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to create file for image %s", imgID)
		return
	}
	defer file.Close()

	// Copy the image data to the file
	_, err = io.Copy(file, bytes.NewReader(buf))
	if err != nil {
		cd.logger.Error().Err(err).Msgf("image downloader: Failed to write image data to file for image from %s", page.URL)
		return
	}

	// Update registry
	cd.downloadMu.Lock()
	registry.Pages[page.Index] = PageInfo{
		Index:       page.Index,
		Width:       config.Width,
		Height:      config.Height,
		Filename:    filename,
		OriginalURL: page.URL,
		Size:        int64(len(buf)),
	}
	cd.downloadMu.Unlock()

	return
}

////////////////////////

// save saves the Registry content to a file in the chapter directory.
func (r *Registry) save(queueInfo *QueueInfo, destination string, logger *zerolog.Logger) (err error) {

	defer util.HandlePanicInModuleThen("manga/downloader/save", func() {
		err = fmt.Errorf("chapter downloader: Failed to save registry content")
	})

	// Verify all images have been downloaded
	allDownloaded := true
	for _, page := range queueInfo.Pages {
		if _, ok := r.Pages[page.Index]; !ok {
			allDownloaded = false
			break
		}
	}

	if !allDownloaded {
        // Allow partial downloads: keep whatever pages succeeded and continue
        // We still write the registry for the downloaded subset so the reader can open it
        logger.Warn().Msg("chapter downloader: Not all images downloaded; saving partial chapter")
        // Do NOT mark as errored; proceed to write registry and finalize
    }

	// Create registry file
	var data []byte
	data, err = json.Marshal(*r)
	if err != nil {
		return err
	}

	registryFilePath := filepath.Join(destination, "registry.json")
	err = os.WriteFile(registryFilePath, data, 0644)
	if err != nil {
		return err
	}

	return
}

func (cd *Downloader) getChapterDownloadDir(downloadId DownloadID) string {
	// Use the new {SERIES}/{CHAPTERNAME} format
	cd.logger.Debug().Str("seriesTitle", downloadId.SeriesTitle).Str("chapterNumber", downloadId.ChapterNumber).Str("chapterTitle", downloadId.ChapterTitle).Msg("chapter downloader: Creating download directory path")
	dirPath := filepath.Join(cd.downloadDir, FormatNewChapterDirName(downloadId.SeriesTitle, downloadId.ChapterNumber, downloadId.ChapterTitle))
	cd.logger.Debug().Str("fullPath", dirPath).Msg("chapter downloader: Full download directory path")
	return dirPath
}

// FormatChapterDirName - Legacy function for backward compatibility
func FormatChapterDirName(provider string, mediaId int, chapterId string, chapterNumber string) string {
	return fmt.Sprintf("%s_%d_%s_%s", provider, mediaId, EscapeChapterID(chapterId), chapterNumber)
}

// SanitizeForFilesystem sanitizes a string to be safe for filesystem use
func SanitizeForFilesystem(name string) string {
	if name == "" {
		return "Unknown"
	}

	// Remove or replace problematic characters
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	name = re.ReplaceAllString(name, "")

	// Replace multiple spaces with single space
	re = regexp.MustCompile(`\s+`)
	name = re.ReplaceAllString(name, " ")

	// Trim spaces and dots from the end (Windows doesn't like trailing dots)
	name = strings.TrimRight(name, ". ")

	// Ensure it's not empty after sanitization
	if name == "" {
		return "Unknown"
	}

	// Limit length to avoid filesystem issues
	if len(name) > 200 {
		name = name[:200]
		name = strings.TrimRight(name, ". ")
	}

	return name
}

// FormatNewChapterDirName creates the new directory structure: {SERIES}/{CHAPTERNAME}
func FormatNewChapterDirName(seriesTitle, chapterNumber, chapterTitle string) string {
	sanitizedSeries := SanitizeForFilesystem(seriesTitle)
	sanitizedChapterTitle := SanitizeForFilesystem(chapterTitle)

	// Create chapter folder name: "Chapter 1 - Title" or just "Chapter 1" if no title
	var chapterFolderName string
	if sanitizedChapterTitle != "" {
		chapterFolderName = fmt.Sprintf("%s - %s", chapterNumber, sanitizedChapterTitle)
	} else {
		chapterFolderName = chapterNumber
	}

	return filepath.Join(sanitizedSeries, chapterFolderName)
}

// ParseChapterDirName parses a chapter directory name and returns the DownloadID.
// e.g. comick_1234_chapter$UNDERSCORE$id_13.5 -> {Provider: "comick", MediaId: 1234, ChapterId: "chapter_id", ChapterNumber: "13.5"}
func ParseChapterDirName(dirName string) (id DownloadID, ok bool) {
	parts := strings.Split(dirName, "_")
	if len(parts) != 4 {
		return id, false
	}

	id.Provider = parts[0]
	var err error
	id.MediaId, err = strconv.Atoi(parts[1])
	if err != nil {
		return id, false
	}
	id.ChapterId = UnescapeChapterID(parts[2])
	id.ChapterNumber = parts[3]

	ok = true
	return
}

func EscapeChapterID(id string) string {
	// Replace underscores with a placeholder to avoid conflicts with the delimiter
	id = strings.ReplaceAll(id, "_", "$UNDERSCORE$")
	// Replace forward slashes with a placeholder
	id = strings.ReplaceAll(id, "/", "$SLASH$")
	// Replace backslashes with a placeholder
	id = strings.ReplaceAll(id, "\\", "$BACKSLASH$")
	// Replace colons with a placeholder
	id = strings.ReplaceAll(id, ":", "$COLON$")
	// Replace asterisks with a placeholder
	id = strings.ReplaceAll(id, "*", "$ASTERISK$")
	// Replace question marks with a placeholder
	id = strings.ReplaceAll(id, "?", "$QUESTION$")
	// Replace quotes with a placeholder
	id = strings.ReplaceAll(id, "\"", "$QUOTE$")
	// Replace less than with a placeholder
	id = strings.ReplaceAll(id, "<", "$LESS$")
	// Replace greater than with a placeholder
	id = strings.ReplaceAll(id, ">", "$GREATER$")
	// Replace pipe with a placeholder
	id = strings.ReplaceAll(id, "|", "$PIPE$")
	return id
}

func UnescapeChapterID(id string) string {
	id = strings.ReplaceAll(id, "$SLASH$", "/")
	id = strings.ReplaceAll(id, "$BACKSLASH$", "\\")
	id = strings.ReplaceAll(id, "$COLON$", ":")
	id = strings.ReplaceAll(id, "$ASTERISK$", "*")
	id = strings.ReplaceAll(id, "$QUESTION$", "?")
	id = strings.ReplaceAll(id, "$QUOTE$", "\"")
	id = strings.ReplaceAll(id, "$LT$", "<")
	id = strings.ReplaceAll(id, "$GT$", ">")
	id = strings.ReplaceAll(id, "$PIPE$", "|")
	id = strings.ReplaceAll(id, "$DOT$", ".")
	id = strings.ReplaceAll(id, "$SPACE$", " ")
	id = strings.ReplaceAll(id, "$UNDERSCORE$", "_")
	return id
}

func (cd *Downloader) getChapterRegistryPath(downloadId DownloadID) string {
	return filepath.Join(cd.getChapterDownloadDir(downloadId), "registry.json")
}

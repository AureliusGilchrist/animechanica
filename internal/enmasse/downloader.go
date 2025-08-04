package enmasse

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"seanime/internal/events"
	"seanime/internal/manga"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// WeebCentralCatalogueEntry represents a single manga entry from the WeebCentral catalogue
type WeebCentralCatalogueEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// KitsuMangaSearchResult represents the response from Kitsu API search
type KitsuMangaSearchResult struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			CanonicalTitle string `json:"canonicalTitle"`
			Titles         struct {
				En   string `json:"en"`
				EnJp string `json:"en_jp"`
				JaJp string `json:"ja_jp"`
			} `json:"titles"`
			Synopsis    string `json:"synopsis"`
			PosterImage struct {
				Tiny   string `json:"tiny"`
				Small  string `json:"small"`
				Medium string `json:"medium"`
				Large  string `json:"large"`
			} `json:"posterImage"`
		} `json:"attributes"`
	} `json:"data"`
}

// Downloader manages the bulk download process for WeebCentral manga
type Downloader struct {
	mu                sync.RWMutex
	isRunning         bool
	isPaused          bool
	currentSeries     string
	processedCount    int
	totalCount        int
	errorCount        int
	status            string
	cataloguePath     string
	kitsuRateLimit    time.Duration
	seriesDelay       time.Duration
	lastKitsuCall     time.Time
	kitsuMutex        sync.Mutex
	processedSeries   []string
	errorSeries       []string
	logger            *zerolog.Logger
	wsEventManager    *events.WSEventManager
	mangaDownloader   *manga.Downloader
	mangaRepository   *manga.Repository
}

// NewDownloader creates a new instance of the En Masse Downloader
func NewDownloader(cataloguePath string, logger *zerolog.Logger, wsEventManager *events.WSEventManager, mangaDownloader *manga.Downloader, mangaRepository *manga.Repository) *Downloader {
	return &Downloader{
		cataloguePath:   cataloguePath,
		kitsuRateLimit:  time.Second * 2, // Kitsu API rate limit for non-logged in accounts
		seriesDelay:     time.Second * 3,  // 3 second delay between series as requested
		status:          "idle",
		logger:          logger,
		wsEventManager:  wsEventManager,
		mangaDownloader: mangaDownloader,
		mangaRepository: mangaRepository,
	}
}

// Start begins the en masse download process
func (d *Downloader) Start() {
	d.mu.Lock()
	if d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = true
	d.isPaused = false
	d.status = "running"
	d.processedCount = 0
	d.errorCount = 0
	d.processedSeries = []string{}
	d.errorSeries = []string{}
	d.mu.Unlock()

	d.wsEventManager.SendEvent(events.InfoToast, "Starting En Masse Download process...")

	// Load WeebCentral catalogue
	catalogue, err := d.loadCatalogue()
	if err != nil {
		d.mu.Lock()
		d.status = "error"
		d.isRunning = false
		d.mu.Unlock()
		d.wsEventManager.SendEvent(events.ErrorToast, fmt.Sprintf("Failed to load catalogue: %v", err))
		return
	}

	d.mu.Lock()
	d.totalCount = len(catalogue)
	d.mu.Unlock()

	d.wsEventManager.SendEvent(events.InfoToast, fmt.Sprintf("Loaded %d manga series from catalogue", len(catalogue)))

	// Process each series sequentially
	for i, entry := range catalogue {
		d.mu.RLock()
		if !d.isRunning {
			d.mu.RUnlock()
			break
		}
		
		// Handle pause
		for d.isPaused && d.isRunning {
			d.mu.RUnlock()
			time.Sleep(time.Second)
			d.mu.RLock()
		}
		
		if !d.isRunning {
			d.mu.RUnlock()
			break
		}
		d.mu.RUnlock()

		d.mu.Lock()
		d.currentSeries = entry.Title
		d.mu.Unlock()

		d.wsEventManager.SendEvent(events.InfoToast, fmt.Sprintf("Processing series %d/%d: %s", i+1, len(catalogue), entry.Title))

		err := d.processSeries(entry)
		if err != nil {
			d.mu.Lock()
			d.errorCount++
			d.errorSeries = append(d.errorSeries, fmt.Sprintf("%s: %v", entry.Title, err))
			d.mu.Unlock()
			d.logger.Debug().Str("series", entry.Title).Msg(fmt.Sprintf("Failed to process series: %v", err))
		} else {
			d.mu.Lock()
			d.processedSeries = append(d.processedSeries, entry.Title)
			d.mu.Unlock()
		}

		d.mu.Lock()
		d.processedCount++
		d.mu.Unlock()

		// 3-second delay between series as requested
		if i < len(catalogue)-1 {
			time.Sleep(d.seriesDelay)
		}
	}

	d.mu.Lock()
	d.isRunning = false
	d.status = "completed"
	d.currentSeries = ""
	d.mu.Unlock()

	d.wsEventManager.SendEvent(events.InfoToast, fmt.Sprintf("En Masse Download completed! Processed: %d, Errors: %d", d.processedCount, d.errorCount))
}

// Pause pauses the download process
func (d *Downloader) Pause() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.isRunning {
		d.isPaused = true
		d.status = "paused"
		d.wsEventManager.SendEvent(events.InfoToast, "En Masse Download paused")
	}
}

// Resume resumes the download process
func (d *Downloader) Resume() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.isRunning && d.isPaused {
		d.isPaused = false
		d.status = "running"
		d.wsEventManager.SendEvent(events.InfoToast, "En Masse Download resumed")
	}
}

// Stop stops the download process
func (d *Downloader) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.isRunning = false
	d.isPaused = false
	d.status = "stopped"
	d.currentSeries = ""
	d.wsEventManager.SendEvent(events.InfoToast, "En Masse Download stopped")
}

// IsRunning returns whether the download process is currently running
func (d *Downloader) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isRunning
}

// GetStatus returns the current status of the download process
func (d *Downloader) GetStatus() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	return map[string]interface{}{
		"isRunning":       d.isRunning,
		"isPaused":        d.isPaused,
		"status":          d.status,
		"currentSeries":   d.currentSeries,
		"processedCount":  d.processedCount,
		"totalCount":      d.totalCount,
		"errorCount":      d.errorCount,
		"processedSeries": d.processedSeries,
		"errorSeries":     d.errorSeries,
	}
}

// loadCatalogue loads the WeebCentral catalogue from the JSON file
func (d *Downloader) loadCatalogue() ([]WeebCentralCatalogueEntry, error) {
	data, err := ioutil.ReadFile(d.cataloguePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read catalogue file: %w", err)
	}

	var catalogue []WeebCentralCatalogueEntry
	err = json.Unmarshal(data, &catalogue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse catalogue JSON: %w", err)
	}

	return catalogue, nil
}

// processSeries processes a single manga series
func (d *Downloader) processSeries(entry WeebCentralCatalogueEntry) error {
	// Search Kitsu API for the manga
	kitsuID, err := d.searchKitsuManga(entry.Title)
	if err != nil {
		return fmt.Errorf("failed to search Kitsu for '%s': %w", entry.Title, err)
	}

	if kitsuID == "" {
		return fmt.Errorf("no Kitsu results found for '%s'", entry.Title)
	}

	// Convert Kitsu ID to negative integer for En Masse Download ID
	kitsuIDInt, err := strconv.Atoi(kitsuID)
	if err != nil {
		return fmt.Errorf("invalid Kitsu ID '%s': %w", kitsuID, err)
	}

	// Make it negative to distinguish from AniList IDs
	enMasseID := -kitsuIDInt

	// Queue the manga for download using existing scan and queue system
	err = d.queueMangaForDownload(enMasseID, entry.Title, entry.ID)
	if err != nil {
		return fmt.Errorf("failed to queue manga for download: %w", err)
	}

	d.logger.Debug().Str("title", entry.Title).Msg(fmt.Sprintf("Successfully queued manga with En Masse ID: %d", enMasseID))
	return nil
}

// WeebCentralChapter represents a chapter from WeebCentral
type WeebCentralChapter struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Number string `json:"number"`
}

// findAvailableChapters finds available chapters for a manga from WeebCentral provider
func (d *Downloader) findAvailableChapters(enMasseID int, title, weebCentralID string) ([]WeebCentralChapter, error) {
	// For now, we'll create mock chapters since we need to integrate with the actual WeebCentral provider
	// In a real implementation, this would query the WeebCentral API to get available chapters
	
	// Create mock chapters (1-10) for testing purposes
	// This simulates finding the first 10 chapters of a manga series
	chapters := make([]WeebCentralChapter, 0, 10)
	for i := 1; i <= 10; i++ {
		chapters = append(chapters, WeebCentralChapter{
			ID:     fmt.Sprintf("%s_chapter_%d", weebCentralID, i),
			Title:  fmt.Sprintf("Chapter %d", i),
			Number: fmt.Sprintf("%d", i),
		})
	}
	
	d.logger.Debug().Str("title", title).Int("chapterCount", len(chapters)).Msg("Found chapters for manga")
	return chapters, nil
}

// searchKitsuManga searches Kitsu API for a manga by title
func (d *Downloader) searchKitsuManga(title string) (string, error) {
	// Apply rate limiting
	d.kitsuMutex.Lock()
	timeSinceLastCall := time.Since(d.lastKitsuCall)
	if timeSinceLastCall < d.kitsuRateLimit {
		time.Sleep(d.kitsuRateLimit - timeSinceLastCall)
	}
	d.lastKitsuCall = time.Now()
	d.kitsuMutex.Unlock()

	// Clean and encode the title for search
	cleanTitle := strings.TrimSpace(title)
	encodedTitle := url.QueryEscape(cleanTitle)

	// Kitsu API search endpoint
	searchURL := fmt.Sprintf("https://kitsu.io/api/edge/manga?filter[text]=%s&page[limit]=5", encodedTitle)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("User-Agent", "Seanime/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Kitsu API returned status %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var searchResult KitsuMangaSearchResult
	err = json.Unmarshal(body, &searchResult)
	if err != nil {
		return "", fmt.Errorf("failed to parse search result: %w", err)
	}

	if len(searchResult.Data) == 0 {
		return "", nil // No results found
	}

	// Return the first result's ID
	return searchResult.Data[0].ID, nil
}

// queueMangaForDownload queues a manga for download using the existing system
func (d *Downloader) queueMangaForDownload(enMasseID int, title, weebCentralID string) error {
	// Check if we should pause or stop
	d.mu.RLock()
	if !d.isRunning {
		d.mu.RUnlock()
		return fmt.Errorf("download process stopped")
	}
	for d.isPaused {
		d.mu.RUnlock()
		time.Sleep(500 * time.Millisecond)
		d.mu.RLock()
		if !d.isRunning {
			d.mu.RUnlock()
			return fmt.Errorf("download process stopped")
		}
	}
	d.mu.RUnlock()

	d.logger.Info().Str("title", title).Int("enMasseID", enMasseID).Str("weebCentralID", weebCentralID).Msg("Scanning manga for chapter metadata")

	// Step 1: Use the existing manga system to scan and cache chapter metadata
	// This populates the filecache that the download system requires
	titlePtr := &title
	container, err := d.mangaRepository.GetMangaChapterContainer(&manga.GetMangaChapterContainerOptions{
		Provider: "weebcentral",
		MediaId:  enMasseID,
		Titles:   []*string{titlePtr},
	})

	if err != nil {
		d.logger.Warn().Str("title", title).Err(err).Msg("Failed to scan manga chapters")
		return fmt.Errorf("failed to scan chapters for '%s': %w", title, err)
	}

	if container == nil || len(container.Chapters) == 0 {
		d.logger.Warn().Str("title", title).Msg("No chapters found for manga")
		return nil
	}

	// Step 2: Queue up to 10 chapters for download with rate limiting
	maxChapters := 10
	chaptersToQueue := container.Chapters
	if len(chaptersToQueue) > maxChapters {
		chaptersToQueue = chaptersToQueue[:maxChapters]
	}

	d.logger.Info().Str("title", title).Int("totalChapters", len(container.Chapters)).Int("queueingChapters", len(chaptersToQueue)).Msg("Queueing chapters for download")

	// Queue each chapter with rate limiting between requests
	for i, chapter := range chaptersToQueue {
		// Check if we should pause or stop during queuing
		d.mu.RLock()
		if !d.isRunning {
			d.mu.RUnlock()
			return fmt.Errorf("download process stopped")
		}
		for d.isPaused {
			d.mu.RUnlock()
			time.Sleep(500 * time.Millisecond)
			d.mu.RLock()
			if !d.isRunning {
				d.mu.RUnlock()
				return fmt.Errorf("download process stopped")
			}
		}
		d.mu.RUnlock()

		// Queue the chapter for download using the existing system
		err := d.mangaDownloader.DownloadChapter(manga.DownloadChapterOptions{
			Provider:   "weebcentral",
			MediaId:    enMasseID,
			ChapterId:  chapter.ID,
			StartNow:   true,
			MangaTitle: title,
		})

		if err != nil {
			d.logger.Warn().Str("title", title).Str("chapterId", chapter.ID).Str("chapterTitle", chapter.Title).Err(err).Msg("Failed to queue chapter")
			// Continue with next chapter instead of failing completely
		} else {
			d.logger.Debug().Str("title", title).Str("chapterId", chapter.ID).Str("chapterTitle", chapter.Title).Msg("Successfully queued chapter")
		}

		// Rate limiting between chapter queuing (prevent overwhelming the provider)
		if i < len(chaptersToQueue)-1 {
			time.Sleep(d.kitsuRateLimit) // 2-second delay between chapter requests
		}
	}

	d.logger.Info().Str("title", title).Int("queuedChapters", len(chaptersToQueue)).Msg("Completed queuing chapters for manga")
	return nil
}

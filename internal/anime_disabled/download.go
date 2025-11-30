//go:build disabled
// +build disabled

package anime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"seanime/internal/events"
	"strconv"
	"strings"
	"sync"
	"time"

	hibikeanime "github.com/5rahim/hibike/pkg/extension/anime"
	hibikeonlinestream "github.com/5rahim/hibike/pkg/extension/onlinestream"
	"github.com/rs/zerolog"
)

type (
	// DownloadManager handles anime episode downloads
	DownloadManager struct {
		repository      *Repository
		logger          *zerolog.Logger
		downloadDir     string
		wsEventManager  events.WSEventManagerInterface
		activeDownloads map[string]*DownloadTask
		mu              sync.RWMutex
	}

	// DownloadTask represents an active download
	DownloadTask struct {
		ID             string         `json:"id"`
		MediaID        int            `json:"mediaId"`
		EpisodeNumber  int            `json:"episodeNumber"`
		Provider       string         `json:"provider"`
		Quality        string         `json:"quality"`
		Language       string         `json:"language"`
		Status         DownloadStatus `json:"status"`
		Progress       float64        `json:"progress"`
		Speed          int64          `json:"speed"`
		ETA            time.Duration  `json:"eta"`
		FilePath       string         `json:"filePath"`
		FileSize       int64          `json:"fileSize"`
		DownloadedSize int64          `json:"downloadedSize"`
		StartTime      time.Time      `json:"startTime"`
		EndTime        *time.Time     `json:"endTime"`
		Error          string         `json:"error"`
		ctx            context.Context
		cancel         context.CancelFunc
	}

	// DownloadStatus represents download states
	DownloadStatus string

	// DownloadOptions contains download configuration
	DownloadOptions struct {
		MediaID       int    `json:"mediaId"`
		EpisodeNumber int    `json:"episodeNumber"`
		Provider      string `json:"provider"`
		Quality       string `json:"quality"`
		Language      string `json:"language"`
		OutputDir     string `json:"outputDir"`
		Filename      string `json:"filename"`
	}

	// DownloadProgressEvent is emitted during downloads
	DownloadProgressEvent struct {
		TaskID         string  `json:"taskId"`
		MediaID        int     `json:"mediaId"`
		EpisodeNumber  int     `json:"episodeNumber"`
		Progress       float64 `json:"progress"`
		Speed          int64   `json:"speed"`
		ETA            string  `json:"eta"`
		DownloadedSize int64   `json:"downloadedSize"`
		TotalSize      int64   `json:"totalSize"`
	}

	// DownloadCompleteEvent is emitted when download completes
	DownloadCompleteEvent struct {
		TaskID        string `json:"taskId"`
		MediaID       int    `json:"mediaId"`
		EpisodeNumber int    `json:"episodeNumber"`
		FilePath      string `json:"filePath"`
		Success       bool   `json:"success"`
		Error         string `json:"error"`
	}
)

const (
	DownloadStatusPending     DownloadStatus = "pending"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusCompleted   DownloadStatus = "completed"
	DownloadStatusFailed      DownloadStatus = "failed"
	DownloadStatusCancelled   DownloadStatus = "cancelled"
)

// NewDownloadManager creates a new download manager
func NewDownloadManager(repository *Repository, logger *zerolog.Logger, downloadDir string, wsEventManager events.WSEventManagerInterface) *DownloadManager {
	return &DownloadManager{
		repository:      repository,
		logger:          logger,
		downloadDir:     downloadDir,
		wsEventManager:  wsEventManager,
		activeDownloads: make(map[string]*DownloadTask),
	}
}

// DownloadEpisode starts downloading an episode
func (dm *DownloadManager) DownloadEpisode(ctx context.Context, options DownloadOptions) (*DownloadTask, error) {
	taskID := fmt.Sprintf("anime_%d_%d_%s", options.MediaID, options.EpisodeNumber, options.Provider)

	dm.mu.Lock()
	if existingTask, exists := dm.activeDownloads[taskID]; exists {
		dm.mu.Unlock()
		return existingTask, nil
	}

	// Create download task
	taskCtx, cancel := context.WithCancel(ctx)
	task := &DownloadTask{
		ID:            taskID,
		MediaID:       options.MediaID,
		EpisodeNumber: options.EpisodeNumber,
		Provider:      options.Provider,
		Quality:       options.Quality,
		Language:      options.Language,
		Status:        DownloadStatusPending,
		StartTime:     time.Now(),
		ctx:           taskCtx,
		cancel:        cancel,
	}

	dm.activeDownloads[taskID] = task
	dm.mu.Unlock()

	// Start download in goroutine
	go dm.executeDownload(task, options)

	dm.logger.Info().
		Str("taskId", taskID).
		Int("mediaId", options.MediaID).
		Int("episode", options.EpisodeNumber).
		Str("provider", options.Provider).
		Msg("anime: Started episode download")

	return task, nil
}

// executeDownload performs the actual download
func (dm *DownloadManager) executeDownload(task *DownloadTask, options DownloadOptions) {
	defer func() {
		dm.mu.Lock()
		delete(dm.activeDownloads, task.ID)
		dm.mu.Unlock()
	}()

	// Get episode container
	container, found := dm.repository.GetEpisodeContainer(options.Provider, options.MediaID)
	if !found {
		dm.failTask(task, "episode container not found")
		return
	}

	// Find the episode
	var targetEpisode *hibikeonlinestream.EpisodeDetails
	for _, episode := range container.Episodes {
		if int(episode.Number) == options.EpisodeNumber {
			targetEpisode = episode
			break
		}
	}

	if targetEpisode == nil {
		dm.failTask(task, "episode not found")
		return
	}

	// Get stream links
	streamContainer, err := dm.repository.GetEpisodeStreamLinks(task.ctx, options.Provider, options.MediaID, targetEpisode.ID)
	if err != nil {
		dm.failTask(task, fmt.Sprintf("failed to get stream links: %v", err))
		return
	}

	// Find best quality stream
	streamLink := dm.selectBestStream(streamContainer.StreamLinks, options.Quality)
	if streamLink == nil {
		dm.failTask(task, "no suitable stream found")
		return
	}

	// Prepare download path
	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(dm.downloadDir, "anime", strconv.Itoa(options.MediaID))
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		dm.failTask(task, fmt.Sprintf("failed to create output directory: %v", err))
		return
	}

	filename := options.Filename
	if filename == "" {
		filename = fmt.Sprintf("Episode_%02d.mp4", options.EpisodeNumber)
	}

	task.FilePath = filepath.Join(outputDir, filename)

	// Start download
	task.Status = DownloadStatusDownloading
	dm.updateProgress(task)

	if err := dm.downloadFile(task, streamLink.URL); err != nil {
		dm.failTask(task, fmt.Sprintf("download failed: %v", err))
		return
	}

	// Download subtitles if available
	if len(streamContainer.SubtitleLinks) > 0 {
		dm.downloadSubtitles(task, streamContainer.SubtitleLinks, outputDir)
	}

	// Mark as completed
	now := time.Now()
	task.Status = DownloadStatusCompleted
	task.EndTime = &now
	task.Progress = 100.0

	dm.updateProgress(task)
	dm.emitCompleteEvent(task, true, "")

	dm.logger.Info().
		Str("taskId", task.ID).
		Str("filePath", task.FilePath).
		Msg("anime: Episode download completed")
}

// downloadFile downloads a file from URL with progress tracking
func (dm *DownloadManager) downloadFile(task *DownloadTask, url string) error {
	req, err := http.NewRequestWithContext(task.ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	task.FileSize = resp.ContentLength

	// Create output file
	out, err := os.Create(task.FilePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Download with progress tracking
	buffer := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64
	startTime := time.Now()

	for {
		select {
		case <-task.ctx.Done():
			return task.ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := out.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			task.DownloadedSize = downloaded

			// Update progress
			if task.FileSize > 0 {
				task.Progress = float64(downloaded) / float64(task.FileSize) * 100
			}

			// Calculate speed and ETA
			elapsed := time.Since(startTime)
			if elapsed > 0 {
				task.Speed = downloaded / int64(elapsed.Seconds())
				if task.Speed > 0 && task.FileSize > 0 {
					remaining := task.FileSize - downloaded
					task.ETA = time.Duration(remaining/task.Speed) * time.Second
				}
			}

			// Emit progress event every 1MB or 5%
			if downloaded%1048576 == 0 || (task.FileSize > 0 && int(task.Progress)%5 == 0) {
				dm.updateProgress(task)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// downloadSubtitles downloads subtitle files
func (dm *DownloadManager) downloadSubtitles(task *DownloadTask, subtitles []*hibikeanime.SubtitleLink, outputDir string) {
	for _, subtitle := range subtitles {
		if subtitle.URL == "" {
			continue
		}

		// Create subtitle filename
		ext := ".srt"
		if strings.Contains(subtitle.URL, ".vtt") {
			ext = ".vtt"
		} else if strings.Contains(subtitle.URL, ".ass") {
			ext = ".ass"
		}

		subtitleFile := fmt.Sprintf("Episode_%02d_%s%s", task.EpisodeNumber, subtitle.Language, ext)
		subtitlePath := filepath.Join(outputDir, subtitleFile)

		// Download subtitle
		if err := dm.downloadSubtitleFile(task.ctx, subtitle.URL, subtitlePath); err != nil {
			dm.logger.Warn().
				Err(err).
				Str("url", subtitle.URL).
				Str("language", subtitle.Language).
				Msg("anime: Failed to download subtitle")
		} else {
			dm.logger.Debug().
				Str("path", subtitlePath).
				Str("language", subtitle.Language).
				Msg("anime: Downloaded subtitle")
		}
	}
}

// downloadSubtitleFile downloads a subtitle file
func (dm *DownloadManager) downloadSubtitleFile(ctx context.Context, url, filePath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// selectBestStream selects the best quality stream
func (dm *DownloadManager) selectBestStream(streams []*hibikeanime.EpisodeStreamLink, preferredQuality string) *hibikeanime.EpisodeStreamLink {
	if len(streams) == 0 {
		return nil
	}

	// Try to find preferred quality
	if preferredQuality != "" {
		for _, stream := range streams {
			if strings.Contains(strings.ToLower(stream.Quality), strings.ToLower(preferredQuality)) {
				return stream
			}
		}
	}

	// Quality priority: 1080p > 720p > 480p > others
	qualityPriority := map[string]int{
		"1080p": 4,
		"720p":  3,
		"480p":  2,
		"360p":  1,
	}

	bestStream := streams[0]
	bestScore := 0

	for _, stream := range streams {
		score := 0
		qualityLower := strings.ToLower(stream.Quality)

		for quality, priority := range qualityPriority {
			if strings.Contains(qualityLower, quality) {
				score = priority
				break
			}
		}

		if score > bestScore {
			bestScore = score
			bestStream = stream
		}
	}

	return bestStream
}

// failTask marks a task as failed
func (dm *DownloadManager) failTask(task *DownloadTask, errorMsg string) {
	now := time.Now()
	task.Status = DownloadStatusFailed
	task.EndTime = &now
	task.Error = errorMsg

	dm.updateProgress(task)
	dm.emitCompleteEvent(task, false, errorMsg)

	dm.logger.Error().
		Str("taskId", task.ID).
		Str("error", errorMsg).
		Msg("anime: Episode download failed")
}

// updateProgress emits progress update event
func (dm *DownloadManager) updateProgress(task *DownloadTask) {
	if dm.wsEventManager == nil {
		return
	}

	event := &DownloadProgressEvent{
		TaskID:         task.ID,
		MediaID:        task.MediaID,
		EpisodeNumber:  task.EpisodeNumber,
		Progress:       task.Progress,
		Speed:          task.Speed,
		ETA:            task.ETA.String(),
		DownloadedSize: task.DownloadedSize,
		TotalSize:      task.FileSize,
	}

	dm.wsEventManager.SendEvent(events.AnimeDownloadProgress, event)
}

// emitCompleteEvent emits download complete event
func (dm *DownloadManager) emitCompleteEvent(task *DownloadTask, success bool, errorMsg string) {
	if dm.wsEventManager == nil {
		return
	}

	event := &DownloadCompleteEvent{
		TaskID:        task.ID,
		MediaID:       task.MediaID,
		EpisodeNumber: task.EpisodeNumber,
		FilePath:      task.FilePath,
		Success:       success,
		Error:         errorMsg,
	}

	dm.wsEventManager.SendEvent(events.AnimeDownloadComplete, event)
}

// CancelDownload cancels an active download
func (dm *DownloadManager) CancelDownload(taskID string) error {
	dm.mu.RLock()
	task, exists := dm.activeDownloads[taskID]
	dm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("download task not found: %s", taskID)
	}

	task.cancel()
	task.Status = DownloadStatusCancelled
	now := time.Now()
	task.EndTime = &now

	dm.logger.Info().
		Str("taskId", taskID).
		Msg("anime: Download cancelled")

	return nil
}

// GetActiveDownloads returns all active downloads
func (dm *DownloadManager) GetActiveDownloads() []*DownloadTask {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	tasks := make([]*DownloadTask, 0, len(dm.activeDownloads))
	for _, task := range dm.activeDownloads {
		tasks = append(tasks, task)
	}

	return tasks
}

// GetDownloadTask returns a specific download task
func (dm *DownloadManager) GetDownloadTask(taskID string) (*DownloadTask, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	task, exists := dm.activeDownloads[taskID]
	return task, exists
}

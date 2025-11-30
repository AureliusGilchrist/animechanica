package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type NyaaCrawlerStatus struct {
	IsRunning            bool      `json:"isRunning"`
	Progress             float64   `json:"progress"`
	CurrentQuery         string    `json:"currentQuery"`
	TotalQueries         int       `json:"totalQueries"`
	ProcessedQueries     int       `json:"processedQueries"`
	TorrentsFound        int       `json:"torrentsFound"`
	TorrentsAdded        int       `json:"torrentsAdded"`
	StartTime            time.Time `json:"startTime"`
	EstimatedTimeLeft    string    `json:"estimatedTimeLeft"`
	LastActivity         time.Time `json:"lastActivity"`
	Logs                 []string  `json:"logs"`
}

type NyaaCrawlerConfig struct {
	SearchQueries    []string `json:"searchQueries"`
	StartPage        int      `json:"startPage"`
	EndPage          int      `json:"endPage"`
	DelaySeconds     int      `json:"delaySeconds"`
	QBittorrentURL   string   `json:"qbittorrentUrl"`
	QBittorrentUser  string   `json:"qbittorrentUser"`
	QBittorrentPass  string   `json:"qbittorrentPass"`
	DownloadPath     string   `json:"downloadPath"`
}

type NyaaCrawlerManager struct {
	status    *NyaaCrawlerStatus
	config    *NyaaCrawlerConfig
	cmd       *exec.Cmd
	mutex     sync.RWMutex
	isRunning bool
}

var crawlerManager = &NyaaCrawlerManager{
	status: &NyaaCrawlerStatus{
		IsRunning:        false,
		Progress:         0.0,
		TotalQueries:     0,
		ProcessedQueries: 0,
		TorrentsFound:    0,
		TorrentsAdded:    0,
		Logs:             []string{},
	},
	config: &NyaaCrawlerConfig{
		SearchQueries: []string{
			"[Judas] batch",
			"[DB] batch", 
			"[EMBER] batch",
			"[Erai-raws] batch",
			"[SubsPlease] batch",
			"[HorribleSubs] batch",
			"[Trix] batch",
		},
		StartPage:       1,
		EndPage:         1000,
		DelaySeconds:    1,
		QBittorrentURL:  "http://localhost:8081/api/v2",
		QBittorrentUser: "admin",
		QBittorrentPass: "lolmao",
		DownloadPath:    "/aeternae/library/anime/seanime",
	},
}

// HandleGetNyaaCrawlerStatus returns the current status of the Nyaa crawler
func (h *Handler) HandleGetNyaaCrawlerStatus(c echo.Context) error {
	crawlerManager.mutex.RLock()
	defer crawlerManager.mutex.RUnlock()

	return h.RespondWithData(c, crawlerManager.status)
}

// HandleGetNyaaCrawlerConfig returns the current configuration
func (h *Handler) HandleGetNyaaCrawlerConfig(c echo.Context) error {
	crawlerManager.mutex.RLock()
	defer crawlerManager.mutex.RUnlock()

	return h.RespondWithData(c, crawlerManager.config)
}

// HandleUpdateNyaaCrawlerConfig updates the crawler configuration
func (h *Handler) HandleUpdateNyaaCrawlerConfig(c echo.Context) error {
	var config NyaaCrawlerConfig
	if err := c.Bind(&config); err != nil {
		return h.RespondWithError(c, err)
	}

	crawlerManager.mutex.Lock()
	crawlerManager.config = &config
	crawlerManager.mutex.Unlock()

	return h.RespondWithData(c, map[string]string{"message": "Configuration updated successfully"})
}

// HandleStartNyaaCrawler starts the Python Nyaa crawler
func (h *Handler) HandleStartNyaaCrawler(c echo.Context) error {
	crawlerManager.mutex.Lock()
	defer crawlerManager.mutex.Unlock()

	if crawlerManager.isRunning {
		return h.RespondWithError(c, fmt.Errorf("crawler is already running"))
	}

	// Create config file for Python script
	// IMPORTANT: The Python crawler expects "config.json" in its working directory.
	// We write that exact filename to ensure it picks up our settings (download path, queries, etc.).
	configPath := filepath.Join(h.App.Config.Data.AppDataDir, "config.json")
	configData := map[string]interface{}{
		"qbittorrent": map[string]interface{}{
			"url":      crawlerManager.config.QBittorrentURL,
			"username": crawlerManager.config.QBittorrentUser,
			"password": crawlerManager.config.QBittorrentPass,
		},
		"nyaa": map[string]interface{}{
			"base_url":     "https://nyaa.si",
			"search_query": "", // Will be set per query
			"category":     "0_0",
			"filter":       "0",
			"start_page":   crawlerManager.config.StartPage,
			"end_page":     crawlerManager.config.EndPage,
			"delay":        crawlerManager.config.DelaySeconds,
		},
		"download_path": crawlerManager.config.DownloadPath,
	}

	configJSON, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return h.RespondWithError(c, err)
	}

	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		return h.RespondWithError(c, err)
	}

	// Start the Python crawler
	crawlerPath := filepath.Join(h.App.Config.Data.WorkingDir, "internal", "torrents", "nyaa", "crawler.py")
	
	// Reset status
	crawlerManager.status = &NyaaCrawlerStatus{
		IsRunning:        true,
		Progress:         0.0,
		TotalQueries:     len(crawlerManager.config.SearchQueries),
		ProcessedQueries: 0,
		TorrentsFound:    0,
		TorrentsAdded:    0,
		StartTime:        time.Now(),
		LastActivity:     time.Now(),
		Logs:             []string{"Starting Nyaa crawler..."},
	}

	crawlerManager.isRunning = true

	// Start crawler in goroutine
	go h.runNyaaCrawler(crawlerPath, configPath)

	return h.RespondWithData(c, map[string]string{"message": "Crawler started successfully"})
}

// HandleStopNyaaCrawler stops the Python Nyaa crawler
func (h *Handler) HandleStopNyaaCrawler(c echo.Context) error {
	crawlerManager.mutex.Lock()
	defer crawlerManager.mutex.Unlock()

	if !crawlerManager.isRunning {
		return h.RespondWithError(c, fmt.Errorf("no crawler is currently running"))
	}

	if crawlerManager.cmd != nil && crawlerManager.cmd.Process != nil {
		crawlerManager.cmd.Process.Kill()
	}

	crawlerManager.isRunning = false
	crawlerManager.status.IsRunning = false
	crawlerManager.status.LastActivity = time.Now()
	
	// Add log entry
	crawlerManager.status.Logs = append(crawlerManager.status.Logs, "Crawler stopped by user")
	if len(crawlerManager.status.Logs) > 100 {
		crawlerManager.status.Logs = crawlerManager.status.Logs[len(crawlerManager.status.Logs)-100:]
	}

	return h.RespondWithData(c, map[string]string{"message": "Crawler stopped successfully"})
}

func (h *Handler) runNyaaCrawler(crawlerPath, configPath string) {
	defer func() {
		crawlerManager.mutex.Lock()
		crawlerManager.isRunning = false
		crawlerManager.status.IsRunning = false
		crawlerManager.status.LastActivity = time.Now()
		crawlerManager.mutex.Unlock()
	}()

	for i, query := range crawlerManager.config.SearchQueries {
		if !crawlerManager.isRunning {
			break
		}

		crawlerManager.mutex.Lock()
		crawlerManager.status.CurrentQuery = query
		crawlerManager.status.ProcessedQueries = i
		crawlerManager.status.Progress = float64(i) / float64(len(crawlerManager.config.SearchQueries))
		crawlerManager.status.LastActivity = time.Now()
		
		// Add log entry
		logMsg := fmt.Sprintf("Processing query %d/%d: %s", i+1, len(crawlerManager.config.SearchQueries), query)
		crawlerManager.status.Logs = append(crawlerManager.status.Logs, logMsg)
		if len(crawlerManager.status.Logs) > 100 {
			crawlerManager.status.Logs = crawlerManager.status.Logs[len(crawlerManager.status.Logs)-100:]
		}
		crawlerManager.mutex.Unlock()

		// Update config file with current query
		configData := map[string]interface{}{
			"qbittorrent": map[string]interface{}{
				"url":      crawlerManager.config.QBittorrentURL,
				"username": crawlerManager.config.QBittorrentUser,
				"password": crawlerManager.config.QBittorrentPass,
			},
			"nyaa": map[string]interface{}{
				"base_url":     "https://nyaa.si",
				"search_query": query,
				"category":     "0_0",
				"filter":       "0",
				"start_page":   crawlerManager.config.StartPage,
				"end_page":     crawlerManager.config.EndPage,
				"delay":        crawlerManager.config.DelaySeconds,
			},
			"download_path": crawlerManager.config.DownloadPath,
		}

		configJSON, _ := json.MarshalIndent(configData, "", "  ")
		os.WriteFile(configPath, configJSON, 0644)

		// Run Python script for this query
		cmd := exec.Command("python3", crawlerPath)
		cmd.Dir = filepath.Dir(configPath)
		
		crawlerManager.mutex.Lock()
		crawlerManager.cmd = cmd
		crawlerManager.mutex.Unlock()

		if err := cmd.Run(); err != nil {
			crawlerManager.mutex.Lock()
			errorMsg := fmt.Sprintf("Error processing query '%s': %v", query, err)
			crawlerManager.status.Logs = append(crawlerManager.status.Logs, errorMsg)
			if len(crawlerManager.status.Logs) > 100 {
				crawlerManager.status.Logs = crawlerManager.status.Logs[len(crawlerManager.status.Logs)-100:]
			}
			crawlerManager.mutex.Unlock()
		}

		// Small delay between queries
		time.Sleep(2 * time.Second)
	}

	// Final status update
	crawlerManager.mutex.Lock()
	crawlerManager.status.Progress = 1.0
	crawlerManager.status.ProcessedQueries = len(crawlerManager.config.SearchQueries)
	crawlerManager.status.CurrentQuery = ""
	crawlerManager.status.Logs = append(crawlerManager.status.Logs, "Crawler completed successfully")
	if len(crawlerManager.status.Logs) > 100 {
		crawlerManager.status.Logs = crawlerManager.status.Logs[len(crawlerManager.status.Logs)-100:]
	}
	crawlerManager.mutex.Unlock()
}

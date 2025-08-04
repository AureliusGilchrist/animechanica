package qbittorrent

import (
	"errors"
	"runtime"
	"seanime/internal/util"
	"time"
)

// getExecutableName returns the appropriate executable name for the current OS
func (c *Client) getExecutableName() string {
	switch runtime.GOOS {
	case "windows":
		return "qbittorrent.exe"
	default:
		return "qbittorrent"
	}
}

// getExecutablePath returns the default path for qBittorrent binary on the current OS
// This is only used when Path is not explicitly set and DisableBinaryUse is false
func (c *Client) getExecutablePath() string {

	if len(c.Path) > 0 {
		return c.Path
	}

	switch runtime.GOOS {
	case "windows":
		return "C:/Program Files/qBittorrent/qbittorrent.exe"
	case "linux":
		return "/usr/bin/qbittorrent" // Default path for Client on most Linux distributions
	case "darwin":
		return "/Applications/qbittorrent.app/Contents/MacOS/qbittorrent" // Default path for Client on macOS
	default:
		return "C:/Program Files/qBittorrent/qbittorrent.exe"
	}
}

// Start attempts to start a local qBittorrent instance
// For Docker/external qBittorrent (Path="" or DisableBinaryUse=true), this is a no-op
// For local qBittorrent, it checks if already running and starts if needed

func (c *Client) Start() error {

	// If the path is empty or DisableBinaryUse is true, assume external/Docker qBittorrent
	if c.Path == "" || c.DisableBinaryUse {
		c.logger.Debug().Msg("qbittorrent: Skipping binary start - using external/Docker qBittorrent instance")
		return nil
	}

	name := c.getExecutableName()
	if util.ProgramIsRunning(name) {
		c.logger.Debug().Msg("qbittorrent: Local qBittorrent instance already running")
		return nil
	}

	c.logger.Info().Msg("qbittorrent: Starting local qBittorrent instance")
	exe := c.getExecutablePath()
	cmd := util.NewCmd(exe)
	err := cmd.Start()
	if err != nil {
		c.logger.Error().Err(err).Str("path", exe).Msg("qbittorrent: Failed to start local qBittorrent")
		return errors.New("failed to start qBittorrent")
	}

	c.logger.Info().Msg("qbittorrent: Local qBittorrent instance started successfully")
	time.Sleep(1 * time.Second)

	return nil
}

// CheckStart verifies that qBittorrent is accessible and starts it if needed
// For Docker/external qBittorrent: Only checks API accessibility 
// For local qBittorrent: Checks accessibility and attempts to start if not running
func (c *Client) CheckStart() bool {
	if c == nil {
		return false
	}

	// If using external/Docker qBittorrent, check if it's accessible via API
	if c.Path == "" || c.DisableBinaryUse {
		c.logger.Debug().Msg("qbittorrent: Checking external/Docker qBittorrent accessibility")
		_, err := c.Application.GetAppVersion()
		if err == nil {
			c.logger.Debug().Msg("qbittorrent: External/Docker qBittorrent is accessible")
			return true
		}
		c.logger.Warn().Err(err).Str("host", c.Host).Int("port", c.Port).Msg("qbittorrent: External/Docker qBittorrent is not accessible")
		return false
	}

	// For local qBittorrent, check if accessible and try to start if needed
	c.logger.Debug().Msg("qbittorrent: Checking local qBittorrent instance")
	_, err := c.Application.GetAppVersion()
	if err == nil {
		c.logger.Debug().Msg("qbittorrent: Local qBittorrent is accessible")
		return true
	}

	c.logger.Info().Msg("qbittorrent: Local qBittorrent not accessible, attempting to start")
	err = c.Start()
	if err != nil {
		c.logger.Error().Err(err).Msg("qbittorrent: Failed to start local qBittorrent")
		return false
	}

	// Wait for local qBittorrent to become available
	timeout := time.After(30 * time.Second)
	ticker := time.Tick(1 * time.Second)
	for {
		select {
		case <-ticker:
			_, err = c.Application.GetAppVersion()
			if err == nil {
				c.logger.Info().Msg("qbittorrent: Local qBittorrent is now accessible")
				return true
			}
		case <-timeout:
			c.logger.Error().Msg("qbittorrent: Timeout waiting for local qBittorrent to become accessible")
			return false
		}
	}
}

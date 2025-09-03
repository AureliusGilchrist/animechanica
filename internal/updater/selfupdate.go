package updater

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/samber/mo"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"seanime/internal/constants"
	"seanime/internal/util"
	"slices"
	"strings"
	"syscall"
)

const (
	tempReleaseDir = "seanime_new_release"
	backupDirName  = "backup_restore_if_failed"
)

type (
	SelfUpdater struct {
		logger          *zerolog.Logger
		breakLoopCh     chan struct{}
		originalExePath mo.Option[string]
		updater         *Updater
		fallbackDest    string

		tmpExecutableName string
	}
)

func NewSelfUpdater() *SelfUpdater {
	logger := util.NewLogger()
	ret := &SelfUpdater{
		logger:          logger,
		breakLoopCh:     make(chan struct{}),
		originalExePath: mo.None[string](),
		updater:         New(constants.Version, logger, nil),
	}

	ret.tmpExecutableName = "seanime.exe.old"
	switch runtime.GOOS {
	case "windows":
		ret.tmpExecutableName = "seanime.exe.old"
	default:
		ret.tmpExecutableName = "seanime.old"
	}

	go func() {
		// Delete all files with the .old extension
		exePath := getExePath()
		entries, err := os.ReadDir(filepath.Dir(exePath))
		if err != nil {
			return
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".old") {
				_ = os.RemoveAll(filepath.Join(filepath.Dir(exePath), entry.Name()))
			}
		}

	}()

	return ret
}

// Started returns a channel that will be closed when the app loop should be broken
func (su *SelfUpdater) Started() <-chan struct{} {
	return su.breakLoopCh
}

func (su *SelfUpdater) StartSelfUpdate(fallbackDestination string) {
    // Updates are disabled in this fork; do nothing.
    su.fallbackDest = fallbackDestination
    // Intentionally do NOT close breakLoopCh to avoid switching the server into update mode
}

// recover will just print a message and attempt to download the latest release
func (su *SelfUpdater) recover(assetUrl string) {

	if su.originalExePath.IsAbsent() {
		return
	}

	if su.fallbackDest != "" {
		su.logger.Info().Str("dest", su.fallbackDest).Msg("selfupdate: Attempting to download the latest release")
		_, _ = su.updater.DownloadLatestRelease(assetUrl, su.fallbackDest)
	}

	su.logger.Error().Msg("selfupdate: Failed to install update. Update downloaded to 'seanime_new_release'")
}

func getExePath() string {
	exe, err := os.Executable() // /path/to/seanime.exe
	if err != nil {
		return ""
	}
	exePath, err := filepath.EvalSymlinks(exe) // /path/to/seanime.exe
	if err != nil {
		return ""
	}

	return exePath
}

func (su *SelfUpdater) Run() error {
    // Updates are disabled in this fork; do nothing and return.
    su.logger.Info().Msg("selfupdate: Run() called, but updates are disabled; skipping")
    return nil
}

func openWindows(path string) error {
	cmd := util.NewCmd("cmd", "/c", "start", "cmd", "/k", path)
	return cmd.Start()
}

func openMacOS(path string) error {
	script := fmt.Sprintf(`
    tell application "Terminal"
        do script "%s"
        activate
    end tell`, path)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Start()
}

func openLinux(path string) error {
	// Filter out the -update flag or we end up in an infinite update loop
	filteredArgs := slices.DeleteFunc(os.Args, func(s string) bool { return s == "-update" })

	// Replace the current process with the updated executable
	return syscall.Exec(path, filteredArgs, os.Environ())
}

// moveContents moves contents of newReleaseDir to exeDir without deleting existing files
func moveContents(newReleaseDir, exeDir string) error {
	// Ensure exeDir exists
	if err := os.MkdirAll(exeDir, 0755); err != nil {
		return err
	}

	// Copy contents of newReleaseDir to exeDir
	return copyDir(newReleaseDir, exeDir)
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination directory if it does not exist
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

// copyDir recursively copies a directory tree, attempting to preserve permissions.
func copyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

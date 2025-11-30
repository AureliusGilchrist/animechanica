package scanner

import (
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"os"
	"path/filepath"
	"seanime/internal/events"
	"strings"
	"sync"
	"time"
)

// Watcher is a custom file system event watcher
type Watcher struct {
	Watcher        *fsnotify.Watcher
	Logger         *zerolog.Logger
	WSEventManager events.WSEventManagerInterface
	TotalSize      string
}

type NewWatcherOptions struct {
	Logger         *zerolog.Logger
	WSEventManager events.WSEventManagerInterface
}

// NewWatcher creates a new Watcher instance for monitoring a directory and its subdirectories
func NewWatcher(opts *NewWatcherOptions) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		Watcher:        watcher,
		Logger:         opts.Logger,
		WSEventManager: opts.WSEventManager,
	}, nil
}

//----------------------------------------------------------------------------------------------------------------------

type WatchLibraryFilesOptions struct {
	LibraryPaths []string
}

// InitLibraryFileWatcher starts watching the specified directory and its subdirectories for file system events
func (w *Watcher) InitLibraryFileWatcher(opts *WatchLibraryFilesOptions) error {
	// Define a function to add directories and their subdirectories to the watcher
	watchDir := func(dir string) error {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return w.Watcher.Add(path)
			}
			return nil
		})
		return err
	}

	// Add the initial directory and its subdirectories to the watcher
	for _, path := range opts.LibraryPaths {
		if err := watchDir(path); err != nil {
			return err
		}
	}

	w.Logger.Info().Msgf("watcher: Watching directories: %+v", opts.LibraryPaths)

	return nil
}

func (w *Watcher) StartWatching(
	onFileAction func(),
) {
	// Start a goroutine to handle file system events
	go func() {
		// Helpers
		isTempDownloadFile := func(name string) bool {
			lower := strings.ToLower(name)
			// Common temp/partial suffixes from downloaders
			if strings.HasSuffix(lower, ".!qb") ||
				strings.HasSuffix(lower, ".part") ||
				strings.HasSuffix(lower, ".tmp") ||
				strings.HasSuffix(lower, ".partial") ||
				strings.HasSuffix(lower, ".crdownload") ||
				strings.HasSuffix(lower, ".aria2") {
				return true
			}
			return false
		}

		// Special theater completed directory: reduce log noise
		const theaterCompletedBase = "/aeternae/theater/anime/completed"
		isUnder := func(base, p string) bool {
			// ensure separator handling is safe
			baseClean := filepath.Clean(base) + string(os.PathSeparator)
			pClean := filepath.Clean(p)
			return strings.HasPrefix(pClean, baseClean)
		}
		isTopLevelUnder := func(base, p string) bool {
			// p should be immediate child of base
			rel, err := filepath.Rel(base, p)
			if err != nil || strings.HasPrefix(rel, "..") {
				return false
			}
			// exactly one path element (no separators)
			return !strings.Contains(rel, string(os.PathSeparator))
		}

		// Debounce onFileAction to avoid flooding during bursts
		var mu sync.Mutex
		var debounceTimer *time.Timer
		trigger := func() {
			mu.Lock()
			defer mu.Unlock()
			if debounceTimer != nil {
				// Reset existing timer
				if !debounceTimer.Stop() {
					select {
					case <-debounceTimer.C:
					default:
					}
				}
				debounceTimer.Reset(750 * time.Millisecond)
				return
			}
			debounceTimer = time.AfterFunc(750*time.Millisecond, func() {
				// Execute action outside the lock
				onFileAction()
				mu.Lock()
				// Guard against nil timer to avoid panic if state changed concurrently
				if debounceTimer != nil {
					_ = debounceTimer.Stop()
					debounceTimer = nil
				}
				mu.Unlock()
			})
		}

		for {
			select {
			case event, ok := <-w.Watcher.Events:
				if !ok {
					return
				}
				// Ignore noisy temp/partial files entirely (no log, no trigger)
				if isTempDownloadFile(event.Name) {
					continue
				}
				// If a new directory is created, start watching it as well
				if event.Op&fsnotify.Create == fsnotify.Create {
					// Limit logging under theater completed dir to top-level directory create events only
					if isUnder(theaterCompletedBase, event.Name) {
						if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() && isTopLevelUnder(theaterCompletedBase, event.Name) {
							w.Logger.Debug().Msgf("watcher: File created: %s", event.Name)
						}
					} else {
						w.Logger.Debug().Msgf("watcher: File created: %s", event.Name)
					}
					w.WSEventManager.SendEvent(events.LibraryWatcherFileAdded, event.Name)
					// attempt to add watcher if it's a directory
					if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
						// Add the new directory (and its current subdirs) to the watcher
						_ = filepath.Walk(event.Name, func(path string, info os.FileInfo, err error) error {
							if err != nil {
								return nil
							}
							if info.IsDir() {
								_ = w.Watcher.Add(path)
							}
							return nil
						})
					}
					trigger()
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					// Limit logging under theater completed dir to top-level directory remove events only
					if isUnder(theaterCompletedBase, event.Name) {
						// We can't Stat removed path; approximate with path depth check
						if isTopLevelUnder(theaterCompletedBase, event.Name) {
							w.Logger.Debug().Msgf("watcher: File removed: %s", event.Name)
						}
					} else {
						w.Logger.Debug().Msgf("watcher: File removed: %s", event.Name)
					}
					w.WSEventManager.SendEvent(events.LibraryWatcherFileRemoved, event.Name)
					trigger()
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					// Suppress write logs under the theater completed dir to reduce noise
					if !isUnder(theaterCompletedBase, event.Name) {
						w.Logger.Trace().Msgf("watcher: File modified: %s", event.Name)
					}
					trigger()
				}
				if event.Op&fsnotify.Rename == fsnotify.Rename {
					// Suppress rename logs under the theater completed dir to reduce noise
					if !isUnder(theaterCompletedBase, event.Name) {
						w.Logger.Trace().Msgf("watcher: File renamed: %s", event.Name)
					}
					trigger()
				}

			case err, ok := <-w.Watcher.Errors:
				if !ok {
					return
				}
				// When kernel/event queue overflows, fsnotify returns an overflow error.
				// Debounce a full rescan by triggering the debounced action and log clearly.
				if strings.Contains(strings.ToLower(err.Error()), "overflow") {
					w.Logger.Warn().Err(err).Msgf("watcher: Overflow detected; scheduling debounced rescan")
					trigger()
					continue
				}
				w.Logger.Warn().Err(err).Msgf("watcher: Error while watching directory")
			}
		}
	}()
}

func (w *Watcher) StopWatching() {
	err := w.Watcher.Close()
	if err == nil {
		w.Logger.Trace().Err(err).Msgf("watcher: Watcher stopped")
	}
}

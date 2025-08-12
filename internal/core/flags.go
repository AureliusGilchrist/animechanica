package core

import (
	"flag"
	"fmt"
	"strings"
)

type (
	SeanimeFlags struct {
		DataDir          string
		Update           bool
		IsDesktopSidecar bool
		PprofEnabled     bool
		PprofAddr        string
	}
)

func GetSeanimeFlags() SeanimeFlags {
	// Help flag
	flag.Usage = func() {
		fmt.Printf("Self-hosted, user-friendly, media server for anime and manga enthusiasts.\n\n")
		fmt.Printf("Usage:\n  seanime [flags]\n\n")
		fmt.Printf("Flags:\n")
		fmt.Printf("  -datadir, --datadir string")
		fmt.Printf("   directory that contains all Seanime data\n")
		fmt.Printf("  -update")
		fmt.Printf("   update the application\n")
		fmt.Printf("  -pprof\n")
		fmt.Printf("   enable pprof profiling server (listens on -pprof-addr)\n")
		fmt.Printf("  -pprof-addr string\n")
		fmt.Printf("   address for pprof server (default 127.0.0.1:6060)\n")
		fmt.Printf("  -h                           show this help message\n")
	}
	// Parse flags
	var dataDir string
	flag.StringVar(&dataDir, "datadir", "", "Directory that contains all Seanime data")
	var update bool
	flag.BoolVar(&update, "update", false, "Update the application")
	var isDesktopSidecar bool
	flag.BoolVar(&isDesktopSidecar, "desktop-sidecar", false, "Run as the desktop sidecar")
	var pprofEnabled bool
	flag.BoolVar(&pprofEnabled, "pprof", false, "Enable pprof profiling server")
	var pprofAddr string
	flag.StringVar(&pprofAddr, "pprof-addr", "127.0.0.1:6060", "Address for pprof server")
	flag.Parse()

	return SeanimeFlags{
		DataDir:          strings.TrimSpace(dataDir),
		Update:           update,
		IsDesktopSidecar: isDesktopSidecar,
		PprofEnabled:     pprofEnabled,
		PprofAddr:        strings.TrimSpace(pprofAddr),
	}
}

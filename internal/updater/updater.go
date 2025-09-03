package updater

import (
	"net/http"
	"seanime/internal/events"
	"time"

	"github.com/rs/zerolog"
	"github.com/samber/mo"
)

const (
	PatchRelease = "patch"
	MinorRelease = "minor"
	MajorRelease = "major"
)

type (
	Updater struct {
		CurrentVersion      string
		hasCheckedForUpdate bool
		LatestRelease       *Release
		checkForUpdate      bool
		logger              *zerolog.Logger
		client              *http.Client
		wsEventManager      mo.Option[events.WSEventManagerInterface]
		announcements       []Announcement
	}

	Update struct {
		Release        *Release `json:"release,omitempty"`
		CurrentVersion string   `json:"current_version,omitempty"`
		Type           string   `json:"type"`
	}
)

func New(currVersion string, logger *zerolog.Logger, wsEventManager events.WSEventManagerInterface) *Updater {
	ret := &Updater{
		CurrentVersion:      currVersion,
		hasCheckedForUpdate: false,
		// Disable update checks by default in this fork
		checkForUpdate:      false,
		logger:              logger,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
		wsEventManager: mo.None[events.WSEventManagerInterface](),
	}

	if wsEventManager != nil {
		ret.wsEventManager = mo.Some[events.WSEventManagerInterface](wsEventManager)
	}

	return ret
}

func (u *Updater) GetLatestUpdate() (*Update, error) {
	// Updates are disabled in this fork; return an empty update
	return &Update{Type: ""}, nil
}

func (u *Updater) ShouldRefetchReleases() {
	// No-op when updates are disabled
}

func (u *Updater) SetEnabled(checkForUpdate bool) {
	u.checkForUpdate = checkForUpdate
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetLatestRelease returns the latest release from the GitHub repository.
func (u *Updater) GetLatestRelease() (*Release, error) {
	// Updates are disabled; never fetch releases
	return nil, nil
}

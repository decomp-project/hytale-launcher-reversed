// Package pkg provides game, Java runtime, and launcher package management
// for the Hytale launcher. It handles updates, patching, and version management.
package pkg

import (
	"context"
	"log/slog"
	"sync"

	"hytale-launcher/internal/appstate"
	"hytale-launcher/internal/verget"
)

// Package-level variables for version manifest getters
var (
	gameManifest     *verget.Getter
	javaManifest     *verget.Getter
	launcherManifest *verget.Getter

	initOnce sync.Once
)

// init initializes the version manifest getters for game, java, and launcher.
func init() {
	initOnce.Do(func() {
		// Game manifest is initialized dynamically based on channel
		gameManifest = verget.NewGetter("game", func(ctx context.Context, channel string, fromBuild int) {
			slog.Debug("requesting patch set",
				"channel", channel,
				"current", fromBuild,
			)
			// Fetch patch set from endpoints
		})

		// Java manifest getter
		javaManifest = verget.NewGetter("jre", func(ctx context.Context, channel string, fromBuild int) {
			verget.GetManifest(channel, "jre")
		})

		// Launcher manifest getter
		launcherManifest = verget.NewGetter("launcher", func(ctx context.Context, channel string, fromBuild int) {
			verget.GetManifest(channel, "launcher")
		})
	})
}

// Update represents an update that can be applied.
type Update interface {
	// Apply applies the update with progress reporting.
	Apply(ctx context.Context, state *appstate.State, reporter ProgressReporter) error
}

// ProgressReporter is a callback for reporting update progress.
type ProgressReporter func(status UpdateStatus)

// UpdateStatus represents the current status of an update operation.
type UpdateStatus struct {
	State      string                 `json:"state"`
	StateData  map[string]interface{} `json:"state_data,omitempty"`
	Progress   float64                `json:"progress"`
	Cancelable bool                   `json:"cancelable"`
	Current    int64                  `json:"current,omitempty"`
	Total      int64                  `json:"total,omitempty"`
	Error      error                  `json:"error,omitempty"`
}

// Common update state constants
const (
	StateDownloading          = "downloading"
	StateDownloadingPatch     = "downloading_patch"
	StateDownloadingSignature = "downloading_patch_signature"
	StateApplyingPatch        = "applying_patch"
	StateValidatingPatch      = "validating_patch"
	StateInstalling           = "installing"
	StateCancelled            = "cancelled"
	StateComplete             = "complete"
	StateError                = "error"
)

// cancelableSaveConsumer wraps a context to check for cancellation during save operations.
type cancelableSaveConsumer struct {
	ctx context.Context
}

// ShouldSave returns true if the save operation should proceed (context not cancelled).
func (c *cancelableSaveConsumer) ShouldSave() bool {
	select {
	case <-c.ctx.Done():
		return false
	default:
		return true
	}
}

// Save performs a save operation if the context is not cancelled.
func (c *cancelableSaveConsumer) Save(data []byte) error {
	if !c.ShouldSave() {
		return c.ctx.Err()
	}
	// Perform actual save
	return nil
}

// InvalidateVersionManifests clears all cached version manifests.
// This forces a fresh fetch on the next update check.
func InvalidateVersionManifests() {
	slog.Debug("invalidating all version manifests")
	if gameManifest != nil {
		gameManifest.Invalidate()
	}
	if javaManifest != nil {
		javaManifest.Invalidate()
	}
	if launcherManifest != nil {
		launcherManifest.Invalidate()
	}
}

package pkg

import (
	"context"

	"hytale-launcher/internal/appstate"
)

// CheckAllUpdates checks for updates across all components (game, java, launcher).
func CheckAllUpdates(ctx context.Context, state *appstate.State, auth *Auth, channel string) ([]Update, error) {
	var updates []Update

	// Check for launcher update first
	launcherUpdate, err := CheckForLauncherUpdate(ctx)
	if err != nil {
		return nil, err
	}
	if launcherUpdate != nil {
		updates = append(updates, launcherUpdate)
		// Return early if launcher needs update - it should be applied first
		return updates, nil
	}

	// Check for Java update
	javaUpdate, err := CheckForJavaUpdate(ctx, state, channel)
	if err != nil {
		return nil, err
	}
	if javaUpdate != nil {
		updates = append(updates, javaUpdate)
	}

	// Check for game update
	game := &Game{
		Channel: channel,
		State:   state,
	}
	gameUpdate, err := game.CheckForUpdate(ctx, auth)
	if err != nil {
		return nil, err
	}
	if gameUpdate != nil {
		updates = append(updates, gameUpdate)
	}

	return updates, nil
}

// ApplyUpdates applies a list of updates in order.
func ApplyUpdates(ctx context.Context, state *appstate.State, updates []Update, reporter ProgressReporter) error {
	totalUpdates := len(updates)

	for i, update := range updates {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Create a sub-reporter that scales progress for this update
		baseProgress := float64(i) / float64(totalUpdates)
		updateWeight := 1.0 / float64(totalUpdates)

		subReporter := func(status UpdateStatus) {
			// Scale the progress
			scaledProgress := baseProgress + (status.Progress * updateWeight)
			status.Progress = scaledProgress
			reporter(status)
		}

		if err := update.Apply(ctx, state, subReporter); err != nil {
			return err
		}
	}

	return nil
}

// UpdateType represents the type of update.
type UpdateType int

const (
	UpdateTypeLauncher UpdateType = iota
	UpdateTypeJava
	UpdateTypeGame
)

// GetUpdateType returns the type of the given update.
func GetUpdateType(u Update) UpdateType {
	switch u.(type) {
	case *launcherUpdate:
		return UpdateTypeLauncher
	case *javaUpdate:
		return UpdateTypeJava
	case *gameUpdate:
		return UpdateTypeGame
	default:
		return UpdateTypeGame
	}
}

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	Type           UpdateType
	CurrentVersion string
	TargetVersion  string
	Size           int64
}

// GetUpdateInfo extracts information from an update for display purposes.
func GetUpdateInfo(u Update) UpdateInfo {
	switch v := u.(type) {
	case *launcherUpdate:
		return UpdateInfo{
			Type:           UpdateTypeLauncher,
			CurrentVersion: v.CurrentVersion,
			TargetVersion:  v.TargetVersion,
			Size:           v.Size,
		}
	case *javaUpdate:
		var current string
		if v.CurrentVersion != nil {
			current = v.CurrentVersion.Version
		}
		return UpdateInfo{
			Type:           UpdateTypeJava,
			CurrentVersion: current,
			TargetVersion:  v.TargetVersion,
			Size:           v.Size,
		}
	case *gameUpdate:
		var current string
		if v.CurrentBuild != nil {
			current = v.CurrentBuild.Version
		}
		return UpdateInfo{
			Type:           UpdateTypeGame,
			CurrentVersion: current,
			TargetVersion:  v.Version,
		}
	default:
		return UpdateInfo{}
	}
}

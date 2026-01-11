package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"hytale-launcher/internal/appstate"
	"hytale-launcher/internal/download"
	"hytale-launcher/internal/endpoints"
	"hytale-launcher/internal/eventgroup"
	"hytale-launcher/internal/hytale"
)

// Auth holds authentication state for game update checks.
type Auth struct {
	Token   string
	Account *GameAccount
}

// GameAccount holds account info for game updates.
type GameAccount struct {
	Patchlines map[string]*GamePatchline
}

// GamePatchline represents a game release channel.
type GamePatchline struct {
	Name        string
	Version     string
	NewestBuild int
}

// Game represents a game channel configuration.
type Game struct {
	Channel string
	State   *appstate.State
}

// gameBuild represents a specific game build.
type gameBuild struct {
	Build   int
	Version string
	Hash    string
}

// gamePatch represents a patch between two game builds.
type gamePatch struct {
	FromBuild    int
	ToBuild      int
	PatchURL     string
	PatchSize    int64
	SignatureURL string
	SigSize      int64

	// Downloaded file paths (set during download)
	patchPath string
	sigPath   string
}

// gamePatchSet represents a collection of patches needed to update.
type gamePatchSet struct {
	Steps []*gamePatch `json:"steps"`
}

// gameUpdate represents a pending game update.
type gameUpdate struct {
	Channel      *Game
	CurrentBuild *gameBuild
	TargetBuild  int
	Version      string
	Patches      *gamePatchSet
}

// currentVersion returns the currently installed game version.
func (g Game) currentVersion() *gameBuild {
	dep := g.State.GetDependency("game")
	if dep == nil {
		return nil
	}

	return &gameBuild{
		Build:   dep.Build,
		Version: dep.Version,
		Hash:    dep.Hash,
	}
}

// CheckForUpdate checks if an update is available for the game channel.
func (g *Game) CheckForUpdate(ctx context.Context, auth *Auth) (Update, error) {
	if auth.Account == nil {
		return nil, fmt.Errorf("no authenticated account available for update check")
	}

	// Get patchline info for the channel
	patchline, ok := auth.Account.Patchlines[g.Channel]
	if !ok {
		return nil, fmt.Errorf("no patchline available for channel %s", g.Channel)
	}

	slog.Debug("patchline index",
		"channel", g.Channel,
		"newest_build", patchline.NewestBuild,
		"build_version", patchline.Version,
	)

	if patchline.NewestBuild < 1 {
		return nil, fmt.Errorf("no builds available for channel %s", g.Channel)
	}

	// Check current version
	current := g.currentVersion()
	var currentBuild int
	if current != nil {
		currentBuild = current.Build
	}

	// No update needed if already on latest
	if currentBuild == patchline.NewestBuild {
		return nil, nil
	}

	// Get patches from API
	patches, err := g.getPatchSet(ctx, auth, currentBuild)
	if err != nil {
		return nil, fmt.Errorf("error getting patch set for channel %s: %w", g.Channel, err)
	}

	if len(patches.Steps) == 0 {
		return nil, fmt.Errorf("no patches available for channel %s from build %d", g.Channel, currentBuild)
	}

	return &gameUpdate{
		Channel:      g,
		CurrentBuild: current,
		TargetBuild:  patchline.NewestBuild,
		Version:      patchline.Version,
		Patches:      patches,
	}, nil
}

// getPatchSet retrieves the patches needed to update from the given build.
func (g *Game) getPatchSet(ctx context.Context, auth *Auth, fromBuild int) (*gamePatchSet, error) {
	// Request patch set from endpoint
	_ = endpoints.GamePatchSet(g.Channel, fromBuild)

	// TODO: Implement actual patch set fetching
	var patchSet gamePatchSet

	// Log the patch steps
	steps := make([]string, len(patchSet.Steps))
	for i, step := range patchSet.Steps {
		steps[i] = fmt.Sprintf("%d->%d", step.FromBuild, step.ToBuild)
	}
	slog.Debug("received patch set",
		"channel", g.Channel,
		"patches", steps,
	)

	return &patchSet, nil
}

// download downloads the patch and its signature.
func (p *gamePatch) download(ctx context.Context, idx, total int, reporter ProgressReporter) error {
	baseProgress := float64(idx) / float64(total)
	patchWeight := (1.0 / float64(total)) * 0.9
	sigWeight := (1.0 / float64(total)) * 0.1

	// Download patch file
	slog.Debug("downloading patch",
		"from", p.FromBuild,
		"to", p.ToBuild,
	)

	patchReporter := download.NewReporter(UpdateStatus{
		State: StateDownloadingPatch,
		StateData: map[string]interface{}{
			"current": idx + 1,
			"total":   total,
		},
	}, baseProgress, patchWeight, reporter)

	patchPath, err := download.DownloadTempSimple(ctx, p.PatchURL, patchReporter)
	if err != nil {
		return err
	}
	p.patchPath = patchPath

	slog.Debug("downloaded patch",
		"from", p.FromBuild,
		"to", p.ToBuild,
		"patch", patchPath,
	)

	// Download signature file
	sigReporter := download.NewReporter(UpdateStatus{
		State: StateDownloadingSignature,
		StateData: map[string]interface{}{
			"current": idx + 1,
			"total":   total,
		},
	}, baseProgress+patchWeight, sigWeight, reporter)

	sigPath, err := download.DownloadTempSimple(ctx, p.SignatureURL, sigReporter)
	if err != nil {
		return err
	}
	p.sigPath = sigPath

	slog.Debug("downloaded patch signature",
		"from", p.FromBuild,
		"to", p.ToBuild,
		"sig", sigPath,
	)

	return nil
}

// mkStagingDir creates a temporary staging directory for patch application.
func (p *gamePatch) mkStagingDir() (string, error) {
	// Check for TMPDIR environment variable first
	if tmpDir, ok := os.LookupEnv("TMPDIR"); ok {
		return os.MkdirTemp(tmpDir, "hytale-patch-staging-*")
	}
	// Check for XDG cache directory second
	if cacheDir, ok := os.LookupEnv("XDG_CACHE_HOME"); ok {
		return os.MkdirTemp(cacheDir, "hytale-patch-staging-*")
	}
	// Fall back to system temp directory
	return os.MkdirTemp("", "hytale-patch-staging-*")
}

// apply applies the patch to the game installation.
func (p *gamePatch) apply(ctx context.Context, gameDir string, reporter ProgressReporter) error {
	slog.Info("applying patch",
		"from", p.FromBuild,
		"to", p.ToBuild,
		"patch", p.patchPath,
	)

	// Create staging directory
	stagingDir, err := p.mkStagingDir()
	if err != nil {
		return fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	// Create state consumer for progress reporting
	stateConsumer := newStateConsumer(func(progress float64) {
		reporter(UpdateStatus{
			State:    StateApplyingPatch,
			Progress: progress,
		})
	})

	// Apply the patch using wharf
	if err := applyWharf(ctx, p.patchPath, p.sigPath, gameDir, stagingDir, stateConsumer); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	return nil
}

// validate validates the patched game installation.
func (p *gamePatch) validate(ctx context.Context, gameDir string, reporter ProgressReporter) error {
	slog.Info("validating patch",
		"from", p.FromBuild,
		"to", p.ToBuild,
	)

	stateConsumer := newStateConsumer(func(progress float64) {
		reporter(UpdateStatus{
			State:    StateValidatingPatch,
			Progress: progress,
		})
	})

	// Validate using wharf
	if err := validateWharf(ctx, p.sigPath, gameDir, stateConsumer); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// Apply applies the game update.
func (u *gameUpdate) Apply(ctx context.Context, state *appstate.State, reporter ProgressReporter) error {
	slog.Info("applying game update",
		"channel", u.Channel.Channel,
		"from", u.CurrentBuild,
		"to", u.TargetBuild,
	)

	// Get game directory
	gameDir := hytale.PackageDir("game", u.Channel.Channel, "latest")

	// Download all patches first
	for i, patch := range u.Patches.Steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := patch.download(ctx, i, len(u.Patches.Steps), reporter); err != nil {
			return u.fallback(ctx, state, reporter, err)
		}
	}

	// Apply patches in order
	for i, patch := range u.Patches.Steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := patch.apply(ctx, gameDir, reporter); err != nil {
			return u.fallback(ctx, state, reporter, err)
		}

		if err := patch.validate(ctx, gameDir, reporter); err != nil {
			return u.fallback(ctx, state, reporter, err)
		}

		// Update progress
		progress := float64(i+1) / float64(len(u.Patches.Steps))
		reporter(UpdateStatus{
			State:    StateApplyingPatch,
			Progress: progress,
		})
	}

	// Clean up patch files
	u.deletePatchFiles()

	// Save signature for future validation
	if err := u.saveSig(gameDir); err != nil {
		slog.Warn("failed to save signature", "error", err)
	}

	// Demote old versions
	u.demoteOldVersions(state)

	// Update dependency state
	state.SetDependency("game", "update", &appstate.Dep{
		Build:   u.TargetBuild,
		Version: u.Version,
	})

	reporter(UpdateStatus{
		State:    StateComplete,
		Progress: 1.0,
	})

	return nil
}

// fallback handles a failed update by attempting recovery.
func (u *gameUpdate) fallback(ctx context.Context, state *appstate.State, reporter ProgressReporter, originalErr error) error {
	slog.Error("update failed, attempting recovery",
		"error", originalErr,
	)

	// Clean up patch files
	u.deletePatchFiles()

	// For now, just return the original error
	// Future: could implement full re-download fallback
	return originalErr
}

// deletePatchFiles removes downloaded patch files.
func (u *gameUpdate) deletePatchFiles() {
	// Use event group to delete files in parallel
	var eg eventgroup.Group

	for _, patch := range u.Patches.Steps {
		p := patch // capture for closure
		eg.Go(func() error {
			if p.patchPath != "" {
				if err := os.Remove(p.patchPath); err != nil && !os.IsNotExist(err) {
					slog.Warn("failed to remove patch file",
						"path", p.patchPath,
						"error", err,
					)
				}
			}
			return nil
		})

		if p.sigPath != "" {
			eg.Go(func() error {
				if err := os.Remove(p.sigPath); err != nil && !os.IsNotExist(err) {
					slog.Warn("failed to remove signature file",
						"path", p.sigPath,
						"error", err,
					)
				}
				return nil
			})
		}
	}

	_ = eg.Wait()
}

// relBinaryPath returns the relative path to the game binary.
func (u *gameUpdate) relBinaryPath() string {
	// Platform-specific binary path
	return filepath.Join("bin", "hytale")
}

// saveSig saves the final signature file for future validation.
func (u *gameUpdate) saveSig(gameDir string) error {
	if len(u.Patches.Steps) == 0 {
		return nil
	}

	lastPatch := u.Patches.Steps[len(u.Patches.Steps)-1]
	if lastPatch.sigPath == "" {
		return nil
	}

	sigDest := filepath.Join(gameDir, ".signature")
	return os.Rename(lastPatch.sigPath, sigDest)
}

// demoteOldVersions marks old game versions as non-latest.
func (u *gameUpdate) demoteOldVersions(state *appstate.State) {
	// Implementation depends on version management strategy
	slog.Debug("demoting old game versions",
		"channel", u.Channel.Channel,
	)
}

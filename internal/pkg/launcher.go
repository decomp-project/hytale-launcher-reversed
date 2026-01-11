package pkg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"hytale-launcher/internal/appstate"
	"hytale-launcher/internal/build"
	"hytale-launcher/internal/crypto"
	"hytale-launcher/internal/download"
	"hytale-launcher/internal/fork"
	"hytale-launcher/internal/ioutil"
)

// launcherUpdate represents a pending launcher update.
type launcherUpdate struct {
	Channel        string
	CurrentVersion string
	CurrentBuild   int
	TargetVersion  string
	TargetBuild    int
	DownloadURL    string
	Hash           string
	Size           int64
}

// CheckForLauncherUpdate checks if a launcher update is available.
func CheckForLauncherUpdate(ctx context.Context) (Update, error) {
	// Get current launcher version
	currentVersion := build.Version
	currentBuild := build.BuildNumber

	// Get manifest for latest version using the getter
	cached, err := launcherManifest.Get(ctx, build.Release)
	if err != nil {
		return nil, fmt.Errorf("failed to get launcher manifest: %w", err)
	}

	// Check if update is needed
	if currentBuild >= cached.Build {
		slog.Debug("launcher is up to date",
			"current", currentBuild,
			"latest", cached.Build,
		)
		return nil, nil
	}

	slog.Info("launcher update available",
		"current_version", currentVersion,
		"current_build", currentBuild,
		"target_version", cached.Version,
		"target_build", cached.Build,
	)

	return &launcherUpdate{
		Channel:        build.Release,
		CurrentVersion: currentVersion,
		CurrentBuild:   currentBuild,
		TargetVersion:  cached.Version,
		TargetBuild:    cached.Build,
		DownloadURL:    cached.URL,
		Hash:           cached.Hash,
		Size:           cached.Size,
	}, nil
}

// Apply applies the launcher update.
func (u *launcherUpdate) Apply(ctx context.Context, state *appstate.State, reporter ProgressReporter) error {
	slog.Info("applying launcher update",
		"from", u.CurrentVersion,
		"to", u.TargetVersion,
	)

	// Download new launcher binary
	downloadReporter := download.NewReporter(UpdateStatus{
		State: StateDownloading,
		StateData: map[string]interface{}{
			"component": "launcher",
			"version":   u.TargetVersion,
		},
	}, 0, 0.8, reporter)

	newBinaryPath, err := download.DownloadTempSimple(u.DownloadURL, downloadReporter)
	if err != nil {
		return fmt.Errorf("failed to download launcher: %w", err)
	}

	// Validate the new binary before applying
	reporter(UpdateStatus{
		State:    StateInstalling,
		Progress: 0.8,
	})

	if err := u.validateBin(ctx, newBinaryPath); err != nil {
		os.Remove(newBinaryPath)
		return fmt.Errorf("launcher validation failed: %w", err)
	}

	// Perform self-update
	if err := u.selfUpdate(ctx, newBinaryPath); err != nil {
		os.Remove(newBinaryPath)
		return fmt.Errorf("self-update failed: %w", err)
	}

	// Note: selfUpdate typically does not return as it spawns a new process
	// and exits the current one

	reporter(UpdateStatus{
		State:    StateComplete,
		Progress: 1.0,
	})

	return nil
}

// validateBin validates the launcher binary by running it with -test flag.
func (u *launcherUpdate) validateBin(ctx context.Context, binPath string) error {
	// Make the binary executable
	if err := ioutil.MakeExecutable(binPath); err != nil {
		return err
	}

	// Run with -test flag to verify functionality
	cmd := exec.CommandContext(ctx, binPath, "-test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("launcher test failed",
			"output", string(output),
			"error", err,
		)
		return errors.New("hytale-launcher binary is not functional")
	}

	return nil
}

// selfUpdate performs a self-update by spawning a helper process.
func (u *launcherUpdate) selfUpdate(ctx context.Context, newBinaryPath string) error {
	// Load self-update key for signing the update request
	key, err := crypto.LoadSelfUpdateKey()
	if err != nil {
		return fmt.Errorf("failed to load self-update key: %w", err)
	}

	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return err
	}

	// Get current process ID
	pid := syscall.Getpid()
	pidStr := strconv.FormatInt(int64(pid), 10)

	// Create HMAC signature for verification
	sig := crypto.HMAC([]byte(pidStr), key)

	// Build arguments for the update helper process
	args := []string{
		"-start-pid", pidStr,
		"-source-exe", newBinaryPath,
		"-dest-exe", currentExe,
		"-launcher-patchline", build.Release,
		"-launcher-version", u.TargetVersion,
		"-sig", sig,
	}

	slog.Info("spawning update helper process",
		"bin", newBinaryPath,
		"args", args,
	)

	// Run the new binary with elevated privileges if needed
	if _, err := fork.RunElevated(newBinaryPath, args); err != nil {
		return err
	}

	// Exit current process to allow update to complete
	os.Exit(0)

	// This line is never reached
	return nil
}

// Populate fills in missing launcher update information from manifest.
func (u *launcherUpdate) Populate(ctx context.Context) error {
	cached, err := launcherManifest.Get(ctx, build.Release)
	if err != nil {
		return err
	}

	u.DownloadURL = cached.URL
	u.Hash = cached.Hash
	u.Size = cached.Size
	u.TargetVersion = cached.Version
	u.TargetBuild = cached.Build

	return nil
}

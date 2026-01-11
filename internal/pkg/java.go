package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"hytale-launcher/internal/appstate"
	"hytale-launcher/internal/build"
	"hytale-launcher/internal/download"
	"hytale-launcher/internal/hytale"
	"hytale-launcher/internal/ioutil"

	"github.com/getsentry/sentry-go"
)

// javaUpdate represents a pending Java runtime update.
type javaUpdate struct {
	Channel        string
	CurrentVersion *appstate.Dep
	TargetVersion  string
	TargetBuild    int
	DownloadURL    string
	Hash           string
	Size           int64
}

// CheckForJavaUpdate checks if a Java runtime update is available.
func CheckForJavaUpdate(ctx context.Context, state *appstate.State, channel string) (Update, error) {
	// Get current Java version
	current := state.GetDependency("jre")

	// Get manifest for latest version using the getter
	cached, err := javaManifest.Get(ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("failed to get Java manifest: %w", err)
	}

	// Check if update is needed
	if current != nil && current.Build >= cached.Build {
		slog.Debug("Java is up to date",
			"current", current.Build,
			"latest", cached.Build,
		)
		return nil, nil
	}

	slog.Info("Java update available",
		"current", current,
		"target", cached.Build,
		"version", cached.Version,
	)

	return &javaUpdate{
		Channel:        channel,
		CurrentVersion: current,
		TargetVersion:  cached.Version,
		TargetBuild:    cached.Build,
		DownloadURL:    cached.URL,
		Hash:           cached.Hash,
		Size:           cached.Size,
	}, nil
}

// Apply applies the Java runtime update.
func (u *javaUpdate) Apply(ctx context.Context, state *appstate.State, reporter ProgressReporter) error {
	slog.Info("applying Java update",
		"version", u.TargetVersion,
		"build", u.TargetBuild,
	)

	// Uninstall old version first
	u.uninstall(ctx, state)

	// Get Java installation directory
	javaDir := hytale.PackageDir("jre", u.Channel, "latest")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(javaDir, 0755); err != nil {
		return fmt.Errorf("failed to create Java directory: %w", err)
	}

	// Download Java archive
	downloadReporter := download.NewReporter(UpdateStatus{
		State: StateDownloading,
		StateData: map[string]interface{}{
			"component": "jre",
			"version":   u.TargetVersion,
		},
	}, 0, 0.8, reporter)

	archivePath, err := download.DownloadTempSimple(u.DownloadURL, downloadReporter)
	if err != nil {
		return fmt.Errorf("failed to download Java: %w", err)
	}
	defer os.Remove(archivePath)

	// Extract archive
	reporter(UpdateStatus{
		State:    StateInstalling,
		Progress: 0.8,
	})

	if err := ioutil.ExtractArchive(archivePath, javaDir); err != nil {
		return fmt.Errorf("failed to extract Java: %w", err)
	}

	// Get Java binary path
	javaBin := u.javaBinaryPath(javaDir)

	// Make binary executable
	if err := ioutil.MakeExecutable(javaBin); err != nil {
		return fmt.Errorf("failed to make Java executable: %w", err)
	}

	// Validate the installation
	if err := u.validateBin(ctx, javaBin); err != nil {
		// Clean up on failure
		os.RemoveAll(javaDir)
		return fmt.Errorf("Java validation failed: %w", err)
	}

	// Update dependency state
	state.SetDependency("jre", u.Channel, &appstate.Dep{
		Build:   u.TargetBuild,
		Version: u.TargetVersion,
		Hash:    u.Hash,
	})

	reporter(UpdateStatus{
		State:    StateComplete,
		Progress: 1.0,
	})

	slog.Info("Java update complete",
		"version", u.TargetVersion,
	)

	return nil
}

// uninstall removes the old Java installation.
func (u *javaUpdate) uninstall(ctx context.Context, state *appstate.State) {
	if u.CurrentVersion == nil {
		return
	}

	javaDir := hytale.PackageDir("jre", u.Channel, "latest")

	if err := os.RemoveAll(javaDir); err != nil {
		sentry.CaptureException(err)
		slog.Warn("failed to remove old java installation",
			"version", u.CurrentVersion.Version,
			"error", err,
			"dir", javaDir,
		)
	}

	// Clear the dependency
	state.SetDependency("jre", u.Channel, nil)
}

// validateBin validates the Java binary by running it with --version.
func (u *javaUpdate) validateBin(ctx context.Context, javaBin string) error {
	// Skip validation in dev mode if environment variable is set
	if build.IsDev() {
		if _, ok := os.LookupEnv("HYTALE_LAUNCHER_NO_TEST_RUN_BINARIES"); ok {
			slog.Debug("skipping binary test run",
				"bin", javaBin,
			)
			return nil
		}
	}

	slog.Debug("validating Java binary",
		"bin", javaBin,
	)

	// Create process with stdin/stdout/stderr
	cmd := exec.CommandContext(ctx, javaBin, "--version")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start java process: %w", err)
	}

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("java validation failed with exit code %d", exitErr.ExitCode())
		}
		return err
	}

	return nil
}

// javaBinaryPath returns the path to the Java binary within the installation directory.
func (u *javaUpdate) javaBinaryPath(javaDir string) string {
	// Platform-specific path
	return filepath.Join(javaDir, "bin", "java")
}

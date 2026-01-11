package selfupdate

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"hytale-launcher/internal/crypto"
	"hytale-launcher/internal/fork"
	"hytale-launcher/internal/ioutil"
	"hytale-launcher/internal/keyring"
)

// Package-level variables set by the build system or runtime configuration.
// These define the source and target executables for self-update operations.
var (
	// SourceBin is the path to the new binary to update from.
	SourceBin string
	// TargetBin is the path to the current binary to update.
	TargetBin string
	// OldChannel is the channel of the old launcher version.
	OldChannel string
	// OldVersion is the version string of the old launcher.
	OldVersion string
	// ParentPID is the PID of the parent process to wait for before updating.
	ParentPID int
	// UpdateSignature is the expected HMAC signature of the update.
	UpdateSignature string
)

const (
	// cleanupNoteKeyName is the keyring key name for encrypting the cleanup note.
	cleanupNoteKeyName = "selfupdate"
	// updateKeyName is the keyring key name for validating update signatures.
	updateKeyName = "selfupdate"
	// processWaitTimeout is how long to wait for the parent process to exit.
	processWaitTimeout = 30 * time.Second
	// processCheckInterval is how often to check if the parent process has exited.
	processCheckInterval = 100 * time.Millisecond
)

// updateKey holds the cached encryption key for update validation.
var updateKey []byte

// init pre-fetches the update key from the keyring.
func init() {
	key, err := keyring.GetOrGenKey(updateKeyName)
	if err != nil {
		return
	}
	updateKey = key
}

// replaceBin copies the contents of the source binary to the target path.
func replaceBin(from, to string) error {
	slog.Debug("replacing binary", "from", from, "to", to)

	data, err := os.ReadFile(from)
	if err != nil {
		return fmt.Errorf("error reading source binary: %w", err)
	}

	if err := os.WriteFile(to, data, 0644); err != nil {
		return fmt.Errorf("error writing destination binary: %w", err)
	}

	return nil
}

// updateBin removes the existing target binary and replaces it with the source.
func updateBin() error {
	slog.Info("updating binary", "from", SourceBin, "to", TargetBin)

	if err := os.Remove(TargetBin); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Error("failed to remove existing executable", "error", err)
			return err
		}
	}

	return replaceBin(SourceBin, TargetBin)
}

// isSet checks if a string pointer is non-nil and non-empty.
func isSet(s string) bool {
	return s != ""
}

// waitForProcessExit waits for the process with the given PID to exit.
// It will timeout after processWaitTimeout and continue anyway.
func waitForProcessExit(pid int) {
	slog.Info("waiting for parent process to exit", "pid", pid)

	deadline := time.Now().Add(processWaitTimeout)

	for time.Now().Before(deadline) {
		if !processExists(pid) {
			slog.Debug("parent process has exited", "pid", pid)
			return
		}
		time.Sleep(processCheckInterval)
	}

	slog.Warn("timed out waiting for parent process to exit", "pid", pid)
}

// validate checks that the update is valid by verifying the HMAC signature
// and ensuring the source and target binaries have valid paths.
func validate(key []byte) error {
	// Compute HMAC of the target binary path
	targetPath := []byte(TargetBin)
	computed := crypto.HMAC(targetPath, key)

	// Verify the signature matches
	if computed != UpdateSignature {
		return errors.New("invalid update signature")
	}

	// Validate that source and target paths start with "/tmp" prefix
	// The update binaries should be placed in a temp directory
	if strings.HasPrefix(SourceBin, "/tmp") && strings.HasPrefix(TargetBin, "/tmp") {
		return nil
	}

	return errors.New("invalid update executables")
}

// fetchUpdateKey retrieves the update validation key.
var fetchUpdateKey = func() ([]byte, error) {
	if updateKey != nil {
		return updateKey, nil
	}
	return keyring.GetOrGenKey(updateKeyName)
}

// Do performs the self-update process.
// It validates the update, waits for the parent process to exit,
// replaces the binary, makes it executable, writes a cleanup note,
// and launches the updated process.
func Do() {
	// Check if update parameters are set
	if !isSet(SourceBin) || !isSet(TargetBin) {
		return
	}

	// Fetch the update key
	key, err := fetchUpdateKey()
	if err != nil {
		slog.Error("error fetching self update key", "error", err)
		return
	}

	// Validate the update
	if err := validate(key); err != nil {
		slog.Error("update validation failed", "error", err)
		return
	}

	slog.Info("performing update", "source", SourceBin, "target", TargetBin)

	// Wait for parent process to exit if PID is set
	if ParentPID > 0 {
		waitForProcessExit(ParentPID)
	}

	// Perform the binary update
	if err := updateBin(); err != nil {
		slog.Error("failed to update", "error", err)
		return
	}

	// Make the new binary executable
	if err := ioutil.MakeExecutable(TargetBin); err != nil {
		slog.Error("failed to make binary executable", "error", err)
		return
	}

	// Write cleanup note if old version info is available
	if isSet(OldChannel) && isSet(OldVersion) {
		note := &cleanupNote{
			Channel: OldChannel,
			Version: OldVersion,
		}
		if err := note.WriteFile(); err != nil {
			slog.Error("failed to write self-update note file", "error", err)
		}
	}

	// Launch the updated process
	slog.Info("launching updated process", "path", TargetBin)

	if _, err := fork.RunAsUser(TargetBin); err != nil {
		slog.Error("failed to launch target exec", "error", err)
		return
	}

	// Exit the current process
	os.Exit(0)
}

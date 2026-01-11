// Package hytale provides common utilities for the Hytale launcher.
package hytale

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/getsentry/sentry-go"
)

// getDefaultAppDataDir returns the default application data directory.
// On Linux, this is XDG_DATA_HOME or ~/.local/share if not set.
func getDefaultAppDataDir() (string, error) {
	// Check XDG_DATA_HOME first
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}

	// Fall back to ~/.local/share
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

// getUserAppDataDir returns the user's Hytale application data directory.
func getUserAppDataDir() (string, error) {
	dir, err := getDefaultAppDataDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine default app data directory: %w", err)
	}
	return filepath.Join(dir, "hytale"), nil
}

var storageDir = sync.OnceValue(func() string {
	path, err := getUserAppDataDir()
	if err != nil {
		wrappedErr := fmt.Errorf("unable to determine hytale storage directory: %v", err)
		sentry.CaptureException(wrappedErr)
		panic(wrappedErr)
	}

	slog.Info("selected hytale storage directory", "path", path)
	return path
})

// StorageDir returns the Hytale storage directory path.
// This function is safe to call concurrently and will only compute
// the path once.
func StorageDir() string {
	return storageDir()
}

// InStorageDir returns the full path to a file or directory within the storage directory.
func InStorageDir(name string) string {
	return filepath.Join(storageDir(), name)
}

// DataDir returns the Hytale data directory path.
// This is an alias for StorageDir for backwards compatibility.
func DataDir() string {
	return storageDir()
}

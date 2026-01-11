package build

import (
	"os"
	"runtime"
)

// OS returns the target operating system.
// In dev mode, it first checks the HYTALE_LAUNCHER_OS environment variable.
// Otherwise, it returns the runtime OS (runtime.GOOS).
func OS() string {
	if isDevMode() {
		if v, ok := os.LookupEnv("HYTALE_LAUNCHER_OS"); ok {
			return v
		}
	}
	return runtime.GOOS
}

// Arch returns the target architecture.
// In dev mode, it first checks the HYTALE_LAUNCHER_ARCH environment variable.
// Otherwise, it returns the runtime architecture (runtime.GOARCH).
func Arch() string {
	if isDevMode() {
		if v, ok := os.LookupEnv("HYTALE_LAUNCHER_ARCH"); ok {
			return v
		}
	}
	return runtime.GOARCH
}

// OfflineMode returns true if offline mode is enabled via environment variable.
// This is only checked in dev mode.
func OfflineMode() bool {
	if isDevMode() {
		_, ok := os.LookupEnv("HYTALE_LAUNCHER_OFFLINE_MODE")
		return ok
	}
	return false
}

// DebugLogging returns true if debug logging is enabled.
// In dev mode, debug logging is always enabled.
// In other modes, it checks the HYTALE_LAUNCHER_DEBUG_LOGGING environment variable.
func DebugLogging() bool {
	if isDevMode() {
		return true
	}
	_, ok := os.LookupEnv("HYTALE_LAUNCHER_DEBUG_LOGGING")
	return ok
}

// TestRunBinaries returns true if test run binaries should be executed.
// In dev mode, this can be disabled via the HYTALE_LAUNCHER_NO_TEST_RUN_BINARIES
// environment variable.
func TestRunBinaries() bool {
	if isDevMode() {
		_, ok := os.LookupEnv("HYTALE_LAUNCHER_NO_TEST_RUN_BINARIES")
		return !ok
	}
	return true
}

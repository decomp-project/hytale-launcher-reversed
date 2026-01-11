// Package appstate manages the persistent application state for the Hytale launcher.
// It handles state loading, saving, and dependency tracking.
package appstate

import (
	"log/slog"
	"path/filepath"

	"hytale-launcher/internal/build"
	"hytale-launcher/internal/logging"
)

// State represents the persistent application state.
type State struct {
	Channel      string                    `json:"channel"`
	IsNew        bool                      `json:"is_new,omitempty"`
	Platform     *build.Platform           `json:"platform,omitempty"`
	Dependencies map[string]map[string]Dep `json:"dependencies,omitempty"`
	OfflineReady bool                      `json:"offline_ready,omitempty"`
	DataDir      string                    `json:"data_dir,omitempty"`
}

// Dep represents a dependency with version, path, and signature information.
type Dep struct {
	Version string `json:"version"`
	Build   int    `json:"build"`
	BuildID int    `json:"build_id"`
	Hash    string `json:"hash,omitempty"`
	Path    string `json:"path,omitempty"`
	SigDir  string `json:"sig_dir,omitempty"`
	SigFile string `json:"sig_file,omitempty"`
}

// Auth represents authentication state for API requests.
type Auth struct {
	Token   string
	Account *Account
}

// Account represents account information for authentication.
type Account struct {
	Patchlines map[string]*Patchline `json:"patchlines"`
}

// Patchline represents a game patchline/channel configuration.
type Patchline struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	NewestBuild int    `json:"newest_build"`
}

// SigPath returns the full signature file path by joining SigDir and SigFile.
// Returns empty string if SigDir is empty.
func (d *Dep) SigPath() string {
	if d.SigDir == "" {
		return ""
	}
	return filepath.Join(d.SigDir, d.SigFile)
}

// ensureDeps initializes the Dependencies map if it is nil.
func (s *State) ensureDeps() {
	if s.Dependencies == nil {
		s.Dependencies = make(map[string]map[string]Dep)
	}
}

// getDeps returns the dependency map for a given package identifier.
// Returns nil if the Dependencies map is nil or the identifier doesn't exist.
func (s *State) getDeps(identifier string) map[string]Dep {
	if s.Dependencies == nil {
		return nil
	}
	return s.Dependencies[identifier]
}

// SetDependency sets or removes a dependency for a given identifier.
// If dep is nil, the dependency entry for the identifier is removed.
// Otherwise, the dependency is added or updated using the dep's Version as the key.
func (s *State) SetDependency(identifier string, cause string, dep *Dep) {
	slog.Debug("setting dependency",
		"identifier", identifier,
		"version", logging.StringPtr(versionFromDep(dep)),
	)

	if dep == nil {
		delete(s.Dependencies, identifier)
		return
	}

	s.ensureDeps()
	deps := s.getDeps(identifier)
	if deps == nil {
		deps = make(map[string]Dep)
		s.Dependencies[identifier] = deps
	}

	deps[dep.Version] = *dep
}

// AddDependency adds a new dependency for a given identifier.
// The dependency is keyed by its Version field.
func (s *State) AddDependency(identifier string, dep Dep) {
	slog.Debug("adding dependency",
		"identifier", identifier,
		"version", dep.Version,
	)

	s.ensureDeps()
	deps := s.getDeps(identifier)
	if deps == nil {
		deps = make(map[string]Dep)
		s.Dependencies[identifier] = deps
	}

	deps[dep.Version] = dep
}

// GetDeps returns the dependency map for a given identifier.
// Returns nil if no dependencies exist for the identifier.
func (s *State) GetDeps(identifier string) map[string]Dep {
	return s.getDeps(identifier)
}

// GetDependency returns the first dependency for a given identifier.
// Returns nil if no dependencies exist for the identifier.
// This is a convenience method when only one dependency is expected.
func (s *State) GetDependency(identifier string) *Dep {
	deps := s.getDeps(identifier)
	if deps == nil {
		return nil
	}
	for _, dep := range deps {
		d := dep // Create a copy to avoid returning a pointer to the loop variable
		return &d
	}
	return nil
}

// RemoveDependency removes a specific version of a dependency for a given identifier.
// If the identifier's dependency map becomes empty, the identifier entry is also removed.
func (s *State) RemoveDependency(identifier string, version string) {
	slog.Debug("removing dependency",
		"identifier", identifier,
		"version", version,
	)

	deps := s.getDeps(identifier)
	if deps == nil {
		return
	}

	delete(deps, version)

	if len(deps) == 0 {
		delete(s.Dependencies, identifier)
	}
}

// versionFromDep returns a pointer to the Version field if dep is not nil.
func versionFromDep(dep *Dep) *string {
	if dep == nil {
		return nil
	}
	return &dep.Version
}

// Package verget provides functionality for retrieving version manifests
// from the Hytale launcher API.
package verget

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"hytale-launcher/internal/endpoints"
	"hytale-launcher/internal/ioutil"
	"hytale-launcher/internal/net"
)

// FetchFunc is a callback for fetching patch/version data.
type FetchFunc func(ctx context.Context, channel string, fromBuild int)

// Getter provides cached version manifest retrieval.
type Getter struct {
	component string
	fetch     FetchFunc
	mu        sync.RWMutex
	cache     *CachedManifest
}

// CachedManifest holds a cached manifest with metadata.
type CachedManifest struct {
	Manifest *Manifest
	Build    int
	Version  string
	URL      string
	Hash     string
	Size     int64
}

// NewGetter creates a new version manifest getter for a component.
func NewGetter(component string, fetch FetchFunc) *Getter {
	return &Getter{
		component: component,
		fetch:     fetch,
	}
}

// Invalidate clears the cached manifest.
func (g *Getter) Invalidate() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cache = nil
}

// Get returns the cached manifest or fetches a new one.
func (g *Getter) Get(ctx context.Context, channel string) (*CachedManifest, error) {
	g.mu.RLock()
	if g.cache != nil {
		defer g.mu.RUnlock()
		return g.cache, nil
	}
	g.mu.RUnlock()

	// Fetch new manifest
	manifest, err := GetManifest(channel, g.component)
	if err != nil {
		return nil, err
	}

	cached := &CachedManifest{
		Manifest: manifest,
		Version:  manifest.Version,
	}

	g.mu.Lock()
	g.cache = cached
	g.mu.Unlock()

	return cached, nil
}

// Platform represents the target operating system.
type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformDarwin  Platform = "darwin"
	PlatformLinux   Platform = "linux"
)

// Arch represents the target CPU architecture.
type Arch string

const (
	ArchAMD64 Arch = "amd64"
	ArchARM64 Arch = "arm64"
)

// Release contains download information for a specific platform/arch combination.
type Release struct {
	// URL is the download URL for this release.
	URL string `json:"url"`

	// Checksum is the SHA256 hash of the download.
	Checksum string `json:"checksum"`

	// Size is the download size in bytes.
	Size int64 `json:"size"`
}

// Manifest represents version information for a component.
// It contains the version string and download URLs for each platform/arch combination.
type Manifest struct {
	// Version is the version string for this manifest.
	Version string `json:"version"`

	// DownloadURL maps platform -> arch -> release info.
	DownloadURL map[Platform]map[Arch]Release `json:"download_url"`
}

// GetRelease returns the release info for a specific platform and architecture.
// Returns nil if no release is available for the given combination.
func (m *Manifest) GetRelease(platform Platform, arch Arch) *Release {
	if m.DownloadURL == nil {
		return nil
	}

	archMap, ok := m.DownloadURL[platform]
	if !ok {
		return nil
	}

	release, ok := archMap[arch]
	if !ok {
		return nil
	}

	return &release
}

// GetManifest fetches the version manifest for a given channel and component.
// The channel is typically "release" or "beta".
// The component is the name of the software component (e.g., "launcher", "jre").
//
// Returns net.ErrOffline if the launcher is in offline mode.
func GetManifest(channel, component string) (*Manifest, error) {
	// Check offline mode first
	if err := net.OfflineError(); err != nil {
		return nil, err
	}

	manifestURL := endpoints.LauncherVersion(channel, component)

	manifest, err := ioutil.Get[Manifest](nil, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest for %s/%s: %w", channel, component, err)
	}

	return &manifest, nil
}

// GetManifestWithClient fetches the version manifest using a custom HTTP client.
// This is useful when authentication or custom transport is needed.
//
// Returns net.ErrOffline if the launcher is in offline mode.
func GetManifestWithClient(client *http.Client, channel, component string, params url.Values) (*Manifest, error) {
	// Check offline mode first
	if err := net.OfflineError(); err != nil {
		return nil, err
	}

	manifestURL := endpoints.LauncherVersion(channel, component)

	manifest, err := ioutil.Get[Manifest](client, manifestURL, params)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest for %s/%s: %w", channel, component, err)
	}

	return &manifest, nil
}

// GetLatestVersion fetches just the version string for a component.
// This is a convenience method that only extracts the version from the manifest.
func GetLatestVersion(channel, component string) (string, error) {
	manifest, err := GetManifest(channel, component)
	if err != nil {
		return "", err
	}
	return manifest.Version, nil
}

// GetDownloadInfo fetches the download information for a specific component,
// platform, and architecture combination.
func GetDownloadInfo(channel, component string, platform Platform, arch Arch) (*Release, error) {
	manifest, err := GetManifest(channel, component)
	if err != nil {
		return nil, err
	}

	release := manifest.GetRelease(platform, arch)
	if release == nil {
		return nil, fmt.Errorf("no release available for %s/%s on %s/%s",
			channel, component, platform, arch)
	}

	return release, nil
}

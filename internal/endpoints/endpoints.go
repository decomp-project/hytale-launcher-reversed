// Package endpoints generates API endpoint URLs for the Hytale launcher.
package endpoints

import (
	"fmt"

	"hytale-launcher/internal/build"
)

// Domain is the base domain for all API endpoints.
// This is set at build time via ldflags:
//
//	-ldflags "-X hytale-launcher/internal/endpoints.Domain=hytale.com"
var Domain string

func init() {
	// In release builds, the domain is already set via ldflags.
	if build.Release == "release" {
		return
	}
	// For dev/test builds, default to production domain if not set.
	if Domain == "" {
		Domain = "hytale.com"
	}
}

// FeedBase returns the base URL for the launcher news feed.
// The returned URL is in the format: https://launcher.{domain}/launcher-feed/{release}/
func FeedBase() string {
	return fmt.Sprintf("https://launcher.%s/launcher-feed/%s/", Domain, build.Release)
}

// Feed returns the full URL for the launcher news feed JSON file.
func Feed() string {
	return FeedBase() + "feed.json"
}

// LauncherVersion returns the URL for fetching launcher/component version manifests.
// Parameters:
//   - platform: the platform identifier (e.g., "windows", "darwin", "linux")
//   - component: the component name (e.g., "launcher", "jre")
func LauncherVersion(platform, component string) string {
	return fmt.Sprintf("https://launcher.%s/version/%s/%s.json", Domain, platform, component)
}

// GamePatchSet returns the URL for fetching game patch information.
// Parameters:
//   - channel: the release channel (e.g., "release", "beta")
//   - version: the patch version number
func GamePatchSet(channel string, version int) string {
	return fmt.Sprintf("https://account-data.%s/patches/%s/%s/%s/%d",
		Domain,
		build.OS(),
		build.Arch(),
		channel,
		version,
	)
}

// LauncherData returns the URL for fetching account launcher data.
// This includes profile, patchline, and EULA information.
func LauncherData() string {
	return fmt.Sprintf("https://account-data.%s/launcher-data", Domain)
}

// OAuthBase returns the base URL for the OAuth authorization server.
func OAuthBase() string {
	return fmt.Sprintf("https://oauth.accounts.%s", Domain)
}

// OAuthAuth returns the OAuth authorization endpoint URL.
func OAuthAuth() string {
	return OAuthBase() + "/oauth2/auth"
}

// OAuthToken returns the OAuth token endpoint URL.
func OAuthToken() string {
	return OAuthBase() + "/oauth2/token"
}

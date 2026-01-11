package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	"golang.org/x/oauth2"

	"hytale-launcher/internal/account"
	"hytale-launcher/internal/appstate"
	"hytale-launcher/internal/auth"
	"hytale-launcher/internal/net"
	"hytale-launcher/internal/news"
	"hytale-launcher/internal/oauth"
	"hytale-launcher/internal/pkg"
)

// currentLoopback holds the active login attempt
var currentLoopback *oauth.Loopback

// CheckForUpdates checks for available updates for the current channel.
// If force is true, it will refresh user data and invalidate version manifests.
// Returns the number of updates found, or -1 if an error occurred.
func (a *App) CheckForUpdates(force bool) int {
	// Ensure we have a valid update environment.
	if a.State == nil || a.Updater == nil {
		slog.Warn("cannot check for updates: no update environment configured")
		return -1
	}

	if force {
		// Check network connectivity and potentially go online.
		if offline := a.CheckNetworkMode(true, "CheckForUpdates"); offline {
			return -1
		}

		// Invalidate cached version manifests.
		pkg.InvalidateVersionManifests()

		// Force refresh user data.
		a.refreshUser(true, "check_for_updates")

		// Refresh the news feed.
		a.RefreshNewsFeed()
	}

	// Check for updates using the updater.
	count, err := a.Updater.CheckForUpdates(a.State, a.Auth)
	if err != nil {
		sentry.CaptureException(err)
		slog.Error("error checking for updates", "error", err)
		return -1
	}

	slog.Info("update check complete",
		"updates_found", count,
		"force", force,
		"channel", a.State.Channel,
	)

	return count
}

// CheckNetworkMode checks if the network is available and updates the mode accordingly.
// If canGoOnline is true and connectivity is available, it will switch to online mode.
// Returns true if the launcher is currently in offline mode.
func (a *App) CheckNetworkMode(canGoOnline bool, cause string) bool {
	slog.Debug("checking network mode", "can_go_online", canGoOnline, "cause", cause)

	// Check for connectivity.
	connected := net.CheckConnectivity()

	currentMode := net.Current()

	if connected && canGoOnline && currentMode == net.ModeOffline {
		// We're offline but have connectivity and permission to go online.
		a.setNetMode(net.ModeOnline, nil)
		return false
	}

	if !connected && currentMode == net.ModeOnline {
		// We were online but lost connectivity.
		a.setNetMode(net.ModeOffline, nil)
		return true
	}

	return currentMode == net.ModeOffline
}

// SetUserProfile changes the current user's active profile.
// It validates the profile UUID and updates the account state.
func (a *App) SetUserProfile(uuid string) error {
	acct := a.Auth.GetAccount()
	if acct == nil {
		return errors.New("no user logged in")
	}

	currentProfile := acct.GetCurrentProfile()

	currentUUID := ""
	if currentProfile != nil {
		currentUUID = currentProfile.UUID
	}

	slog.Debug("requested set user profile",
		"uuid", uuid,
		"current", currentUUID,
	)

	// If already on this profile, do nothing.
	if currentProfile != nil && currentProfile.UUID == uuid {
		return nil
	}

	// Set the new current profile.
	if err := acct.SetCurrentProfile(uuid); err != nil {
		sentry.CaptureException(err)
		slog.Error("error setting user profile",
			"error", err,
			"uuid", uuid,
			"profiles", acct.Profiles,
		)
		return err
	}

	// Ensure the current channel is still valid for this profile.
	a.ensureValidChannel(a.getCurrentChannel())

	// Only save if the profile actually changed.
	if currentProfile == nil || currentProfile.UUID != uuid {
		a.Auth.SaveAccount("set_user_profile")
	}

	// Notify the frontend.
	a.Emit("profile_changed")

	return nil
}

// selectDefaultProfile ensures a profile is selected.
// If the current profile is invalid, it selects the first available profile.
func (a *App) selectDefaultProfile() {
	acct := a.Auth.GetAccount()
	if acct == nil {
		return
	}

	// If current profile is valid, try to re-validate it.
	if acct.SelectedProfile != nil {
		if err := acct.SetCurrentProfile(*acct.SelectedProfile); err != nil {
			sentry.CaptureException(err)
			// Clear invalid profile selection.
			acct.SetCurrentProfile("")
		}
	}

	// If no profile is selected and we have profiles, select the first one.
	if acct.SelectedProfile == nil && len(acct.Profiles) > 0 {
		firstUUID := acct.Profiles[0].UUID
		slog.Debug("selecting default profile", "profile", firstUUID)
		a.SetUserProfile(firstUUID)
	}
}

// GetUserChannels returns the list of channels available to the current user.
// In offline mode, only channels that are offline-ready are returned.
func (a *App) GetUserChannels() []string {
	if net.Current() == net.ModeOffline {
		return a.getOfflineChannels()
	}
	return a.getEntitledChannels()
}

// RefreshNewsFeed fetches the latest news articles.
// It emits a hint event to the frontend when new articles are available.
func (a *App) RefreshNewsFeed() error {
	hasNew, err := news.GetFeedArticles(true)
	if err != nil {
		return err
	}

	if hasNew {
		a.Emit("hint:news_available")
	}

	return nil
}

// GetAccount returns the current user's account for frontend access.
func (a *App) GetAccount() *account.Account {
	return a.Auth.GetAccount()
}

// IsLoggedIn returns true if a user is currently logged in.
func (a *App) IsLoggedIn() bool {
	return a.Auth.IsLoggedIn()
}

// Logout logs out the current user and clears their session.
func (a *App) Logout() error {
	// Clear the update environment.
	a.SetChannel(nil)

	// Stop the refresh loop.
	if a.refresher != nil {
		a.refresher.Stop()
		a.refresher = nil
	}

	// Logout from the auth controller.
	if err := a.Auth.Logout(); err != nil {
		return err
	}

	// Notify the frontend.
	a.Emit("logout")
	a.ReloadLauncher("logout")

	return nil
}

// GetState returns the current app state for the frontend.
func (a *App) GetState() *appstate.State {
	return a.State
}

// GetCurrentChannel returns the currently selected channel name.
func (a *App) GetCurrentChannel() *string {
	return a.getCurrentChannel()
}

// Login initiates the OAuth login flow.
// It starts a local loopback HTTP server for the callback and returns the authorization URL.
func (a *App) Login() (string, error) {
	// Stop any existing login attempt
	if currentLoopback != nil {
		currentLoopback.Stop()
	}

	// Create new loopback handler
	currentLoopback = oauth.NewLoopback()

	// Start the loopback server and get the authorization URL
	authURL, err := currentLoopback.Start()
	if err != nil {
		currentLoopback = nil
		return "", err
	}

	// Wait for the login to complete in background
	go a.waitForLogin()

	return authURL, nil
}

// waitForLogin waits for the OAuth flow to complete and processes the result.
func (a *App) waitForLogin() {
	loopback := currentLoopback
	if loopback == nil {
		return
	}

	defer func() {
		loopback.Stop()
		currentLoopback = nil
	}()

	// Wait for token with 5 minute timeout
	token, err := loopback.Wait(5 * time.Minute)
	if err != nil {
		slog.Error("login failed", "error", err)
		a.Emit("login_error", err.Error())
		return
	}

	// Get the OAuth config for token refresh
	config := loopback.GetConfig()

	// Create the account from the token
	if err := a.createAccountFromToken(token, config); err != nil {
		slog.Error("failed to create account", "error", err)
		a.Emit("login_error", err.Error())
		return
	}

	slog.Info("login successful")
	a.Emit("login_success")
	a.ReloadLauncher("login_success")
}

// createAccountFromToken creates a new account from an OAuth token.
func (a *App) createAccountFromToken(token *oauth2.Token, config *oauth2.Config) error {
	// Set the OAuth config for token refresh
	auth.SetOAuthConfig(config)

	// Create HTTP client with token
	client := config.Client(context.Background(), token)

	// Fetch user data from the API
	// TODO: Implement launcher data fetching
	// For now, create a minimal account

	acct := &account.Account{
		Token: account.Token{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			Expiry:       token.Expiry,
		},
	}

	// Set the account in the auth controller
	a.Auth.SetAccount(acct, client)

	return nil
}

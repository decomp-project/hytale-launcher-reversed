// Package app provides the main application logic for the Hytale launcher.
// It handles application lifecycle, user authentication, update channels,
// and communication with the frontend via Wails.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"hytale-launcher/internal/account"
	"hytale-launcher/internal/appstate"
	"hytale-launcher/internal/auth"
	"hytale-launcher/internal/hytale"
	"hytale-launcher/internal/ioutil"
	"hytale-launcher/internal/net"
	"hytale-launcher/internal/throttle"
	"hytale-launcher/internal/update"
	"hytale-launcher/internal/updater"
)

// App is the main application struct that manages the launcher's state and behavior.
// It coordinates between the authentication controller, update system, and frontend.
type App struct {
	// ctx is the Wails application context, used for emitting events to the frontend.
	ctx context.Context

	// Auth is the authentication controller managing user sessions and OAuth tokens.
	Auth *auth.Controller

	// ready is a channel that signals when the backend initialization is complete.
	ready chan struct{}

	// listen is the update event listener that forwards events to the frontend.
	listen *appListen

	// Updater handles checking for and applying game updates.
	Updater *updater.Updater

	// refresher periodically refreshes application state.
	refresher *throttle.Refresher

	// refreshMu protects the refresh operation from concurrent access.
	refreshMu sync.Mutex

	// State is the current update channel's state, including dependencies.
	State *appstate.State

	// selectedChannel holds the name of the currently selected update channel.
	selectedChannel *string
}

// New creates a new App instance.
func New() *App {
	return &App{
		ready: make(chan struct{}),
	}
}

// init initializes the application backend.
// It creates the storage directory, initializes the auth controller,
// and sets up the user session if one exists.
func (a *App) init() error {
	// Ensure the storage directory exists.
	if err := ioutil.MkdirAll(hytale.StorageDir()); err != nil {
		return fmt.Errorf("unable to create storage directory: %w", err)
	}

	// Initialize the authentication controller.
	a.Auth = new(auth.Controller)
	if err := a.Auth.Init(); err != nil {
		return fmt.Errorf("unable to initialize auth controller: %w", err)
	}

	// If user is already logged in, initialize their session.
	if profile := a.getCurrentProfile(); profile != nil {
		a.userInit()
	}

	// Clean up the download cache directory.
	cacheDir := hytale.InStorageDir("cache")
	if err := os.RemoveAll(cacheDir); err != nil {
		slog.Warn("unable to flush download cache", "error", err)
	}

	slog.Info("app initialized")

	// Signal that initialization is complete.
	a.ready <- struct{}{}
	close(a.ready)

	return nil
}

// DomReady is called by Wails when the frontend DOM is ready.
// It starts a goroutine that waits for backend initialization
// and then notifies the frontend.
func (a *App) DomReady(ctx context.Context) {
	go func() {
		slog.Debug("frontend ready, waiting for backend")
		<-a.ready
		slog.Debug("backend ready, notifying frontend")
		a.ReloadLauncher("dom_ready")
	}()
}

// Startup is called by Wails when the application starts.
// It stores the context and initializes the application backend.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	if err := a.init(); err != nil {
		sentry.CaptureException(err)
		slog.Error("error during app initialization", "error", err)
		panic(err)
	}
}

// Emit sends an event to the frontend with the given name and arguments.
// Events named "update:status" are not logged to avoid log spam.
func (a *App) Emit(name string, args ...any) {
	if name != "update:status" {
		slog.Debug("emitting event", "name", name, "args", args)
	}

	runtime.EventsEmit(a.ctx, name, args...)
}

// ReloadLauncher emits a "reload" event to the frontend, causing it to refresh its state.
// The cause parameter is logged for debugging purposes.
func (a *App) ReloadLauncher(cause string) {
	slog.Debug("reloading launcher", "cause", cause)
	a.Emit("reload")
}

// userInit initializes user-specific state after login.
// It selects the default profile, restores the previously selected channel if valid,
// and starts the periodic refresh loop.
func (a *App) userInit() {
	a.selectDefaultProfile()

	// Check if the previously selected channel is still available.
	acct := a.Auth.GetAccount()
	if acct != nil && acct.SelectedChannel != nil {
		channels := a.GetUserChannels()
		if slices.Contains(channels, *acct.SelectedChannel) {
			slog.Info("restoring previously selected channel", "channel", *acct.SelectedChannel)
			a.SetChannel(acct.SelectedChannel)
		}
	}

	// Start the periodic refresh loop (every hour).
	a.refresher = throttle.NewRefresher(a.refresh)
	a.refresher.Start(time.Hour)
}

// refresh performs a soft refresh of the application state.
// It checks for updates and refreshes the news feed.
func (a *App) refresh() error {
	slog.Debug("soft refreshing application state")

	// Check for updates without forcing a network request.
	count := a.CheckForUpdates(false)
	if count > 0 {
		a.Emit("hint:updates_available")
	}

	// Refresh the news feed.
	if err := a.RefreshNewsFeed(); err != nil {
		return fmt.Errorf("unable to refresh news feed: %w", err)
	}

	if count > 0 {
		a.Emit("hint:news_available")
	}

	return nil
}

const refreshCooldown = 15 * time.Minute

// refreshUser refreshes the current user's account data.
// If force is false, it will only refresh if the last refresh was more than 15 minutes ago.
func (a *App) refreshUser(force bool, cause string) {
	slog.Debug("requested user account refresh", "force", force, "cause", cause)

	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()

	acct := a.Auth.GetAccount()
	if acct == nil {
		return
	}

	// Check refresh cooldown unless forced.
	if !force && time.Since(acct.LastRefresh) < refreshCooldown {
		return
	}

	// Refresh the account from the server.
	if err := acct.Refresh(a.Auth.Client(), cause); err == nil {
		a.selectDefaultProfile()
		a.Auth.SaveAccount("refresh_user")
	}
}

// setNetMode updates the network mode and ensures the current channel is still valid.
func (a *App) setNetMode(mode net.Mode, schedule *update.Schedule) {
	oldMode := net.Current()
	net.SetMode(mode)

	if oldMode != mode {
		slog.Info("setting network mode", "mode", mode)
		a.ensureValidChannel(a.getCurrentChannel())
		a.Emit("setNetworkMode", mode)

		// If a schedule was provided, notify the update listener.
		if schedule != nil && a.listen != nil {
			a.listen.Notify(update.Notification{
				// Schedule-related notification fields would go here.
			})
		}
	}
}

// getCurrentProfile returns the current user's profile, or nil if not logged in.
func (a *App) getCurrentProfile() *account.Profile {
	if a.Auth == nil {
		return nil
	}

	acct := a.Auth.GetAccount()
	if acct == nil {
		return nil
	}

	return acct.GetCurrentProfile()
}

package app

import (
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/getsentry/sentry-go"

	"hytale-launcher/internal/appstate"
	"hytale-launcher/internal/update"
	"hytale-launcher/internal/updater"
)

// channelsEqual checks if two channel pointers reference equivalent values.
func channelsEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// ReleaseChannels contains the list of known release channels in priority order.
// This is used to select a fallback channel when the user's channel is not available.
var ReleaseChannels = []string{"release"}

// loadEnv loads the state for a given channel from disk.
// If the state file doesn't exist, it creates a new state.
func (a *App) loadEnv(channel string) *appstate.State {
	state, err := appstate.Load(channel)

	// Handle errors (except "file not found" which is expected for new channels).
	if err != nil && !errors.Is(err, appstate.ErrNotFound) {
		sentry.CaptureException(err)
		slog.Error("failed to load channel", "channel", channel, "error", err)
	}

	// If state doesn't exist, create a new one.
	if state == nil {
		state = appstate.New(channel)
	}

	return state
}

// SetChannel changes the current update channel.
// This updates the stored state, creates a new updater for the channel,
// and persists the selection to the user's account.
func (a *App) SetChannel(channel *string) {
	currentChannel := a.getCurrentChannel()

	// Log the channel change.
	newChannelStr := formatChannel(channel)
	currentChannelStr := formatChannel(currentChannel)
	slog.Info("setting channel", "channel", newChannelStr, "current", currentChannelStr)

	// Handle nil channel (clearing the selection).
	if channel == nil {
		a.Updater = nil
		a.State = nil
		goto updateAccount
	}

	// Load the state for the new channel.
	a.State = a.loadEnv(*channel)

	// Create a new updater with JRE and game packages.
	a.Updater = updater.New(
		a.listen,
		updater.Package{Name: "jre", Pkg: &update.JREPackage{}},
		updater.Package{Name: "game", Pkg: &update.GamePackage{}},
	)

updateAccount:
	// Save the channel selection to the user's account if it changed.
	if !channelsEqual(currentChannel, channel) {
		if acct := a.Auth.GetAccount(); acct != nil {
			acct.SelectedChannel = channel
			a.Auth.SaveAccount("channel_set")
		}
	}
}

// preferredChannels returns the list of channels in preference order.
// This is used when the user's channel is not available to select a fallback.
var preferredChannels = ReleaseChannels

// ensureValidChannel checks if the current channel is still valid for the user.
// If not, it selects the first available preferred channel.
func (a *App) ensureValidChannel(currentChannel *string) {
	userChannels := a.GetUserChannels()

	currentChannelStr := formatChannel(currentChannel)
	slog.Debug("validating current channel access", "current", currentChannelStr, "options", userChannels)

	// Check if current channel is still in the user's available channels.
	channelValid := currentChannel == nil
	if currentChannel != nil {
		channelValid = slices.Contains(userChannels, *currentChannel)
	}

	// If current channel is no longer valid, find a fallback.
	if !channelValid {
		for _, preferred := range preferredChannels {
			if slices.Contains(userChannels, preferred) {
				a.SetChannel(&preferred)
				return
			}
		}

		// No preferred channel available - clear the selection.
		a.SetChannel(nil)
	}
}

// getCurrentChannel returns a copy of the current channel name.
func (a *App) getCurrentChannel() *string {
	if a.State == nil {
		return nil
	}
	channel := a.State.Channel
	return &channel
}

// getEntitledChannels returns the list of channels the current user is entitled to.
// It filters the user's entitlements to only include patchline-based channels.
func (a *App) getEntitledChannels() []string {
	profile := a.getCurrentProfile()
	if profile == nil {
		return ReleaseChannels
	}

	var channels []string
	for _, entitlement := range profile.Entitlements {
		// Only include patchline entitlements.
		if strings.HasPrefix(entitlement, "patchline:") {
			channel := strings.TrimPrefix(entitlement, "patchline:")
			channels = append(channels, channel)
		}
	}

	return channels
}

// getOfflineChannels returns the list of channels available in offline mode.
// A channel is available offline if its state indicates it's ready for offline use.
func (a *App) getOfflineChannels() []string {
	entitled := a.getEntitledChannels()
	var available []string

	for _, channel := range entitled {
		state, err := appstate.Load(channel)

		// Skip channels that can't be loaded (unless it's just not found).
		if err != nil && !errors.Is(err, appstate.ErrNotFound) {
			sentry.CaptureException(err)
			slog.Error("failed to load channel for offline status", "channel", channel, "error", err)
			continue
		}

		// Include channels that are offline-ready.
		if state != nil && state.OfflineReady {
			available = append(available, channel)
		}
	}

	return available
}

// formatChannel returns a string representation of a channel pointer.
// Returns "<nil>" if the pointer is nil.
func formatChannel(channel *string) string {
	if channel == nil {
		return "<nil>"
	}
	return *channel
}

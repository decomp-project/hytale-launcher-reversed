// Package account provides account management functionality for the Hytale launcher.
// It handles user profiles, authentication tokens, and account persistence.
package account

import (
	"fmt"
	"log"
	"time"
)

// keyName is the keyring key name used for encrypting account data.
const keyName = "3CA80030-8679-41AD-9E5C-09705C233580"

// Token represents OAuth authentication tokens for a user.
type Token struct {
	// AccessToken is the OAuth access token string.
	AccessToken string `json:"access_token"`
	// RefreshToken is the OAuth refresh token string.
	RefreshToken string `json:"refresh_token"`
	// Expiry is the expiration time of the access token.
	Expiry time.Time `json:"expiry"`
}

// Profile represents a user profile in the Hytale launcher.
type Profile struct {
	// Name is the display name of the profile.
	Name string `json:"name"`
	// UUID is the unique identifier for this profile.
	UUID string `json:"uuid"`
	// Entitlements is a list of granted entitlements (e.g., "patchline:release").
	Entitlements []string `json:"entitlements,omitempty"`
	// Token is the OAuth token for this profile.
	Token Token `json:"token,omitempty"`
}

// Patchline represents a game patchline/channel configuration.
type Patchline struct {
	// Name is the display name of the patchline.
	Name string `json:"name"`
	// Version is the current version of this patchline.
	Version int `json:"version"`
}

// Account represents a user's account data including profiles and settings.
type Account struct {
	// Profiles is the list of user profiles associated with this account.
	Profiles []Profile `json:"profiles"`
	// Patchlines maps patchline names to their configurations.
	Patchlines map[string]Patchline `json:"patchlines"`
	// EULAAcceptedAt records when the EULA was accepted, if at all.
	EULAAcceptedAt *time.Time `json:"eula_accepted_at,omitempty"`
	// Token holds the OAuth tokens for this account.
	Token Token `json:"token"`

	// SelectedProfile is the UUID of the currently selected profile.
	SelectedProfile *string `json:"selected_profile,omitempty"`
	// SelectedChannel is the currently selected patchline/channel name.
	SelectedChannel *string `json:"selected_channel,omitempty"`

	// CurrentProfile points to the currently selected profile in the Profiles slice.
	// This is not serialized to JSON.
	CurrentProfile *Profile `json:"-"`

	// LastRefresh is the last time account data was refreshed from the server.
	LastRefresh time.Time `json:"-"`

	// filePath is the path where the account file is stored.
	filePath string
}

// newAccount creates a new Account with the given file path.
func newAccount(filePath string) *Account {
	return &Account{
		filePath: filePath,
	}
}

// SetCurrentProfile sets the current profile by UUID.
// If uuid is empty, the current profile is cleared.
// Returns an error if no profile with the given UUID is found.
func (a *Account) SetCurrentProfile(uuid string) error {
	log.Printf("setting current profile to %s", uuid)

	if uuid == "" {
		a.CurrentProfile = nil
		a.SelectedProfile = nil
		return nil
	}

	for i := range a.Profiles {
		if a.Profiles[i].UUID == uuid {
			a.CurrentProfile = &a.Profiles[i]
			a.SelectedProfile = &uuid
			return nil
		}
	}

	return fmt.Errorf("no profile with UUID %s found", uuid)
}

// GetCurrentProfile returns the currently selected profile.
// Returns nil if no profile is selected or if the current profile
// is no longer in the profiles list.
func (a *Account) GetCurrentProfile() *Profile {
	if a.CurrentProfile == nil {
		return nil
	}

	// Verify the current profile still exists in the profiles list
	for i := range a.Profiles {
		if a.Profiles[i].UUID == a.CurrentProfile.UUID {
			return &a.Profiles[i]
		}
	}

	return nil
}


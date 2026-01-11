package account

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"hytale-launcher/internal/build"
	"hytale-launcher/internal/endpoints"
	"hytale-launcher/internal/ioutil"
	"hytale-launcher/internal/net"
)

// launcherData represents the response from the launcher data API.
// This is an internal type used to deserialize the API response.
type launcherData struct {
	// Owner is the owner identifier for the account.
	Owner string `json:"owner"`
	// Profiles contains the list of user profiles.
	Profiles []Profile `json:"profiles"`
	// Patchlines maps channel names to their configurations.
	Patchlines map[string]Patchline `json:"patchlines"`
	// EULAAcceptedAt records when the EULA was accepted, if at all.
	EULAAcceptedAt *time.Time `json:"eula_accepted_at,omitempty"`
}

// Refresh fetches the latest account data from the server.
// It updates the account's Profiles, Patchlines, EULAAcceptedAt, and RefreshedAt fields.
// The client should be an authenticated HTTP client.
// The cause parameter is used for logging purposes.
//
// Returns an error if the network request fails or if the launcher is offline.
func (a *Account) Refresh(client *http.Client, cause string) error {
	slog.Debug("refreshing account data", "cause", cause)

	// Check if we're offline
	if err := net.OfflineError(); err != nil {
		return err
	}

	// Build query parameters
	params := url.Values{}
	params.Set("os", build.OS())
	params.Set("arch", build.Arch())

	// Fetch launcher data from the API
	data, err := ioutil.Get[launcherData](client, endpoints.LauncherData(), params)
	if err != nil {
		return fmt.Errorf("error fetching account launcher data: %w", err)
	}

	// Only update if we received profiles
	if len(data.Profiles) == 0 {
		return nil
	}

	// Update account fields with new data
	a.Profiles = data.Profiles
	a.Patchlines = data.Patchlines
	a.EULAAcceptedAt = data.EULAAcceptedAt
	a.LastRefresh = time.Now()

	return nil
}

package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"hytale-launcher/internal/endpoints"
)

// OAuth configuration constants
const (
	ClientID = "hytale-launcher"
	Scopes   = "openid offline auth:launcher"
)

// callbackData holds data received from an OAuth callback.
// Based on decompiled structure analysis:
// - Offset 0x00: success (bool)
// - Offset 0x10: port (int64)
type callbackData struct {
	Success bool
	Port    int64
}

// stateData contains OAuth state information for CSRF protection.
// Based on decompiled structure analysis:
// - Offset 0x08: state string length
// - Offset 0x10: state string data pointer
// - Offset 0x18: verifier string length
// - Offset 0x20: verifier string data pointer
type stateData struct {
	State    string
	Verifier string
}

// result represents the outcome of an OAuth flow.
// Based on decompiled structure analysis:
// - Offset 0x00: token pointer
// - Offset 0x08: error interface type
// - Offset 0x10: error interface data
type result struct {
	Token *oauth2.Token
	Err   error
}

// Loopback handles OAuth authentication via a local HTTP server.
// Based on decompiled structure analysis:
// - Offset 0x08: ClientID string
// - Offset 0x18: RedirectURL string
// - Offset 0x20: Port int64
// - Offset 0x28: Config interface type (oauth2.Config)
// - Offset 0x30: Config interface data
type Loopback struct {
	ClientID    string
	RedirectURL string
	Port        int
	Config      *oauth2.Config

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	state    *stateData
	resultCh chan result
}

// NewLoopback creates a new Loopback handler with default configuration.
func NewLoopback() *Loopback {
	return &Loopback{
		ClientID: ClientID,
		resultCh: make(chan result, 1),
	}
}

// generateRandomString generates a cryptographically secure random string.
func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateCodeChallenge creates a S256 code challenge from a code verifier.
func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// Start initializes the loopback server and returns the authorization URL.
// The server listens on a random available port on localhost.
func (l *Loopback) Start() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Clean up any existing server
	if l.server != nil {
		l.server.Close()
	}

	// Generate PKCE parameters
	state, err := generateRandomString(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	codeVerifier, err := generateRandomString(64)
	if err != nil {
		return "", fmt.Errorf("failed to generate code verifier: %w", err)
	}

	l.state = &stateData{
		State:    state,
		Verifier: codeVerifier,
	}

	codeChallenge := generateCodeChallenge(codeVerifier)

	// Start loopback server on a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to start loopback server: %w", err)
	}

	l.listener = listener
	l.Port = listener.Addr().(*net.TCPAddr).Port
	l.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", l.Port)

	slog.Info("loopback server starting", "port", l.Port)

	// Create HTTP server for callback
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", l.handleCallback)

	l.server = &http.Server{Handler: mux}

	// Start server in background
	go func() {
		if err := l.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("loopback server error", "error", err)
		}
	}()

	// Build OAuth2 config
	l.Config = &oauth2.Config{
		ClientID: l.ClientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  endpoints.OAuthAuth(),
			TokenURL: endpoints.OAuthToken(),
		},
		RedirectURL: l.RedirectURL,
		Scopes:      []string{Scopes},
	}

	// Build authorization URL with PKCE
	params := url.Values{
		"client_id":             {l.ClientID},
		"redirect_uri":          {l.RedirectURL},
		"response_type":         {"code"},
		"scope":                 {Scopes},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	authURL := endpoints.OAuthAuth() + "?" + params.Encode()

	slog.Debug("generated OAuth URL", "url", authURL)

	return authURL, nil
}

// handleCallback processes the OAuth callback from the authorization server.
func (l *Loopback) handleCallback(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	state := l.state
	l.mu.Unlock()

	if state == nil {
		http.Error(w, "No login in progress", http.StatusBadRequest)
		return
	}

	// Verify state parameter
	if r.URL.Query().Get("state") != state.State {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		l.resultCh <- result{Err: errors.New("invalid state parameter")}
		return
	}

	// Check for error response
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		http.Error(w, fmt.Sprintf("Authorization error: %s", errDesc), http.StatusBadRequest)
		l.resultCh <- result{Err: fmt.Errorf("authorization error: %s - %s", errParam, errDesc)}
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No authorization code received", http.StatusBadRequest)
		l.resultCh <- result{Err: errors.New("no authorization code received")}
		return
	}

	// Send success response to browser
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Login Successful</title></head>
<body style="background:#1b2636;color:#d2d9e2;font-family:sans-serif;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;">
<div style="text-align:center;">
<h1>Login Successful</h1>
<p>You can close this window and return to the Hytale Launcher.</p>
</div>
</body>
</html>`))

	// Exchange code for tokens
	go l.exchangeCode(code)
}

// exchangeCode exchanges the authorization code for tokens.
func (l *Loopback) exchangeCode(code string) {
	l.mu.Lock()
	state := l.state
	config := l.Config
	l.mu.Unlock()

	if state == nil || config == nil {
		l.resultCh <- result{Err: errors.New("no login state available")}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Exchange code for token using PKCE
	token, err := config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", state.Verifier),
	)

	if err != nil {
		slog.Error("failed to exchange code for tokens", "error", err)
		l.resultCh <- result{Err: fmt.Errorf("token exchange failed: %w", err)}
		return
	}

	slog.Info("login successful, received tokens")
	l.resultCh <- result{Token: token}
}

// Wait blocks until the OAuth flow completes and returns the token.
// Returns an error if the flow fails or times out.
func (l *Loopback) Wait(timeout time.Duration) (*oauth2.Token, error) {
	select {
	case res := <-l.resultCh:
		return res.Token, res.Err
	case <-time.After(timeout):
		return nil, errors.New("login timeout")
	}
}

// Stop shuts down the loopback server.
func (l *Loopback) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.server != nil {
		l.server.Close()
		l.server = nil
	}
	if l.listener != nil {
		l.listener.Close()
		l.listener = nil
	}
	l.state = nil
}

// GetConfig returns the OAuth2 config used for this login.
// Returns nil if Start() hasn't been called.
func (l *Loopback) GetConfig() *oauth2.Config {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Config
}

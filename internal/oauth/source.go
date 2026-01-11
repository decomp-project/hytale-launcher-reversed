// Package oauth provides OAuth2 token source wrappers with change notification support.
package oauth

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// TokenObserver is a callback function that is invoked when a token changes.
type TokenObserver func(*oauth2.Token)

// watchTokenSource wraps an oauth2.TokenSource and notifies an observer when
// the token changes. It is safe for concurrent use.
type watchTokenSource struct {
	mux      sync.Mutex
	src      oauth2.TokenSource
	observer TokenObserver
	prev     *oauth2.Token
}

// tokenEqual compares two tokens for equality based on their AccessToken,
// RefreshToken, and Expiry fields.
func tokenEqual(a, b *oauth2.Token) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.AccessToken != b.AccessToken {
		return false
	}
	if a.RefreshToken != b.RefreshToken {
		return false
	}
	if !a.Expiry.Equal(b.Expiry) {
		return false
	}
	return true
}

// Token retrieves a token from the underlying source. If the token has changed
// from the previous retrieval, the observer callback is invoked with the new token.
// This method is safe for concurrent use.
func (w *watchTokenSource) Token() (*oauth2.Token, error) {
	w.mux.Lock()
	defer w.mux.Unlock()

	tok, err := w.src.Token()
	if err != nil {
		return nil, err
	}

	if !tokenEqual(tok, w.prev) {
		w.prev = tok
		w.observer(tok)
	}

	return tok, nil
}

// NewWatchClient creates an HTTP client that uses the provided OAuth2 configuration
// and token. The observer callback is invoked whenever the token is refreshed.
// The returned client has a default timeout of 10 seconds.
func NewWatchClient(ctx context.Context, config *oauth2.Config, token *oauth2.Token, observer TokenObserver) *http.Client {
	// Create a token source that can refresh the token
	src := config.TokenSource(ctx, token)

	// Wrap with a watchTokenSource that notifies on changes
	watchSrc := &watchTokenSource{
		src:      src,
		observer: observer,
		prev:     token,
	}

	// Create the OAuth2 client
	client := oauth2.NewClient(ctx, watchSrc)
	client.Timeout = 10 * time.Second

	return client
}

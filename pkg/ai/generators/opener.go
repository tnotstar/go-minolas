// Package generators provides AI text generation with a pluggable provider
// architecture. It allows creating generator clients using URL schemes,
// enabling a flexible and extensible approach to AI provider connectivity.
//
// The package supports registering custom provider drivers through the
// Opener interface, following the same pluggable pattern as the sqlt package.
package generators

import (
	"context"
	"errors"
	"net/url"
	"sync"
)

var (
	// ErrUnsupportedOpener is returned when no registered opener can handle
	// the provided generator URL scheme.
	ErrUnsupportedOpener = errors.New("unsupported provider or incorrect url scheme")
)

// Opener defines the interface for AI generator provider openers.
// Each opener is responsible for handling specific URL schemes and creating
// appropriate generator clients.
type Opener interface {

	// Id returns a unique identifier for this opener.
	Id() string

	// Open creates and returns a Generator client for the given URL.
	// Returns an error if the client cannot be created.
	Open(ctx context.Context, u *url.URL) (Generator, error)

	// CanOpen reports whether this opener can handle the given URL scheme.
	CanOpen(u *url.URL) bool
}

var (
	openers   = make(map[string]Opener)
	openersMu sync.RWMutex
)

// Open creates a Generator client using the provided URL string.
// It parses the URL and delegates to the first registered opener that
// can handle the URL's scheme.
//
// Returns ErrUnsupportedOpener if no registered opener can handle the URL.
// Returns an error if the URL cannot be parsed or if the opener fails to
// create a generator client.
//
// Example:
//
//	gen, err := generators.Open("gemini://my-api-key@gemini-2.0-flash")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer gen.Close()
func Open(ctx context.Context, aiurl string) (Generator, error) {
	openersMu.Lock()
	defer openersMu.Unlock()

	u, err := url.Parse(aiurl)
	if err != nil {
		return nil, err
	}
	for _, op := range openers {
		if op.CanOpen(u) {
			return op.Open(ctx, u)
		}
	}
	return nil, ErrUnsupportedOpener
}

// RegisterOpener registers a generator opener for use with Open.
// It panics if the opener is nil or if an opener with the same Id
// has already been registered.
//
// RegisterOpener is typically called in the init function of packages
// that implement generator providers.
func RegisterOpener(opener Opener) {
	openersMu.Lock()
	defer openersMu.Unlock()

	if opener == nil {
		panic("generators: Register opener is nil")
	}
	id := opener.Id()
	if _, dup := openers[id]; dup {
		panic("generators: Register called twice for provider: " + id)
	}
	openers[id] = opener
}

// ResetOpeners removes all registered openers.
// This function is primarily useful for testing.
func ResetOpeners() {
	openersMu.Lock()
	defer openersMu.Unlock()

	for op := range openers {
		delete(openers, op)
	}
}

// ListOpeners returns a list of IDs for all registered openers.
func ListOpeners() []string {
	openersMu.RLock()
	defer openersMu.RUnlock()

	ids := make([]string, 0, len(openers))
	for id := range openers {
		ids = append(ids, id)
	}
	return ids
}

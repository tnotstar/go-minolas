// Package sqlt provides database connection utilities with a pluggable
// driver architecture. It allows opening database connections using URL
// schemes similar to standard database connection strings.
//
// The package supports registering custom database drivers through the
// Opener interface, enabling a flexible and extensible approach to
// database connectivity.
package sqlt

import (
	"database/sql"
	"errors"
	"net/url"
	"sync"
)

var (
	// ErrUnsupportedOpener is returned when no registered opener can handle
	// the provided database URL scheme.
	ErrUnsupportedOpener = errors.New("unsupported driver or incorrect url scheme")
)

// Opener defines the interface for database connection openers.
// Each opener is responsible for handling specific database URL schemes
// and creating appropriate database connections.
type Opener interface {
	// Id returns a unique identifier for this opener.
	Id() string

	// Open creates and returns a database connection for the given URL.
	// Returns an error if the connection cannot be established.
	Open(*url.URL) (*sql.DB, error)

	// CanOpen reports whether this opener can handle the given URL scheme.
	CanOpen(*url.URL) bool
}

var (
	openers   = make(map[string]Opener)
	openersMu sync.RWMutex
)

// Open opens a database connection using the provided URL string.
// It parses the URL and delegates to the first registered opener that
// can handle the URL's scheme.
//
// Returns ErrUnsupportedOpener if no registered opener can handle the URL.
// Returns an error if the URL cannot be parsed or if the opener fails to
// establish a connection.
//
// Example:
//
//	db, err := sqlt.Open("sqlite::memory:")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
func Open(dburl string) (*sql.DB, error) {
	openersMu.Lock()
	defer openersMu.Unlock()

	u, err := url.Parse(dburl)
	if err != nil {
		return nil, err
	}
	for _, op := range openers {
		if op.CanOpen(u) {
			return op.Open(u)
		}
	}
	return nil, ErrUnsupportedOpener
}

// RegisterOpener registers a database opener for use with Open.
// It panics if the opener is nil or if an opener with the same Id
// has already been registered.
//
// RegisterOpener is typically called in the init function of packages
// that implement database drivers.
func RegisterOpener(opener Opener) {
	openersMu.Lock()
	defer openersMu.Unlock()

	if opener == nil {
		panic("sqlt: Register opener is nil")
	}
	id := opener.Id()
	if _, dup := openers[id]; dup {
		panic("sqlt: Register called twice for driver: " + id)
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

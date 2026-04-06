package sqlt

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// SqliteOpener implements the Opener interface for SQLite databases.
// It supports both "sqlite" and "sqlite3" URL schemes.
//
// URL format: sqlite://[path][?query]
//
// Examples:
//   - sqlite::memory: - in-memory database
//   - sqlite:file.db - file-based database
//   - sqlite:file.db?mode=ro - read-only database
type SqliteOpener struct {
}

// Id returns the unique identifier for the SQLite opener.
func (o *SqliteOpener) Id() string {
	return "sqlite"
}

// Open opens a SQLite database connection using the provided URL.
// Returns an error if the URL is nil, has an unsupported scheme,
// or if the database connection cannot be established.
func (o *SqliteOpener) Open(u *url.URL) (*sql.DB, error) {
	if u == nil {
		return nil, errors.New("sqlt: database URL cannot be nil for SqliteOpener")
	}
	if !o.CanOpen(u) {
		return nil, fmt.Errorf("sqlt: scheme %q not supported by SqliteOpener (expected sqlite or sqlite3)", u.Scheme)
	}

	// Build DSN from URL components
	dsn := u.Host + u.Path
	if u.RawQuery != "" {
		dsn += "?" + u.RawQuery
	}
	return sql.Open("sqlite", dsn)
}

// CanOpen reports whether this opener can handle the given URL.
// It returns true for "sqlite" and "sqlite3" schemes.
func (o *SqliteOpener) CanOpen(u *url.URL) bool {
	return u.Scheme == "sqlite" || u.Scheme == "sqlite3"
}

func init() {
	RegisterOpener(&SqliteOpener{})
}

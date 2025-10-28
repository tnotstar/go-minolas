package sqlt

import (
	"database/sql"
	"errors"
	"net/url"

	_ "modernc.org/sqlite"
)

type SqliteOpener struct {
}

func (o *SqliteOpener) Id() string {
	return "sqlite"
}

func (o *SqliteOpener) Open(u *url.URL) (*sql.DB, error) {
	if u == nil {
		return nil, errors.New("sqlt: database url cannot be nil for SqliteOpener")
	}
	if !o.CanOpen(u) {
		return nil, errors.New("sqlt: invalid database `url` for SqliteOpener")
	}
	var dsn string

	if u.Opaque != "" {
		// For "sqlite3:file.db?mode=memory", u.Opaque is "file.db" and u.RawQuery
		// is "mode=memory".
		// The DSN for sqlite is everything after the scheme and '://'.
		dsn = u.Opaque
	} else if u.Path != "" {
		// For "sqlite3:///path/to/db.sqlite", u.Path is "/path/to/db.sqlite".
		// For path-based URLs like sqlite3:///path/to/file.db
		// u.Path will be /path/to/file.db.
		// On Windows, sqlite3:///C:/path/to/file.db gives u.Path /C:/path/to/file.db
		dsn = u.Path
	}
	if u.RawQuery != "" {
		dsn = dsn + "?" + u.RawQuery
	}

	return sql.Open(o.Id(), dsn)
}

func (o *SqliteOpener) CanOpen(u *url.URL) bool {
	return u.Scheme == "sqlite" || u.Scheme == "sqlite3"
}

func init() {
	registerOpener(&SqliteOpener{})
}

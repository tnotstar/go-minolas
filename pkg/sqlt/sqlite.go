package sqlt

import (
	"database/sql"
	"errors"
	"net/url"
)

type SqliteOpener struct {
}

func (o *SqliteOpener) Id() string {
	return "sqlite3"
}

func (o *SqliteOpener) Open(u *url.URL) (*sql.DB, error) {
	if u == nil {
		return nil, errors.New("database url cannot be nil")
	}
	return sql.Open("sqlite3", u.EscapedPath())
}

func (o *SqliteOpener) CanOpen(u *url.URL) bool {
	return u.Scheme == "sqlite" || u.Scheme == "sqlite3"
}

func init() {
	registerOpener(&SqliteOpener{})
}

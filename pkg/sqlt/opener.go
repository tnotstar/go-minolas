package sqlt

import (
	"database/sql"
	"errors"
	"net/url"
	"sync"
)

var (
	ErrUnsupportedOpener = errors.New("unsupported driver or incorrect url scheme")
)

type Opener interface {
	Id() string
	Open(*url.URL) (*sql.DB, error)
	CanOpen(*url.URL) bool
}

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

var openers = make(map[string]Opener)
var openersMu sync.RWMutex

func registerOpener(opener Opener) {
	openersMu.Lock()
	defer openersMu.Unlock()

	if opener == nil {
		panic("sqlt: Register opener is nil")
	}
	id := opener.Id()
	if _, dup := openers[id]; dup {
		panic("sql: Register called twice for driver: " + id)
	}
	openers[id] = opener
}

func resetOpeners() {
	openersMu.Lock()
	defer openersMu.Unlock()

	for op := range openers {
		delete(openers, op)
	}
}

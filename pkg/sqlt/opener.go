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

func OpenDB(dburl string) (*sql.DB, error) {
	theOpenersLock.Lock()
	defer theOpenersLock.Unlock()

	u, err := url.Parse(dburl)
	if err != nil {
		return nil, err
	}
	for _, op := range theOpeners {
		if op.CanOpen(u) {
			return op.Open(u)
		}
	}
	return nil, ErrUnsupportedOpener
}

var theOpeners = make(map[string]Opener)
var theOpenersLock sync.RWMutex

func registerOpener(opener Opener) {
	theOpenersLock.Lock()
	defer theOpenersLock.Unlock()

	if opener == nil {
		panic("sqlt: Register opener is nil")
	}
	id := opener.Id()
	if _, dup := theOpeners[id]; dup {
		panic("sql: Register called twice for driver: " + id)
	}
	theOpeners[id] = opener
}

func resetOpeners() {
	theOpenersLock.Lock()
	defer theOpenersLock.Unlock()

	for op := range theOpeners {
		delete(theOpeners, op)
	}
}

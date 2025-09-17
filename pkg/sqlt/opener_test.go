package sqlt

import (
	"database/sql"
	"errors"
	"net/url"
	"testing"
)

type mockOpener struct {
	id      string
	canOpen bool
	openErr error
	db      *sql.DB
}

func (m *mockOpener) Id() string {
	return m.id
}

func (m *mockOpener) CanOpen(u *url.URL) bool {
	return m.canOpen
}

func (m *mockOpener) Open(u *url.URL) (*sql.DB, error) {
	return m.db, m.openErr
}

func TestOpenDB_InvalidURLReturnsAnError(t *testing.T) {
	resetOpeners()

	_, err := OpenDB("%#@$")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestOpenDB_NoMatchingOpenerReturnsErrUnsupportedOpener(t *testing.T) {
	resetOpeners()

	_, err := OpenDB("notfound://user:pass@localhost/db")
	if !errors.Is(err, ErrUnsupportedOpener) {
		t.Errorf("expected ErrUnsupportedOpener, got %v", err)
	}
}

func TestOpenDB_MatchingOpenerReturnsTheOpenedDB(t *testing.T) {
	op := &mockOpener{
		id:      "mock",
		canOpen: true,
		db:      &sql.DB{},
	}
	resetOpeners()
	registerOpener(op)

	db, err := OpenDB("mock://anything")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if db != op.db {
		t.Errorf("expected returned db to match mock db")
	}
}

func TestOpen_MatchingOpenerReturnsError(t *testing.T) {
	op := &mockOpener{
		id:      "mock",
		canOpen: true,
		openErr: errors.New("fail"),
	}
	resetOpeners()
	registerOpener(op)

	_, err := OpenDB("mock://anything")
	if err == nil || err.Error() != "fail" {
		t.Errorf("expected error 'fail', got %v", err)
	}
}

func TestRegister_NilOpenerPanics(t *testing.T) {
	resetOpeners()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil opener, got none")
		}
	}()
	registerOpener(nil)
}

func TestRegister_DuplicatePanics(t *testing.T) {
	resetOpeners()
	op := &mockOpener{id: "dup"}
	registerOpener(op)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate opener, got none")
		}
	}()
	registerOpener(op)
}

func TestRegister_ValidOpener(t *testing.T) {
	op := &mockOpener{id: "valid"}
	resetOpeners()
	registerOpener(op)

	if got, ok := theOpeners["valid"]; !ok || got != op {
		t.Error("opener not registered correctly")
	}
}

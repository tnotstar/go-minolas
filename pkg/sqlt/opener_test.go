package sqlt

import (
	"database/sql"
	"errors"
	"net/url"
	"sync"
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

func TestOpen(t *testing.T) {
	mockDB := &sql.DB{}
	errFail := errors.New("fail")

	testCases := []struct {
		name       string
		dbURL      string
		openers    []Opener
		wantDB     *sql.DB
		wantErr    error
		setup      func()
		assertions func(t *testing.T, db *sql.DB, err error)
	}{
		{
			name:    "invalid url returns an error",
			dbURL:   "%#@$",
			wantErr: &url.Error{},
		},
		{
			name:    "no matching opener returns unsupported error",
			dbURL:   "notfound://user:pass@localhost/db",
			wantErr: ErrUnsupportedOpener,
		},
		{
			name:  "matching opener returns the opened db",
			dbURL: "mock://anything",
			openers: []Opener{
				&mockOpener{id: "mock", canOpen: true, db: mockDB},
			},
			wantDB: mockDB,
		},
		{
			name:  "matching opener returns an error",
			dbURL: "mock://anything",
			openers: []Opener{
				&mockOpener{id: "mock", canOpen: true, openErr: errFail},
			},
			wantErr: errFail,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ResetOpeners()
			for _, op := range tc.openers {
				RegisterOpener(op)
			}

			db, err := Open(tc.dbURL)

			if tc.wantErr != nil {
				// Check if the error is of the expected type or value
				if !errors.Is(err, tc.wantErr) {
					var urlErr *url.Error
					if _, ok := tc.wantErr.(*url.Error); ok && errors.As(err, &urlErr) {
						// Correctly identified a URL parsing error, so we're good.
					} else {
						t.Errorf("expected error '%v', got '%v'", tc.wantErr, err)
					}
				}
			} else if err != nil {
				t.Errorf("expected no error, got '%v'", err)
			}
			if db != tc.wantDB {
				t.Errorf("expected db '%v', got '%v'", tc.wantDB, db)
			}
		})
	}
}

func TestRegister_NilOpenerPanics(t *testing.T) {
	ResetOpeners()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil opener, got none")
		}
	}()
	RegisterOpener(nil)
}

func TestRegister_DuplicatePanics(t *testing.T) {
	ResetOpeners()
	op := &mockOpener{id: "dup"}
	RegisterOpener(op)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate opener, got none")
		}
	}()
	RegisterOpener(op)
}

func TestRegister_ValidOpener(t *testing.T) {
	op := &mockOpener{id: "valid"}
	ResetOpeners()
	RegisterOpener(op)

	if got, ok := openers["valid"]; !ok || got != op {
		t.Error("opener not registered correctly")
	}
}

func TestResetOpeners(t *testing.T) {
	ResetOpeners()
	RegisterOpener(&mockOpener{id: "test"})

	if len(openers) != 1 {
		t.Fatal("opener should have been registered")
	}

	ResetOpeners()

	if len(openers) != 0 {
		t.Errorf("expected openers to be empty, but got %d", len(openers))
	}
}

func TestConcurrency(t *testing.T) {
	ResetOpeners()
	// This test is designed to be run with the -race flag
	// to detect data races.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		RegisterOpener(&mockOpener{id: "concurrent", canOpen: true})
	}()
	go func() {
		defer wg.Done()
		_, _ = Open("concurrent://db")
	}()
	wg.Wait()
}

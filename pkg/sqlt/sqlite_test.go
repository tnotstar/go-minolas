package sqlt

import (
	"net/url"
	"testing"
)

func TestSqliteOpener_CanOpen(t *testing.T) {
	o := &SqliteOpener{}
	cases := []struct {
		u    string
		want bool
	}{
		{"sqlite::memory:", true},
		{"sqlite3::memory:", true},
		{"sqlite:///tmp/db.sqlite", true},
		{"mysql://localhost/db", false},
	}

	for _, c := range cases {
		u, err := url.Parse(c.u)
		if err != nil {
			t.Fatalf("failed to parse url %q: %v", c.u, err)
		}
		if got := o.CanOpen(u); got != c.want {
			t.Errorf("CanOpen(%q) = %v; want %v", c.u, got, c.want)
		}
	}
}

func TestSqliteOpener_Open_NilURL(t *testing.T) {
	o := &SqliteOpener{}
	if _, err := o.Open(nil); err == nil {
		t.Fatal("expected error for nil url")
	}
}

func TestSqliteOpener_Open_InvalidScheme(t *testing.T) {
	o := &SqliteOpener{}
	u, _ := url.Parse("mysql://x")
	if _, err := o.Open(u); err == nil {
		t.Fatal("expected error for invalid scheme")
	}
}

func TestSqliteOpener_Open_Memory(t *testing.T) {
	o := &SqliteOpener{}
	u, err := url.Parse("sqlite::memory:")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	db, err := o.Open(u)
	if err != nil {
		t.Fatalf("Open memory returned error: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	// close to release resources
	_ = db.Close()
}

func TestSqliteOpener_Open(t *testing.T) {
	o := &SqliteOpener{}
	cases := []struct {
		name    string
		u       string
		wantErr bool
	}{
		{"memory db", "sqlite::memory:", false},
		{"memory db with query", "sqlite::memory:?cache=shared", false},
		{"file db", "sqlite:test.db", false},
		{"file db with path", "sqlite:///tmp/test.db", false},
		{"file db with query", "sqlite:test.db?mode=ro", false},
		{"unsupported scheme", "mysql://localhost", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.u)
			if err != nil {
				t.Fatalf("failed to parse url %q: %v", tc.u, err)
			}
			db, err := o.Open(u)
			if (err != nil) != tc.wantErr {
				t.Errorf("Open() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if db != nil {
				_ = db.Close()
			}
		})
	}
}

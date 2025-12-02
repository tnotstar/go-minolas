package sqlt

import (
	"net/url"
	"testing"
)

func TestMsSqlOpener_CanOpen(t *testing.T) {
	o := &MsSqlOpener{}
	cases := []struct {
		u    string
		want bool
	}{
		{"sqlserver://localhost/mydb", true},
		{"sqlserver://server.database.windows.net?database=mydb", true},
		{"sqlite::memory:", false},
		{"mysql://localhost/db", false},
		{"postgres://localhost/db", false},
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

func TestMsSqlOpener_Open_NilURL(t *testing.T) {
	o := &MsSqlOpener{}
	if _, err := o.Open(nil); err == nil {
		t.Fatal("expected error for nil url")
	}
}

func TestMsSqlOpener_Open_InvalidScheme(t *testing.T) {
	o := &MsSqlOpener{}
	u, _ := url.Parse("mysql://x")
	if _, err := o.Open(u); err == nil {
		t.Fatal("expected error for invalid scheme")
	}
}

func TestMsSqlOpener_Id(t *testing.T) {
	o := &MsSqlOpener{}
	if got := o.Id(); got != "mssql" {
		t.Errorf("Id() = %q; want %q", got, "mssql")
	}
}

func TestMsSqlOpener_Open(t *testing.T) {
	o := &MsSqlOpener{}
	cases := []struct {
		name    string
		u       string
		wantErr bool
	}{
		{
			name:    "valid url with database",
			u:       "sqlserver://user:pass@localhost:1433?database=testdb",
			wantErr: false, // sql.Open doesn't actually connect, so no error
		},
		{
			name:    "azure sql database url",
			u:       "sqlserver://user:pass@myserver.database.windows.net?database=mydb",
			wantErr: false,
		},
		{
			name:    "url with connection parameters",
			u:       "sqlserver://localhost?database=mydb&encrypt=true",
			wantErr: false,
		},
		{
			name:    "unsupported scheme",
			u:       "mysql://localhost",
			wantErr: true,
		},
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

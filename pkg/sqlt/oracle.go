package sqlt

import (
	"database/sql"
	"errors"
	"net/url"
	"strconv"
	"strings"

	go_ora "github.com/sijms/go-ora/v2"
)

type OracleSQLOpener struct {
}

func (o *OracleSQLOpener) Id() string {
	return "oracle"
}

func (o *OracleSQLOpener) Open(u *url.URL) (*sql.DB, error) {
	if u == nil {
		return nil, errors.New("database url cannot be nil")
	}

	server := u.Hostname()
	p := u.Port()
	if p == "" {
		p = "1521"
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return nil, err
	}
	database := strings.TrimPrefix(u.EscapedPath(), "/")

	var user, passwd string
	user = u.User.Username()
	if pwd, ok := u.User.Password(); ok {
		passwd = pwd
	}
	dsn := go_ora.BuildUrl(server, port, database, user, passwd, nil)

	return sql.Open("oracle", dsn)
}

func (o *OracleSQLOpener) CanOpen(u *url.URL) bool {
	return u.Scheme == "oracle"
}

func init() {
	RegisterOpener(&OracleSQLOpener{})
}

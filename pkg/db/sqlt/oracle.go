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

// GetSchemas returns the list of schemas in Oracle.
func (o *OracleSQLOpener) GetSchemas(db *sql.DB) ([]DBSchemaInfo, error) {
	return []DBSchemaInfo{
		{Name: "HR"},
	}, nil
}

// GetTables returns sample tables for Oracle.
func (o *OracleSQLOpener) GetTables(db *sql.DB) ([]DBTableInfo, error) {
	return []DBTableInfo{
		{Schema: "HR", Name: "EMPLOYEES"},
		{Schema: "HR", Name: "DEPARTMENTS"},
	}, nil
}

// GetColumns returns sample columns for Oracle.
func (o *OracleSQLOpener) GetColumns(db *sql.DB) ([]DBColumnInfo, error) {
	return []DBColumnInfo{
		{Schema: "HR", Table: "EMPLOYEES", Name: "EMPLOYEE_ID", DataType: "NUMBER", IsPrimaryKey: true, IsNullable: false},
		{Schema: "HR", Table: "EMPLOYEES", Name: "FIRST_NAME", DataType: "VARCHAR2", IsPrimaryKey: false, IsNullable: true},
		{Schema: "HR", Table: "EMPLOYEES", Name: "LAST_NAME", DataType: "VARCHAR2", IsPrimaryKey: false, IsNullable: false},
		{Schema: "HR", Table: "EMPLOYEES", Name: "DEPARTMENT_ID", DataType: "NUMBER", IsPrimaryKey: false, IsNullable: true},
		{Schema: "HR", Table: "DEPARTMENTS", Name: "DEPARTMENT_ID", DataType: "NUMBER", IsPrimaryKey: true, IsNullable: false},
		{Schema: "HR", Table: "DEPARTMENTS", Name: "DEPARTMENT_NAME", DataType: "VARCHAR2", IsPrimaryKey: false, IsNullable: false},
	}, nil
}

// GetRelations returns sample relations for Oracle.
func (o *OracleSQLOpener) GetRelations(db *sql.DB) ([]DBRelationInfo, error) {
	return []DBRelationInfo{
		{
			ConstraintName: "FK_DEPT_ID",
			SourceSchema:   "HR",
			SourceTable:    "EMPLOYEES",
			SourceColumn:   "DEPARTMENT_ID",
			TargetSchema:   "HR",
			TargetTable:    "DEPARTMENTS",
			TargetColumn:   "DEPARTMENT_ID",
		},
	}, nil
}


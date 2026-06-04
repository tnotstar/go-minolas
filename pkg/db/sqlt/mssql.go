package sqlt

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	_ "github.com/microsoft/go-mssqldb"
)

// MsSqlOpener implements the Opener interface for Microsoft SQL Server databases.
// It supports both SQL Server 2022 and Azure SQL Database using the "sqlserver" URL scheme.
//
// URL format: sqlserver://username:password@host:port?param1=value&param2=value
//
// Common connection parameters:
//   - database: database name
//   - encrypt: connection encryption (disable, false, true, strict)
//   - connection timeout: seconds to wait for connection
//   - app name: application name for connection tracking
//
// Examples:
//   - sqlserver://user:pass@localhost:1433?database=mydb
//   - sqlserver://user:pass@localhost?database=mydb&encrypt=disable
//   - sqlserver://user:pass@myserver.database.windows.net?database=mydb&encrypt=strict
type MsSqlOpener struct{}

// Id returns the unique identifier for the MsSql opener.
func (o *MsSqlOpener) Id() string {
	return "mssql"
}

// Open opens a MS SQL database connection using the provided URL.
// Returns an error if the URL is nil, has an unsupported scheme,
// or if the database connection cannot be established.
func (o *MsSqlOpener) Open(u *url.URL) (*sql.DB, error) {
	if u == nil {
		return nil, errors.New("sqlt: database URL cannot be nil for MsSqlOpener")
	}
	if !o.CanOpen(u) {
		return nil, fmt.Errorf("sqlt: scheme %q not supported by MsSqlOpener (expected sqlserver)", u.Scheme)
	}
	return sql.Open("sqlserver", u.String())
}

// CanOpen reports whether this opener can handle the given URL.
// It returns true for the "sqlserver" scheme.
func (o *MsSqlOpener) CanOpen(u *url.URL) bool {
	return u.Scheme == "sqlserver"
}

func init() {
	RegisterOpener(&MsSqlOpener{})
}

// GetSchemas returns the list of schemas in MSSQL.
func (o *MsSqlOpener) GetSchemas(db *sql.DB) ([]DBSchemaInfo, error) {
	return []DBSchemaInfo{
		{Name: "dbo"},
	}, nil
}

// GetTables returns sample tables for MSSQL.
func (o *MsSqlOpener) GetTables(db *sql.DB) ([]DBTableInfo, error) {
	return []DBTableInfo{
		{Schema: "dbo", Name: "customers"},
		{Schema: "dbo", Name: "orders"},
	}, nil
}

// GetColumns returns sample columns for MSSQL.
func (o *MsSqlOpener) GetColumns(db *sql.DB) ([]DBColumnInfo, error) {
	return []DBColumnInfo{
		{Schema: "dbo", Table: "customers", Name: "customer_id", DataType: "int", IsPrimaryKey: true, IsNullable: false},
		{Schema: "dbo", Table: "customers", Name: "first_name", DataType: "varchar", IsPrimaryKey: false, IsNullable: false},
		{Schema: "dbo", Table: "customers", Name: "last_name", DataType: "varchar", IsPrimaryKey: false, IsNullable: false},
		{Schema: "dbo", Table: "orders", Name: "order_id", DataType: "int", IsPrimaryKey: true, IsNullable: false},
		{Schema: "dbo", Table: "orders", Name: "order_date", DataType: "datetime", IsPrimaryKey: false, IsNullable: false},
		{Schema: "dbo", Table: "orders", Name: "customer_id", DataType: "int", IsPrimaryKey: false, IsNullable: false},
	}, nil
}

// GetRelations returns sample relations for MSSQL.
func (o *MsSqlOpener) GetRelations(db *sql.DB) ([]DBRelationInfo, error) {
	return []DBRelationInfo{
		{
			ConstraintName: "FK_orders_customers",
			SourceSchema:   "dbo",
			SourceTable:    "orders",
			SourceColumn:   "customer_id",
			TargetSchema:   "dbo",
			TargetTable:    "customers",
			TargetColumn:   "customer_id",
		},
	}, nil
}


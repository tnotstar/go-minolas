package sqlt

import "database/sql"

// DBSchemaInfo represents database schema metadata (INFORMATION_SCHEMA.SCHEMATA).
type DBSchemaInfo struct {
	Name string
}

// DBTableInfo represents database table metadata (INFORMATION_SCHEMA.TABLES).
type DBTableInfo struct {
	Schema string
	Name   string
}

// DBColumnInfo represents database column metadata (INFORMATION_SCHEMA.COLUMNS).
type DBColumnInfo struct {
	Schema       string
	Table        string
	Name         string
	DataType     string
	IsPrimaryKey bool
	IsNullable   bool
	DefaultValue string
}

// DBRelationInfo represents foreign key relationships (INFORMATION_SCHEMA.KEY_COLUMN_USAGE).
type DBRelationInfo struct {
	ConstraintName string
	SourceSchema   string
	SourceTable    string
	SourceColumn   string
	TargetSchema   string
	TargetTable    string
	TargetColumn   string
}

// MetadataExtractor defines a generic interface to query database schema metadata.
type MetadataExtractor interface {
	GetSchemas(db *sql.DB) ([]DBSchemaInfo, error)
	GetTables(db *sql.DB) ([]DBTableInfo, error)
	GetColumns(db *sql.DB) ([]DBColumnInfo, error)
	GetRelations(db *sql.DB) ([]DBRelationInfo, error)
}

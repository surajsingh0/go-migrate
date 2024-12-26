package main

import (
	"database/sql"
	"fmt"
	"strings"
)

type DBConfig struct {
	Type string
	URL  string
}

// ParseDBURL parses the database URL and returns the type and modified URL if needed
func ParseDBURL(dbURL string) (*DBConfig, error) {
	parts := strings.SplitN(dbURL, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid database URL format. Expected: dbtype://connection-url")
	}

	dbType := strings.ToLower(parts[0])
	connectionURL := parts[1]

	switch dbType {
	case "postgres", "postgresql":
		return &DBConfig{Type: "postgres", URL: dbURL}, nil
	case "mysql":
		// MySQL driver doesn't expect mysql:// prefix
		return &DBConfig{Type: "mysql", URL: connectionURL}, nil
	case "sqlite", "sqlite3":
		// SQLite driver expects just the file path
		return &DBConfig{Type: "sqlite3", URL: connectionURL}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// initializeDB sets up the database connection and schema based on the database type
func initializeDB(config *DBConfig) (*sql.DB, error) {
	db, err := sql.Open(config.Type, config.URL)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	return db, nil
}

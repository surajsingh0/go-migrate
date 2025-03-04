package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type testDB struct {
	driver string
	url    string
	config Config
}

var testDatabases = []testDB{
	{
		driver: "sqlite3",
		url:    ":memory:",
		config: Config{DatabaseType: "sqlite3"},
	},
	// Uncomment to test with PostgreSQL
	/*
		{
			driver: "postgres",
			url:    "postgres://postgres:postgres@localhost:5432/migrations_test?sslmode=disable",
			config: Config{DatabaseType: "postgres"},
		},
	*/
}

// setupTestMigrations creates a temporary directory with a pair of migration files for version 1
// and another pair for version 2. It returns the directory path and a cleanup function.
func setupTestMigrations(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "migrations_test")
	if err != nil {
		t.Fatal(err)
	}

	// Test migration files: version 001 creates a "users" table; version 002 alters it.
	migrations := map[string]string{
		"001_create_users_up.sql": `
			CREATE TABLE users (
				id INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`,
		"001_create_users_down.sql": `
			DROP TABLE users;
		`,
		"002_add_email_up.sql": `
			ALTER TABLE users
			ADD COLUMN email TEXT;
		`,
		"002_add_email_down.sql": `
			ALTER TABLE users 
			DROP COLUMN email;
		`,
	}

	for filename, content := range migrations {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			os.RemoveAll(tempDir)
			t.Fatal(err)
		}
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	return tempDir, cleanup
}

func TestNew(t *testing.T) {
	for _, db := range testDatabases {
		t.Run(fmt.Sprintf("Database=%s", db.driver), func(t *testing.T) {
			conn, err := sql.Open(db.driver, db.url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			migrator := New(conn, "test_dir", db.config)
			if migrator == nil {
				t.Error("Expected non-nil migrator")
			}
			if migrator != nil && migrator.db != conn {
				t.Error("Expected db connection to be set")
			}
			if migrator != nil && migrator.migrationsDir != "test_dir" {
				t.Error("Expected migrations directory to be set")
			}
		})
	}
}

func TestInit(t *testing.T) {
	for _, db := range testDatabases {
		t.Run(fmt.Sprintf("Database=%s", db.driver), func(t *testing.T) {
			conn, err := sql.Open(db.driver, db.url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			migrator := New(conn, "test_dir", db.config)
			if err := migrator.Init(); err != nil {
				t.Fatalf("Failed to initialize migrations table: %v", err)
			}

			// Verify that the migrations table was created.
			var tableName string
			query := "SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'"
			if db.driver == "postgres" {
				query = "SELECT table_name FROM information_schema.tables WHERE table_name='schema_migrations'"
			}
			err = conn.QueryRow(query).Scan(&tableName)
			if err != nil {
				t.Errorf("Migrations table not found: %v", err)
			}
			if tableName != "schema_migrations" {
				t.Error("Expected migrations table to be created")
			}
		})
	}
}

func TestLoadMigrations(t *testing.T) {
	for _, db := range testDatabases {
		t.Run(fmt.Sprintf("Database=%s", db.driver), func(t *testing.T) {
			tempDir, cleanup := setupTestMigrations(t)
			defer cleanup()

			conn, err := sql.Open(db.driver, db.url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			migrator := New(conn, tempDir, db.config)
			if err := migrator.LoadMigrations(); err != nil {
				t.Fatalf("Failed to load migrations: %v", err)
			}
			// Expect 2 migrations (001 and 002).
			if len(migrator.migrations) != 2 {
				t.Errorf("Expected 2 migrations, got %d", len(migrator.migrations))
			}
			// Check that migrations are sorted by version.
			if migrator.migrations[0].Version != 1 || migrator.migrations[1].Version != 2 {
				t.Error("Expected migrations to be sorted by version")
			}
		})
	}
}

func TestMigrate(t *testing.T) {
	for _, db := range testDatabases {
		t.Run(fmt.Sprintf("Database=%s", db.driver), func(t *testing.T) {
			tempDir, cleanup := setupTestMigrations(t)
			defer cleanup()

			conn, err := sql.Open(db.driver, db.url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			migrator := New(conn, tempDir, db.config)
			if err := migrator.Init(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.LoadMigrations(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.Migrate(); err != nil {
				t.Fatalf("Failed to run migrations: %v", err)
			}

			// Verify that both migrations were applied.
			applied, err := migrator.GetAppliedMigrations()
			if err != nil {
				t.Fatal(err)
			}
			if len(applied) != 2 {
				t.Errorf("Expected 2 applied migrations, got %d", len(applied))
			}

			// Verify the "users" table structure: the email column should exist.
			var email string
			err = conn.QueryRow("SELECT email FROM users WHERE 1=0").Scan(&email)
			if err != nil && err != sql.ErrNoRows {
				t.Errorf("Users table structure incorrect: %v", err)
			}
		})
	}
}

// TestRollback verifies that a single rollback (steps = 1) removes the most recent migration.
func TestRollback(t *testing.T) {
	for _, db := range testDatabases {
		t.Run(fmt.Sprintf("Database=%s", db.driver), func(t *testing.T) {
			tempDir, cleanup := setupTestMigrations(t)
			defer cleanup()

			conn, err := sql.Open(db.driver, db.url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			migrator := New(conn, tempDir, db.config)
			if err := migrator.Init(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.LoadMigrations(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.Migrate(); err != nil {
				t.Fatal(err)
			}

			// Rollback the last applied migration.
			if err := migrator.Rollback(1); err != nil {
				t.Fatalf("Failed to rollback 1 migration: %v", err)
			}

			applied, err := migrator.GetAppliedMigrations()
			if err != nil {
				t.Fatal(err)
			}
			if len(applied) != 1 {
				t.Errorf("Expected 1 applied migration after rollback, got %d", len(applied))
			}

			// Verify that the email column has been removed.
			var columns string
			if db.driver == "postgres" {
				columns = "SELECT column_name FROM information_schema.columns WHERE table_name='users' AND column_name='email'"
			} else {
				columns = "SELECT name FROM pragma_table_info('users') WHERE name='email'"
			}
			rows, err := conn.Query(columns)
			if err != nil {
				t.Fatal(err)
			}
			if rows.Next() {
				t.Error("Email column should have been removed after rollback")
			}
			rows.Close()
		})
	}
}

// TestRollbackMultipleSteps verifies that rolling back more than one migration works correctly.
func TestRollbackMultipleSteps(t *testing.T) {
	for _, db := range testDatabases {
		t.Run(fmt.Sprintf("Database=%s", db.driver), func(t *testing.T) {
			tempDir, cleanup := setupTestMigrations(t)
			defer cleanup()

			conn, err := sql.Open(db.driver, db.url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			migrator := New(conn, tempDir, db.config)
			if err := migrator.Init(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.LoadMigrations(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.Migrate(); err != nil {
				t.Fatal(err)
			}

			// Rollback both migrations.
			if err := migrator.Rollback(2); err != nil {
				t.Fatalf("Failed to rollback 2 migrations: %v", err)
			}

			applied, err := migrator.GetAppliedMigrations()
			if err != nil {
				t.Fatal(err)
			}
			if len(applied) != 0 {
				t.Errorf("Expected 0 applied migrations after rolling back all, got %d", len(applied))
			}

			// Verify that the users table no longer exists.
			var tableName string
			query := "SELECT name FROM sqlite_master WHERE type='table' AND name='users'"
			if db.driver == "postgres" {
				query = "SELECT table_name FROM information_schema.tables WHERE table_name='users'"
			}
			err = conn.QueryRow(query).Scan(&tableName)
			if err == nil {
				t.Error("Users table should have been dropped after rolling back all migrations")
			}
		})
	}
}

func TestGetAppliedMigrations(t *testing.T) {
	for _, db := range testDatabases {
		t.Run(fmt.Sprintf("Database=%s", db.driver), func(t *testing.T) {
			tempDir, cleanup := setupTestMigrations(t)
			defer cleanup()

			conn, err := sql.Open(db.driver, db.url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			migrator := New(conn, tempDir, db.config)
			if err := migrator.Init(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.LoadMigrations(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.Migrate(); err != nil {
				t.Fatal(err)
			}

			applied, err := migrator.GetAppliedMigrations()
			if err != nil {
				t.Fatal(err)
			}
			if len(applied) != 2 {
				t.Errorf("Expected 2 applied migrations, got %d", len(applied))
			}

			// Ensure that migration timestamps are not in the future.
			for _, ts := range applied {
				if ts.After(time.Now()) {
					t.Error("Migration timestamp should not be in the future")
				}
			}
		})
	}
}

func TestParseMigrationFilename(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		wantVersion    int
		wantName       string
		wantDirection  string
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:          "valid up migration",
			filename:      "001_create_users_up.sql",
			wantVersion:   1,
			wantName:      "create_users",
			wantDirection: "up",
			wantErr:       false,
		},
		{
			name:          "valid down migration",
			filename:      "002_add_email_down.sql",
			wantVersion:   2,
			wantName:      "add_email",
			wantDirection: "down",
			wantErr:       false,
		},
		{
			name:          "complex name migration",
			filename:      "003_create_user_posts_comments_up.sql",
			wantVersion:   3,
			wantName:      "create_user_posts_comments",
			wantDirection: "up",
			wantErr:       false,
		},
		{
			name:           "invalid extension",
			filename:       "001_create_users_up.txt",
			wantErr:        true,
			expectedErrMsg: "file must have .sql extension",
		},
		{
			name:           "missing parts",
			filename:       "001_up.sql",
			wantErr:        true,
			expectedErrMsg: "filename must have at least version, name, and direction parts",
		},
		{
			name:           "invalid version",
			filename:       "abc_create_users_up.sql",
			wantErr:        true,
			expectedErrMsg: "invalid version number",
		},
		{
			name:           "invalid direction",
			filename:       "001_create_users_invalid.sql",
			wantErr:        true,
			expectedErrMsg: "direction must be 'up' or 'down'",
		},
		{
			name:           "empty name",
			filename:       "001__up.sql",
			wantErr:        true,
			expectedErrMsg: "name part cannot be empty",
		},
		{
			name:           "no extension",
			filename:       "001_create_users_up",
			wantErr:        true,
			expectedErrMsg: "file must have .sql extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migrator := &Migrator{}
			gotVersion, gotName, gotDirection, err := migrator.parseMigrationFilename(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseMigrationFilename() error = nil, wanted error containing %q", tt.expectedErrMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("parseMigrationFilename() error = %v, wanted error containing %q", err, tt.expectedErrMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("parseMigrationFilename() unexpected error = %v", err)
				return
			}
			if gotVersion != tt.wantVersion {
				t.Errorf("gotVersion = %v, want %v", gotVersion, tt.wantVersion)
			}
			if gotName != tt.wantName {
				t.Errorf("gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotDirection != tt.wantDirection {
				t.Errorf("gotDirection = %v, want %v", gotDirection, tt.wantDirection)
			}
		})
	}
}

// TestIntegration simulates a full migration lifecycle.
func TestIntegration(t *testing.T) {
	for _, db := range testDatabases {
		t.Run(fmt.Sprintf("Database=%s", db.driver), func(t *testing.T) {
			tempDir, cleanup := setupTestMigrations(t)
			defer cleanup()

			conn, err := sql.Open(db.driver, db.url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()

			migrator := New(conn, tempDir, db.config)
			steps := []struct {
				name string
				fn   func() error
			}{
				{"Init", migrator.Init},
				{"LoadMigrations", migrator.LoadMigrations},
				{"Migrate", migrator.Migrate},
				{"Rollback 1 step", func() error { return migrator.Rollback(1) }},
				{"Migrate Again", migrator.Migrate},
			}
			for _, step := range steps {
				t.Run(step.name, func(t *testing.T) {
					if err := step.fn(); err != nil {
						t.Fatalf("%s failed: %v", step.name, err)
					}
				})
			}
			applied, err := migrator.GetAppliedMigrations()
			if err != nil {
				t.Fatal(err)
			}
			if len(applied) != 2 {
				t.Errorf("Expected 2 applied migrations, got %d", len(applied))
			}
		})
	}
}

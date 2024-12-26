package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	/*{
		driver: "postgres",
		url:    "postgres://postgres:postgres@localhost:5432/migrations_test?sslmode=disable",
		config: Config{DatabaseType: "postgres"},
	},*/
}

func setupTestMigrations(t *testing.T) (string, func()) {
	// Create temporary directory for migrations
	tempDir, err := os.MkdirTemp("", "migrations_test")
	if err != nil {
		t.Fatal(err)
	}

	// Create test migration files
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

	// Return cleanup function
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
			if migrator.db != conn {
				t.Error("Expected db connection to be set")
			}
			if migrator.migrationsDir != "test_dir" {
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

			// Verify migrations table exists
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

			if len(migrator.migrations) != 2 {
				t.Errorf("Expected 2 migrations, got %d", len(migrator.migrations))
			}

			// Check migrations are sorted
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

			// Initialize and load migrations
			if err := migrator.Init(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.LoadMigrations(); err != nil {
				t.Fatal(err)
			}

			// Run migrations
			if err := migrator.Migrate(); err != nil {
				t.Fatalf("Failed to run migrations: %v", err)
			}

			// Verify migrations were applied
			applied, err := migrator.GetAppliedMigrations()
			if err != nil {
				t.Fatal(err)
			}

			if len(applied) != 2 {
				t.Errorf("Expected 2 applied migrations, got %d", len(applied))
			}

			// Verify users table structure
			var email string
			err = conn.QueryRow("SELECT email FROM users WHERE 1=0").Scan(&email)
			if err != nil && err != sql.ErrNoRows {
				t.Errorf("Users table structure incorrect: %v", err)
			}
		})
	}
}

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

			// Initialize and run migrations
			if err := migrator.Init(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.LoadMigrations(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.Migrate(); err != nil {
				t.Fatal(err)
			}

			// Rollback last migration
			if err := migrator.Rollback(); err != nil {
				t.Fatalf("Failed to rollback: %v", err)
			}

			// Verify only one migration remains
			applied, err := migrator.GetAppliedMigrations()
			if err != nil {
				t.Fatal(err)
			}

			if len(applied) != 1 {
				t.Errorf("Expected 1 applied migration after rollback, got %d", len(applied))
			}

			// Verify email column was removed
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
				t.Error("Email column should have been removed")
			}
			rows.Close()
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

			// Initialize and apply migrations
			if err := migrator.Init(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.LoadMigrations(); err != nil {
				t.Fatal(err)
			}
			if err := migrator.Migrate(); err != nil {
				t.Fatal(err)
			}

			// Get applied migrations
			applied, err := migrator.GetAppliedMigrations()
			if err != nil {
				t.Fatal(err)
			}

			// Check results
			if len(applied) != 2 {
				t.Errorf("Expected 2 applied migrations, got %d", len(applied))
			}

			// Verify timestamps
			for _, timestamp := range applied {
				if timestamp.After(time.Now()) {
					t.Error("Migration timestamp should not be in the future")
				}
			}
		})
	}
}

// Integration test that simulates real usage
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

			// Test full migration lifecycle
			steps := []struct {
				name string
				fn   func() error
			}{
				{"Init", migrator.Init},
				{"LoadMigrations", migrator.LoadMigrations},
				{"Migrate", migrator.Migrate},
				{"Rollback", migrator.Rollback},
				{"Migrate Again", migrator.Migrate},
			}

			for _, step := range steps {
				t.Run(step.name, func(t *testing.T) {
					if err := step.fn(); err != nil {
						t.Fatalf("%s failed: %v", step.name, err)
					}
				})
			}

			// Verify final state
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

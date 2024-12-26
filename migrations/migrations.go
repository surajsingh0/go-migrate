package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Config struct {
	DatabaseType string
}

// Migration represents a single database migration
type Migration struct {
	Version   int
	Name      string
	UpSQL     string
	DownSQL   string
	AppliedAt *time.Time
}

// Migrator handles database migrations
type Migrator struct {
	db            *sql.DB
	migrationsDir string
	config        Config
	migrations    []*Migration
}

// dbDialect encapsulates database-specific behaviors
type dbDialect struct {
	createTableSQL string
	placeholder    func(int) string
}

// getDialect returns the appropriate dialect for the database type
func getDialect(dbType string) (*dbDialect, error) {
	dialects := map[string]dbDialect{
		"postgres": {
			createTableSQL: `
				CREATE TABLE IF NOT EXISTS schema_migrations (
					version INTEGER PRIMARY KEY,
					name TEXT NOT NULL,
					applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				)`,
			placeholder: func(i int) string { return fmt.Sprintf("$%d", i) },
		},
		"mysql": {
			createTableSQL: `
				CREATE TABLE IF NOT EXISTS schema_migrations (
					version INTEGER PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`,
			placeholder: func(i int) string { return "?" },
		},
		"sqlite3": {
			createTableSQL: `
				CREATE TABLE IF NOT EXISTS schema_migrations (
					version INTEGER PRIMARY KEY,
					name TEXT NOT NULL,
					applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`,
			placeholder: func(i int) string { return "?" },
		},
	}

	dialect, ok := dialects[dbType]
	if !ok {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	return &dialect, nil
}

func New(db *sql.DB, migrationsDir string, config Config) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: migrationsDir,
		config:        config,
	}
}

// Init creates the migrations table if it doesn't exist
func (m *Migrator) Init() error {
	dialect, err := getDialect(m.config.DatabaseType)
	if err != nil {
		return err
	}

	_, err = m.db.Exec(dialect.createTableSQL)
	return err
}

// LoadMigrations reads all migration files from the migrations directory
func (m *Migrator) LoadMigrations() error {
	files, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return err
	}

	// Group up and down files
	migrationFiles := make(map[int]map[string]string)
	for _, file := range files {
		fmt.Println(file.Name(), strings.HasSuffix(file.Name(), ".sql"))
		if strings.HasSuffix(file.Name(), ".sql") {
			var version int
			var _, direction string
			version, _, direction, err := m.parseMigrationFilename(file.Name())
			if err != nil {
				continue
			}

			if migrationFiles[version] == nil {
				migrationFiles[version] = make(map[string]string)
			}

			content, err := os.ReadFile(filepath.Join(m.migrationsDir, file.Name()))
			if err != nil {
				return err
			}

			migrationFiles[version][direction] = string(content)
		}
	}

	// Create migration objects
	for version, files := range migrationFiles {
		m.migrations = append(m.migrations, &Migration{
			Version: version,
			Name:    fmt.Sprintf("%d", version),
			UpSQL:   files["up"],
			DownSQL: files["down"],
		})
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	return nil
}

// GetAppliedMigrations retrieves all applied migrations from the database
func (m *Migrator) GetAppliedMigrations() (map[int]time.Time, error) {
	rows, err := m.db.Query("SELECT version, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]time.Time)
	for rows.Next() {
		var version int
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return nil, err
		}
		applied[version] = appliedAt
	}

	return applied, nil
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate() error {
	dialect, err := getDialect(m.config.DatabaseType)
	if err != nil {
		return err
	}

	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	// Run pending migrations
	for _, migration := range m.migrations {
		if _, ok := applied[migration.Version]; !ok {
			// Start transaction
			tx, err := m.db.Begin()
			if err != nil {
				return err
			}

			// Apply migration
			if _, err := tx.Exec(migration.UpSQL); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to apply migration %d: %v", migration.Version, err)
			}

			// Record migration using database-specific placeholders
			insertSQL := fmt.Sprintf(
				"INSERT INTO schema_migrations (version, name, applied_at) VALUES (%s, %s, %s)",
				dialect.placeholder(1),
				dialect.placeholder(2),
				dialect.placeholder(3),
			)

			_, err = tx.Exec(insertSQL, migration.Version, migration.Name, time.Now())
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to record migration %d: %v", migration.Version, err)
			}

			// Commit transaction
			if err := tx.Commit(); err != nil {
				return err
			}

			fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Name)
		}
	}

	return nil
}

// Rollback reverts the last applied migration
func (m *Migrator) Rollback() error {
	dialect, err := getDialect(m.config.DatabaseType)
	if err != nil {
		return err
	}

	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		return errors.New("no migrations to rollback")
	}

	// Find last applied migration
	var lastVersion int
	var lastAppliedAt time.Time
	for version, appliedAt := range applied {
		if appliedAt.After(lastAppliedAt) {
			lastVersion = version
			lastAppliedAt = appliedAt
		}
	}

	// Find migration object
	var migration *Migration
	for _, m := range m.migrations {
		if m.Version == lastVersion {
			migration = m
			break
		}
	}

	if migration == nil {
		return fmt.Errorf("migration %d not found", lastVersion)
	}

	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	// Revert migration
	if _, err := tx.Exec(migration.DownSQL); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to rollback migration %d: %v", lastVersion, err)
	}

	// Remove migration record using database-specific placeholder
	deleteSQL := fmt.Sprintf("DELETE FROM schema_migrations WHERE version = %s", dialect.placeholder(1))
	if _, err := tx.Exec(deleteSQL, lastVersion); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove migration record %d: %v", lastVersion, err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	fmt.Printf("Rolled back migration %d: %s\n", migration.Version, migration.Name)
	return nil
}

func (m *Migrator) parseMigrationFilename(filename string) (version int, name, direction string, err error) {
	if !strings.HasSuffix(filename, ".sql") {
		return 0, "", "", fmt.Errorf("file must have .sql extension")
	}

	parts := strings.Split(strings.TrimSuffix(filename, ".sql"), "_")
	if len(parts) < 3 {
		return 0, "", "", fmt.Errorf("filename must have at least version, name, and direction parts")
	}

	_, err = fmt.Sscanf(parts[0], "%d", &version)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid version number: %v", err)
	}

	direction = parts[len(parts)-1]
	if direction != "up" && direction != "down" {
		return 0, "", "", fmt.Errorf("direction must be 'up' or 'down'")
	}

	name = strings.Join(parts[1:len(parts)-1], "_")
	if name == "" {
		return 0, "", "", fmt.Errorf("name part cannot be empty")
	}

	return version, name, direction, nil
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/surajsingh0/go-migrate-easy/migrations"
)

func main() {
	dbURL := flag.String("db", "", "Database connection URL (format: dbtype://connection-url)")
	migrationsDir := flag.String("dir", "migrations", "Migrations directory")
	command := flag.String("command", "up", "Command to run (up, down, create)")
	name := flag.String("name", "", "Migration name (required for create)")
	steps := flag.Int("steps", 1, "Number of migrations to rollback (only used with 'down' command)")
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("Database URL is required")
	}

	dbConfig, err := ParseDBURL(*dbURL)
	if err != nil {
		log.Fatal(err)
	}

	db, err := initializeDB(dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	migrator := migrations.New(db, *migrationsDir, migrations.Config{
		DatabaseType: dbConfig.Type,
		// Add any database-specific configuration here
	})

	if err := migrator.Init(); err != nil {
		log.Fatal(err)
	}

	if err := migrator.LoadMigrations(); err != nil {
		log.Fatal(err)
	}

	switch *command {
	case "up":
		if err := migrator.Migrate(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Migrations completed successfully")

	case "down":
		if err := migrator.Rollback(*steps); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Rollback of %d migration(s) completed successfully\n", *steps)

	case "create":
		if *name == "" {
			log.Fatal("Migration name is required for create command")
		}
		if err := createMigrationFiles(*migrationsDir, *name, dbConfig.Type); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Created migration files for %s\n", *name)

	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}

func createMigrationFiles(dir, name, dbType string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	version := len(files)/2 + 1

	// Template for different database types
	var upTemplate, downTemplate string
	switch dbType {
	case "postgres":
		upTemplate = "-- PostgreSQL up migration\n"
		downTemplate = "-- PostgreSQL down migration\n"
	case "mysql":
		upTemplate = "-- MySQL up migration\n"
		downTemplate = "-- MySQL down migration\n"
	case "sqlite3":
		upTemplate = "-- SQLite up migration\n"
		downTemplate = "-- SQLite down migration\n"
	default:
		upTemplate = "-- Add migration up SQL here\n"
		downTemplate = "-- Add migration down SQL here\n"
	}

	// Create up migration
	upFile := fmt.Sprintf("%s/%03d_%s_up.sql", dir, version, name)
	if err := os.WriteFile(upFile, []byte(upTemplate), 0644); err != nil {
		return err
	}

	// Create down migration
	downFile := fmt.Sprintf("%s/%03d_%s_down.sql", dir, version, name)
	if err := os.WriteFile(downFile, []byte(downTemplate), 0644); err != nil {
		return err
	}

	return nil
}

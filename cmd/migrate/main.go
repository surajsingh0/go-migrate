package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/surajsingh0/go-migrate-easy/migrations"
)

func main() {
	dbURL := flag.String("db", "", "Database connection URL")
	migrationsDir := flag.String("dir", "migrations", "Migrations directory")
	command := flag.String("command", "up", "Command to run (up, down, create)")
	name := flag.String("name", "", "Migration name (required for create)")
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("Database URL is required")
	}

	db, err := sql.Open("postgres", *dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	migrator := migrations.New(db, *migrationsDir)

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
		if err := migrator.Rollback(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Rollback completed successfully")

	case "create":
		if *name == "" {
			log.Fatal("Migration name is required for create command")
		}
		if err := createMigrationFiles(*migrationsDir, *name); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Created migration files for %s\n", *name)

	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}

func createMigrationFiles(dir string, name string) error {
	// Ensure migrations directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Get next version number
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	version := len(files)/2 + 1

	// Create up migration
	upFile := fmt.Sprintf("%s/%03d_%s_up.sql", dir, version, name)
	if err := os.WriteFile(upFile, []byte("-- Add migration up SQL here\n"), 0644); err != nil {
		return err
	}

	// Create down migration
	downFile := fmt.Sprintf("%s/%03d_%s_down.sql", dir, version, name)
	if err := os.WriteFile(downFile, []byte("-- Add migration down SQL here\n"), 0644); err != nil {
		return err
	}

	return nil
}

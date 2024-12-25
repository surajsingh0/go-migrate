# Go-Migrate-Easy

A simple, robust database migration library for Go applications. This library provides both a programmatic API and CLI tool for managing database migrations.

## Features

- ✨ Simple API for programmatic use
- 🛠 CLI tool for manual migration management
- 📦 Automatic version tracking
- ⚡ Transaction support
- 🔄 Up/Down migrations
- 🚀 Easy to integrate
- 📝 Descriptive logging

## Installation

```bash
go get github.com/surajsingh0/go-migrate-easy
```

## CLI Usage

### Creating a New Migration

```bash
# Create new migration files
migrate -db="postgres://user:pass@localhost:5432/dbname" -command=create -name=create_users

# This creates:
# migrations/001_create_users_up.sql
# migrations/001_create_users_down.sql
```

### Writing Migrations

```sql
-- migrations/001_create_users_up.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- migrations/001_create_users_down.sql
DROP TABLE users;
```

### Running Migrations

```bash
# Apply all pending migrations
migrate -db="postgres://user:pass@localhost:5432/dbname" -command=up

# Rollback last migration
migrate -db="postgres://user:pass@localhost:5432/dbname" -command=down
```

### CLI Options

```bash
migrate [options]

Options:
  -db string       Database connection URL (required)
  -dir string      Migrations directory (default "migrations")
  -command string  Command to run (up, down, create)
  -name string     Migration name (required for create)
```

## Programmatic Usage

### Basic Example

```go
package main

import (
    "database/sql"
    "log"
    
    "github.com/surajsingh0/go-migrate-easy/migrations"
    _ "github.com/lib/pq"
)

func main() {
    // Connect to database
    db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/dbname")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create migrator
    migrator := migrations.New(db, "migrations")

    // Initialize migrations table
    if err := migrator.Init(); err != nil {
        log.Fatal(err)
    }

    // Load and run migrations
    if err := migrator.LoadMigrations(); err != nil {
        log.Fatal(err)
    }

    if err := migrator.Migrate(); err != nil {
        log.Fatal(err)
    }
}
```

### Advanced Usage

```go
package main

import (
    "database/sql"
    "log"
    
    "github.com/surajsingh0/go-migrate-easy/migrations"
)

func main() {
    db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/dbname")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    migrator := migrations.New(db, "migrations")

    // Initialize
    if err := migrator.Init(); err != nil {
        log.Fatal(err)
    }

    // Get applied migrations
    applied, err := migrator.GetAppliedMigrations()
    if err != nil {
        log.Fatal(err)
    }

    // Check if specific migration is applied
    if _, ok := applied[1]; ok {
        log.Println("Migration 1 is applied")
    }

    // Rollback last migration
    if err := migrator.Rollback(); err != nil {
        log.Fatal(err)
    }
}
```

## Migration File Format

Migration files should follow this naming convention:
```
{version}_{name}_{direction}.sql

Examples:
001_create_users_up.sql
001_create_users_down.sql
002_add_email_up.sql
002_add_email_down.sql
```

## Best Practices

1. **Always Include Down Migrations**
   - Make sure each migration has both up and down SQL
   - Test both directions before committing

2. **Keep Migrations Small**
   - One logical change per migration
   - Easier to debug and rollback

3. **Use Transactions**
   - The library handles transactions automatically
   - Ensures database consistency

4. **Version Control**
   - Commit migrations with your code
   - Never modify existing migrations
   - Create new migrations for changes

5. **Testing**
   - Test migrations on development database first
   - Include sample data in tests

## Common Scenarios

### Adding a New Table

```sql
-- 001_create_users_up.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL
);

-- 001_create_users_down.sql
DROP TABLE users;
```

### Adding a Column

```sql
-- 002_add_email_up.sql
ALTER TABLE users
ADD COLUMN email VARCHAR(255);

-- 002_add_email_down.sql
ALTER TABLE users
DROP COLUMN email;
```

### Adding a Foreign Key

```sql
-- 003_create_posts_up.sql
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    title VARCHAR(255) NOT NULL
);

-- 003_create_posts_down.sql
DROP TABLE posts;
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

If you encounter any issues or have questions, please file an issue on GitHub.
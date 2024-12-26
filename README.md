# Go-Migrate-Easy

A simple, robust database migration package/library and CLI tool. Designed for seamless integration with Go applications through a programmatic API, the CLI tool also allows developers using any language or framework to manage database migrations effortlessly across multiple database types.

## Features

- ✨ Simple API for programmatic use
- 🛠 CLI tool for use with any programming language or framework
- 📦 Automatic version tracking
- ⚡ Transaction support
- 🔄 Up/Down migrations
- 🚀 Easy to integrate
- 📝 Descriptive logging
- 🔌 Multiple database support (PostgreSQL, MySQL, SQLite)

## Installation

### Binary Installation

```bash
# Using go install
go install github.com/surajsingh0/go-migrate-easy/cmd/migrate@latest

# From source
git clone https://github.com/surajsingh0/go-migrate-easy.git
cd go-migrate-easy
go build -o migrate cmd/migrate/main.go
```

### Library Installation

```bash
# Install the library
go get github.com/surajsingh0/go-migrate-easy

# Install database drivers as needed
go get github.com/lib/pq             # PostgreSQL
go get github.com/go-sql-driver/mysql # MySQL
go get github.com/mattn/go-sqlite3    # SQLite
```

## CLI Usage

### Database Connection URLs

```bash
# PostgreSQL
migrate -db="postgres://user:pass@localhost:5432/dbname"

# MySQL
migrate -db="mysql://user:pass@localhost:3306/dbname"

# SQLite
migrate -db="sqlite:///path/to/database.db"
```

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
-- PostgreSQL Example
-- migrations/001_create_users_up.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- MySQL Example
-- migrations/001_create_users_up.sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- SQLite Example
-- migrations/001_create_users_up.sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
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
    _ "github.com/lib/pq"           // PostgreSQL driver
    _ "github.com/go-sql-driver/mysql" // MySQL driver
    _ "github.com/mattn/go-sqlite3"    // SQLite driver
)

func main() {
    // Connect to database (choose appropriate driver and URL)
    db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/dbname")
    // OR for MySQL:
    // db, err := sql.Open("mysql", "user:pass@tcp(localhost:3306)/dbname")
    // OR for SQLite:
    // db, err := sql.Open("sqlite3", "./database.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create migrator with database type
    migrator := migrations.New(db, "migrations", migrations.Config{
        DatabaseType: "postgres", // or "mysql" or "sqlite3"
    })

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
    // Connect to your preferred database
    db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/dbname")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    migrator := migrations.New(db, "migrations", migrations.Config{
        DatabaseType: "postgres",
    })

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

## Database-Specific Considerations

### PostgreSQL
- Uses `$1, $2, ...` for query parameters
- Supports `TIMESTAMP WITH TIME ZONE`
- Uses `SERIAL` for auto-incrementing IDs

### MySQL
- Uses `?` for query parameters
- Default timestamp is `TIMESTAMP`
- Uses `AUTO_INCREMENT` for auto-incrementing IDs

### SQLite
- Uses `?` for query parameters
- Uses `AUTOINCREMENT` for auto-incrementing IDs
- Some constraints and types may differ

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

1. **Database Compatibility**
   - Test migrations on all supported databases
   - Use database-specific features carefully
   - Consider using common SQL features when possible

2. **Always Include Down Migrations**
   - Make sure each migration has both up and down SQL
   - Test both directions before committing

3. **Keep Migrations Small**
   - One logical change per migration
   - Easier to debug and rollback

4. **Use Transactions**
   - The library handles transactions automatically
   - Ensures database consistency

5. **Version Control**
   - Commit migrations with your code
   - Never modify existing migrations
   - Create new migrations for changes

6. **Testing**
   - Test migrations on development database first
   - Include sample data in tests
   - Test across all supported databases

## Common Scenarios

### Adding a New Table

```sql
-- PostgreSQL
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL
);

-- MySQL
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(100) NOT NULL
);

-- SQLite
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL
);
```

### Adding a Column

```sql
-- Works across all supported databases
ALTER TABLE users
ADD COLUMN email VARCHAR(255);
```

### Adding a Foreign Key

```sql
-- PostgreSQL and MySQL
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    title VARCHAR(255) NOT NULL
);

-- SQLite
CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    title TEXT NOT NULL
);
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

If you encounter any issues or have questions, please file an issue on GitHub.
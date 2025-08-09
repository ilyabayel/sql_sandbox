# SQL Sandbox - PostgreSQL Test Database Library

A Go library for creating isolated PostgreSQL database sandboxes for unit testing. This library provides a clean, isolated database environment for each test, ensuring tests don't interfere with each other.

## Features

- ✅ **Automatic Migration Check**: Ensures the main database is migrated to the latest version before creating test databases
- ✅ **Template-based Database Creation**: Creates test databases from a template database for fast setup
- ✅ **Unique Database Names**: Generates unique database names to prevent conflicts
- ✅ **Automatic Cleanup**: Automatically drops test databases after each test
- ✅ **Dependency Injection Ready**: Easy integration with dependency injection patterns
- ✅ **Thread-safe**: Safe for concurrent test execution
- ✅ **Configurable**: Flexible configuration options

## Installation

```bash
go get github.com/ilyabayel/sql_sandbox
```

## Quick Start

```go
package main

import (
    "database/sql"
    "testing"
    "time"
    
    "github.com/ilyabayel/sql_sandbox"
)

func TestUserService(t *testing.T) {
    // Create sandbox with your main database connection
    mainDBURL := "postgres://username:password@localhost:5432/main_db?sslmode=disable"
    
    sandbox, err := sql_sandbox.New(mainDBURL, nil)
    if err != nil {
        t.Fatal(err)
    }
    defer sandbox.Close() // This will clean up the test database
    
    // Get the test database connection
    db := sandbox.DB()
    
    // Use the database in your tests
    // The database is a fresh copy of your main database
    // with all migrations applied
}
```

## Configuration

You can customize the sandbox behavior using the `Config` struct:

```go
config := &sql_sandbox.Config{
    TemplateDBName:  "template_test",     // Name of the template database
    TestDBPrefix:    "test_db_",          // Prefix for test database names
    MaxConnections:  10,                  // Max connections in the pool
    ConnectionTimeout: 30 * time.Second,  // Connection timeout
}

sandbox, err := sql_sandbox.New(mainDBURL, config)
```

## How It Works

### 1. Migration Check
Before creating any test databases, the library checks that your main database is migrated to the latest version. This ensures all test databases start with the correct schema.

### 2. Template Database
The library creates a template database from your main database. This template is used to quickly create test databases, making the setup process much faster than running migrations for each test.

### 3. Test Database Creation
For each test, a unique test database is created from the template. The database name includes a timestamp and process ID to ensure uniqueness.

### 4. Cleanup
When `sandbox.Close()` is called, the test database is automatically dropped, ensuring no leftover data between tests.

## Usage Examples

### Basic Usage

```go
func TestBasicDatabase(t *testing.T) {
    mainDBURL := "postgres://user:pass@localhost:5432/main_db"
    
    sandbox, err := sql_sandbox.New(mainDBURL, nil)
    require.NoError(t, err)
    defer sandbox.Close()
    
    db := sandbox.DB()
    
    // Your test code here
    _, err = db.Exec("CREATE TABLE test_table (id SERIAL PRIMARY KEY, name TEXT)")
    require.NoError(t, err)
}
```

### With Dependency Injection

```go
type UserService struct {
    db *sql.DB
}

func NewUserService(db *sql.DB) *UserService {
    return &UserService{db: db}
}

func TestUserService(t *testing.T) {
    mainDBURL := "postgres://user:pass@localhost:5432/main_db"
    
    sandbox, err := sql_sandbox.New(mainDBURL, nil)
    require.NoError(t, err)
    defer sandbox.Close()
    
    // Inject the test database into your service
    userService := NewUserService(sandbox.DB())
    
    // Test your service
    user, err := userService.CreateUser("John Doe", "john@example.com")
    require.NoError(t, err)
    assert.Equal(t, "John Doe", user.Name)
}
```

### Parallel Tests

The library is thread-safe and supports parallel test execution:

```go
func TestParallel(t *testing.T) {
    t.Parallel() // Safe to use with sql_sandbox
    
    mainDBURL := "postgres://user:pass@localhost:5432/main_db"
    
    sandbox, err := sql_sandbox.New(mainDBURL, nil)
    require.NoError(t, err)
    defer sandbox.Close()
    
    // Each test gets its own isolated database
    db := sandbox.DB()
    // ... test code
}
```

## Migration Integration

The library includes built-in support for [golang-migrate](https://github.com/golang-migrate/migrate) and provides an interface for custom migration systems.

### Using golang-migrate

```go
import (
    "sql_sandbox"
)

func main() {
    mainDBURL := "postgres://user:pass@localhost:5432/main_db?sslmode=disable"
    
    // Create migration checker using golang-migrate
    migrationChecker := sql_sandbox.NewGolangMigrateChecker("./migrations")
    
    // Create sandbox with migration checker
    sandbox, err := sql_sandbox.NewWithMigrationChecker(mainDBURL, nil, migrationChecker)
    if err != nil {
        log.Fatal(err)
    }
    defer sandbox.Close()
    
    // Your test code here
    db := sandbox.DB()
    // ...
}
```

### Migration File Format

The library expects migration files in the golang-migrate format:

```
migrations/
├── 001_create_users_table.up.sql
├── 001_create_users_table.down.sql
├── 002_add_user_index.up.sql
└── 002_add_user_index.down.sql
```

Example migration file (`001_create_users_table.up.sql`):
```sql
-- +migrate Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

Example down migration (`001_create_users_table.down.sql`):
```sql
-- +migrate Down
DROP TABLE users;
```

### Custom Migration Systems

You can implement your own migration checker by implementing the `MigrationChecker` interface:

```go
type MigrationChecker interface {
    EnsureMigrated(dbURL string) error
}
```

Example with a custom migration system:

```go
type CustomMigrationChecker struct {
    // Your custom fields
}

func (c *CustomMigrationChecker) EnsureMigrated(dbURL string) error {
    // Your custom migration logic
    return nil
}

// Use with sandbox
sandbox, err := sql_sandbox.NewWithMigrationChecker(mainDBURL, nil, &CustomMigrationChecker{})
```

## Best Practices

1. **Always defer cleanup**: Use `defer sandbox.Close()` to ensure test databases are cleaned up
2. **Use unique database names**: The library handles this automatically, but ensure your configuration doesn't conflict
3. **Test in isolation**: Each test should be independent and not rely on data from other tests
4. **Use transactions when needed**: For complex test scenarios, consider using database transactions
5. **Monitor database connections**: Ensure your connection pool settings are appropriate for your test load

## Error Handling

The library provides detailed error messages for common issues:

- Database connection failures
- Template database creation issues
- Test database creation/drop failures
- Migration check failures

## Performance Considerations

- **Template Database**: Creating test databases from a template is much faster than running migrations
- **Connection Pooling**: Configure appropriate connection pool settings for your test environment
- **Parallel Tests**: The library supports parallel test execution but monitor database connection limits

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

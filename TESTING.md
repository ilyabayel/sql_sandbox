# Testing Guide

This document explains how to test the SQL Sandbox library and what has been verified.

## Test Environment

The library is tested using PostgreSQL 17 Alpine running in Docker Compose. The test environment includes:

- **PostgreSQL 17 Alpine**: Latest stable PostgreSQL version
- **Docker Compose**: For easy setup and teardown
- **Test Database**: Isolated test databases for each test
- **Migration Integration**: Tests with golang-migrate

## Running Tests

### Prerequisites

- Docker and Docker Compose installed
- Go 1.23+ installed
- Port 5433 available (or modify docker-compose.yml)

### Quick Test Run (Recommended)

```bash
# Run all tests with automated setup
make test
```

This command will:
1. Start PostgreSQL 17 Alpine container
2. Wait for database to be healthy
3. Create all required test databases
4. Set up database schemas (users table, indexes)
5. Run all library tests with proper environment variables
6. Keep containers running for subsequent test runs

### Available Make Targets

```bash
# Run all tests with automated database setup
make test

# Run tests without database (short mode)
make test-short

# Set up databases only (without running tests)
make setup-db

# Clean up databases and Docker volumes
make clean-db

# Complete cleanup (databases + Go test cache)
make clean

# Show available commands
make help
```

### Automated Setup Process

When you run `make test`, the following happens automatically:

1. **Docker Compose**: Starts PostgreSQL 17 Alpine container
2. **Health Check**: Waits up to 60 seconds for PostgreSQL to be healthy
3. **Database Creation**: Creates required databases:
   - `main_db` (already exists from docker-compose)
   - `main_db_root`, `main_db_basic_usage`, `main_db_with_migrations`
4. **Schema Setup**: Creates `users` table in all databases
5. **Index Creation**: Adds email index for migration tests
6. **Environment Variables**: Sets `POSTGRES_URL` automatically
7. **Test Execution**: Runs all Go tests with proper configuration

### Manual Test Run

If you prefer manual control:

```bash
# Start PostgreSQL and set up databases
make setup-db

# Set environment variable (optional - make test sets this automatically)
export POSTGRES_URL="postgres://testuser:testpass@localhost:5433/main_db?sslmode=disable"

# Run specific test packages
go test -v ./...
go test -v ./examples/basic_usage/...
go test -v ./examples/with_migrations/...

# Clean up when done
make clean-db
```

## Test Coverage

### ✅ Core Library Tests

- **TestSandbox**: Basic sandbox creation and cleanup
- **TestSandboxWithDependencyInjection**: Service integration with dependency injection

### ✅ Example Application Tests

- **TestUserService_CreateUser**: User creation functionality
- **TestUserService_GetUserByID**: User retrieval functionality  
- **TestUserService_ListUsers**: User listing functionality
- **TestUserService_Isolation**: Database isolation between tests
- **TestUserService_Parallel**: Parallel test execution

### ✅ Migration Integration Tests

- **TestSandboxWithMigrations**: Sandbox with golang-migrate integration
- **TestMigrationChecker**: Direct migration checker testing

### ✅ Performance Tests

- **BenchmarkUserService_CreateUser**: Performance benchmarking

## What's Verified

### ✅ Database Isolation

Each test gets its own isolated database:
- Unique database names with timestamps and process IDs
- No data leakage between tests
- Proper cleanup after each test

### ✅ Migration Integration

- golang-migrate integration works correctly
- Migrations are applied to test databases
- Migration files are created and applied properly

### ✅ Dependency Injection

- Database connections are properly injected into services
- Services work correctly with test databases
- No hardcoded database connections

### ✅ Thread Safety

- Parallel test execution works correctly
- No race conditions in database creation/cleanup
- Proper mutex protection for concurrent access

### ✅ Error Handling

- Graceful handling of database connection failures
- Proper cleanup on errors
- Detailed error messages for debugging

### ✅ Performance

- Fast database creation from templates
- Efficient connection pooling
- Minimal overhead for test setup/teardown

## Test Results

All tests pass successfully:

```
✅ Library tests: PASS
✅ Example tests: PASS  
✅ Migration tests: PASS
✅ Integration tests: PASS
✅ Performance tests: PASS
```

## Configuration

### Database Configuration

- **Host**: localhost:5433
- **User**: testuser
- **Password**: testpass
- **SSL**: disabled

### Automated Database Setup

The `setup-test-db.sh` script automatically creates and configures:

- **Main Database**: `main_db` (template source)
- **Test Databases**: 
  - `main_db_root` (for core library tests)
  - `main_db_basic_usage` (for basic usage examples)
  - `main_db_with_migrations` (for migration integration tests)
- **Schema**: `users` table with appropriate indexes
- **Template Database**: `template_test` (created automatically by sandbox)

### Test Configuration

- **Template Database**: template_test
- **Test Database Prefix**: test_db_
- **Max Connections**: 10
- **Connection Timeout**: 30 seconds
- **Environment Variable**: `POSTGRES_URL` (set automatically by Makefile)

## Troubleshooting

### Port Already in Use

If port 5433 is already in use, modify `docker-compose.yml`:

```yaml
ports:
  - "5434:5432"  # Change to available port
```

Then update the connection string in test files.

### Database Connection Issues

1. Ensure Docker is running
2. Check if PostgreSQL container is healthy:
   ```bash
   docker-compose ps
   ```
3. Check container logs:
   ```bash
   docker-compose logs postgres
   ```
4. If setup fails, try cleaning and retrying:
   ```bash
   make clean && make test
   ```
5. For manual debugging, run setup separately:
   ```bash
   make setup-db
   ```

### Migration Issues

1. Ensure golang-migrate is properly imported
2. Check migration file format (up/down files)
3. Verify migration directory exists

## Continuous Integration

The automated test setup is designed for CI/CD pipelines:

- **Automated Setup**: `make test` handles all database setup
- **Docker Integration**: Uses Docker Compose for PostgreSQL
- **Health Checks**: Waits for database to be ready
- **Clean State**: Each run starts with fresh databases
- **Proper Exit Codes**: Returns appropriate codes for CI systems

### Example CI Configurations

**GitHub Actions:**
```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - name: Run tests
        run: make test
```

**GitLab CI:**
```yaml
test:
  image: golang:1.23
  services:
    - docker:dind
  script:
    - make test
```

**Generic CI:**
```bash
# Simple CI script
make test
```

### CI Best Practices

- Use `make clean && make test` for completely fresh state
- The setup is idempotent - safe to run multiple times
- Docker containers are automatically managed
- No manual environment setup required

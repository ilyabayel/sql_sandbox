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

### Quick Test Run

```bash
# Run all tests with PostgreSQL 17
./test.sh
```

This script will:
1. Start PostgreSQL 17 Alpine container
2. Wait for database to be ready
3. Run all library tests
4. Run example tests
5. Run migration integration tests
6. Clean up containers

### Manual Test Run

```bash
# Start PostgreSQL
docker-compose up -d postgres

# Wait for database to be ready
docker-compose exec -T postgres pg_isready -U testuser -d main_db

# Set environment variable
export POSTGRES_URL="postgres://testuser:testpass@localhost:5433/main_db?sslmode=disable"

# Run tests
go test -v ./...
go test -v ./examples/basic_usage/...
go test -v ./examples/with_migrations/...

# Clean up
docker-compose down
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
- **Database**: main_db
- **User**: testuser
- **Password**: testpass
- **SSL**: disabled

### Test Configuration

- **Template Database**: template_test
- **Test Database Prefix**: test_db_
- **Max Connections**: 10
- **Connection Timeout**: 30 seconds

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

### Migration Issues

1. Ensure golang-migrate is properly imported
2. Check migration file format (up/down files)
3. Verify migration directory exists

## Continuous Integration

The test script (`test.sh`) is designed to be run in CI/CD pipelines:

- Starts fresh PostgreSQL instance
- Runs all tests in isolation
- Cleans up automatically
- Returns proper exit codes

Example CI configuration:

```yaml
test:
  script:
    - chmod +x test.sh
    - ./test.sh
  services:
    - docker:dind
```

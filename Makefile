.PHONY: test test-short setup-db clean-db clean ci help

# Default target
.DEFAULT_GOAL := help

# Set up databases and run all tests
test: setup-db
	@echo "Running all tests..."
	POSTGRES_URL="postgres://testuser:testpass@localhost:5433/main_db?sslmode=disable" go test -v ./...

# Run tests in short mode (skips database tests)
test-short:
	@echo "Running tests in short mode (no database)..."
	go test -short -v ./...

# Set up PostgreSQL and test databases
setup-db:
	@echo "Setting up test databases..."
	./setup-test-db.sh

# Clean up PostgreSQL and test databases
clean-db:
	@echo "Cleaning up databases..."
	docker-compose down -v || true

# Clean up test artifacts and databases
clean: clean-db
	@echo "Cleaning up test cache..."
	go clean -testcache

# CI target - set up databases and run tests using existing PostgreSQL service
ci:
	@echo "Setting up databases for CI environment..."
	@./setup-ci-db.sh
	@echo "Running all tests..."
	POSTGRES_URL="postgres://testuser:testpass@127.0.0.1:5433/main_db?sslmode=disable" go test -v ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  test        - Set up databases and run all tests (default)"
	@echo "  test-short  - Run tests without database (short mode)"
	@echo "  setup-db    - Set up PostgreSQL and test databases"
	@echo "  clean-db    - Clean up databases and Docker volumes"
	@echo "  clean       - Clean up databases and test cache"
	@echo "  ci          - Set up databases and run tests (for CI environment)"
	@echo "  help        - Show this help message"

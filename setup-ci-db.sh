#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# PostgreSQL connection details for CI environment
PG_HOST="127.0.0.1"
PG_PORT="5433"
PG_USER="testuser"
PG_PASSWORD="testpass"
PG_ADMIN_DB="postgres"

# Database configuration
DATABASES=(
  "main_db"
  "main_db_root"
  "main_db_basic_usage" 
  "main_db_with_migrations"
)

echo "Setting up test databases for CI environment..."

# Check if psql is available (required for CI environment)
if ! command -v psql &> /dev/null; then
    echo "Error: psql command not found. This script is designed for CI environments with PostgreSQL client installed." >&2
    echo "For local development, use 'make test' instead." >&2
    exit 1
fi

# Wait for PostgreSQL to be ready (in case CI hasn't finished startup)
echo "Waiting for PostgreSQL to be ready..."
MAX_WAIT_SECONDS=30
elapsed=0
until PGPASSWORD="$PG_PASSWORD" psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_ADMIN_DB" -c "SELECT 1;" >/dev/null 2>&1; do
  if (( elapsed >= MAX_WAIT_SECONDS )); then
    echo "Error: PostgreSQL did not become ready within ${MAX_WAIT_SECONDS}s" >&2
    exit 1
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done
echo "PostgreSQL is ready."

# Create databases if they don't exist (skip main_db as it should already exist in CI)
echo "Creating required databases..."
for db in "${DATABASES[@]}"; do
  if [[ "$db" != "main_db" ]]; then
    echo "Creating database: $db"
    PGPASSWORD="$PG_PASSWORD" psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_ADMIN_DB" \
      -c "CREATE DATABASE \"$db\";" 2>/dev/null || {
      echo "Database $db already exists or creation failed, continuing..."
    }
  else
    echo "Using existing database: $db"
  fi
done

# Create users table schema in each database
USERS_TABLE_SQL="
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
"

# Create index for the migrations database
USERS_INDEX_SQL="CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);"

echo "Setting up database schemas..."
for db in "${DATABASES[@]}"; do
  echo "Setting up schema in: $db"
  PGPASSWORD="$PG_PASSWORD" psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$db" \
    -c "$USERS_TABLE_SQL"
  
  # Add index for migrations database
  if [[ "$db" == "main_db_with_migrations" ]]; then
    PGPASSWORD="$PG_PASSWORD" psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$db" \
      -c "$USERS_INDEX_SQL"
  fi
done

echo "CI database setup completed successfully!"
echo "Available databases:"
for db in "${DATABASES[@]}"; do
  echo "  - $db"
done

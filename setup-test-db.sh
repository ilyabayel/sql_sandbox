#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Prefer docker compose v2, fall back to docker-compose
if command -v docker &>/dev/null && docker compose version &>/dev/null; then
  COMPOSE_CMD=(docker compose)
elif command -v docker-compose &>/dev/null; then
  COMPOSE_CMD=(docker-compose)
else
  echo "Error: docker compose/docker-compose not found. Please install Docker Desktop or docker-compose." >&2
  exit 1
fi

# Container name from docker-compose.yml
CONTAINER_NAME="sql_sandbox_postgres"

echo "Starting PostgreSQL (Postgres 17 Alpine)..."
"${COMPOSE_CMD[@]}" up -d postgres

# Wait for health check to pass
MAX_WAIT_SECONDS=60
elapsed=0
echo "Waiting for PostgreSQL to be healthy..."
until [[ $(docker inspect -f '{{.State.Health.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo "starting") == "healthy" ]]; do
  if (( elapsed >= MAX_WAIT_SECONDS )); then
    echo "Error: PostgreSQL did not become healthy within ${MAX_WAIT_SECONDS}s" >&2
    docker logs "$CONTAINER_NAME" || true
    exit 1
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done
echo "PostgreSQL is healthy."

# Database configuration - including the main database that templates are created from
DATABASES=(
  "main_db"
  "main_db_root"
  "main_db_basic_usage" 
  "main_db_with_migrations"
)

# Create databases if they don't exist (skip main_db as it already exists)
echo "Setting up test databases..."
for db in "${DATABASES[@]}"; do
  if [[ "$db" != "main_db" ]]; then
    echo "Creating database: $db"
    docker exec "$CONTAINER_NAME" psql -U testuser -d main_db -c "CREATE DATABASE \"$db\";" 2>/dev/null || {
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
  docker exec "$CONTAINER_NAME" psql -U testuser -d "$db" -c "$USERS_TABLE_SQL"
  
  # Add index for migrations database
  if [[ "$db" == "main_db_with_migrations" ]]; then
    docker exec "$CONTAINER_NAME" psql -U testuser -d "$db" -c "$USERS_INDEX_SQL"
  fi
done

echo "Database setup completed successfully!"
echo "Available databases:"
for db in "${DATABASES[@]}"; do
  echo "  - $db"
done

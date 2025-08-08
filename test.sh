#!/bin/bash

set -e

echo "🚀 Starting PostgreSQL 17 Alpine..."
docker-compose up -d postgres

echo "⏳ Waiting for PostgreSQL to be ready..."
until docker-compose exec -T postgres pg_isready -U testuser -d main_db; do
    echo "Waiting for PostgreSQL..."
    sleep 2
done

echo "✅ PostgreSQL is ready!"

# Set environment variables for tests
export POSTGRES_URL="postgres://testuser:testpass@localhost:5433/main_db?sslmode=disable"

echo "🧪 Running library tests..."
go test -v ./...

echo "🧪 Running example tests..."
cd examples/basic_usage && go test -v ./... && cd ../..

echo "🧪 Running migration example tests..."
cd examples/with_migrations && go test -v ./... && cd ../..

echo "🧪 Running integration tests..."
go test -v -run TestSandbox ./...

echo "✅ All tests completed successfully!"

echo "🧹 Cleaning up..."
docker-compose down

echo "🎉 Test run completed successfully!"

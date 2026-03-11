#!/bin/bash
set -e

echo "=== ATAP Platform Setup ==="

# Check dependencies
command -v docker >/dev/null 2>&1 || { echo "Docker required. Install: https://docs.docker.com/get-docker/"; exit 1; }
command -v go >/dev/null 2>&1 || { echo "Go required. Install: https://go.dev/dl/"; exit 1; }

# Go dependencies
echo "→ Fetching Go dependencies..."
cd platform
go mod tidy
cd ..

# Start services
echo "→ Starting PostgreSQL and Redis..."
docker compose up -d postgres redis
sleep 3

# Run migrations
echo "→ Running database migrations..."
PGPASSWORD=atap psql -h localhost -U atap -d atap -f platform/migrations/001_init.sql

# Build and run
echo "→ Building platform..."
cd platform
go build -o ../bin/atap-platform ./cmd/server
cd ..

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Start the platform:"
echo "  ./bin/atap-platform"
echo ""
echo "Or with Docker:"
echo "  docker compose up"
echo ""
echo "Register your first agent:"
echo "  curl -X POST http://localhost:8080/v1/register -H 'Content-Type: application/json' -d '{\"name\": \"my-agent\"}'"
echo ""

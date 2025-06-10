#!/bin/bash

# Startup script

echo "Starting Quota Manager System..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go first."
    exit 1
fi

# Check if PostgreSQL is installed
if ! command -v psql &> /dev/null; then
    echo "PostgreSQL is not installed. Please install PostgreSQL first."
    exit 1
fi

# Set environment variables
export POSTGRES_HOST=${POSTGRES_HOST:-localhost}
export POSTGRES_PORT=${POSTGRES_PORT:-5432}
export POSTGRES_USER=${POSTGRES_USER:-postgres}
export POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-password}
export POSTGRES_DB=${POSTGRES_DB:-quota_manager}

echo "Environment variables set:"
echo "  POSTGRES_HOST: $POSTGRES_HOST"
echo "  POSTGRES_PORT: $POSTGRES_PORT"
echo "  POSTGRES_USER: $POSTGRES_USER"
echo "  POSTGRES_DB: $POSTGRES_DB"

# Enter project directory
cd "$(dirname "$0")/.."

# Download dependencies
echo "Downloading dependencies..."
go mod tidy

# Initialize database
echo "Initializing database..."
if [ -f "scripts/init_db.sql" ]; then
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -f scripts/init_db.sql
fi

# Generate test data
echo "Generating test data..."
cd scripts
go run generate_data.go
cd ..

# Start AiGateway mock service
echo "Starting AiGateway mock service..."
cd scripts/aigateway-mock
go mod tidy
nohup go run main.go > aigateway.log 2>&1 &
AIGATEWAY_PID=$!
echo "AiGateway mock service started with PID: $AIGATEWAY_PID"
cd ../..

# Start main service
echo "Starting Quota Manager service..."
go run cmd/main.go

# Cleanup
echo "Shutting down services..."
if [ ! -z "$AIGATEWAY_PID" ]; then
    kill $AIGATEWAY_PID
    echo "AiGateway mock service stopped"
fi

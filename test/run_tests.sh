#!/bin/bash

echo "=== Quota Manager Integration Test Startup Script ==="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed, please install Go first"
    exit 1
fi

# Check if PostgreSQL is installed
if ! command -v psql &> /dev/null; then
    echo "PostgreSQL is not installed, please install PostgreSQL first"
    exit 1
fi

# Set environment variables
export POSTGRES_HOST=${POSTGRES_HOST:-localhost}
export POSTGRES_PORT=${POSTGRES_PORT:-5432}
export POSTGRES_USER=${POSTGRES_USER:-postgres}
export POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-password}
export POSTGRES_DB=${POSTGRES_DB:-quota_manager}

echo "Database configuration in use:"
echo "  Host: $POSTGRES_HOST"
echo "  Port: $POSTGRES_PORT"
echo "  User: $POSTGRES_USER"
echo "  Database: $POSTGRES_DB"

# Enter test directory
cd "$(dirname "$0")"

# Check database connection
echo "Checking database connection..."
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT 1;" > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "Database connection failed, please check if the database is running and configured correctly"
    echo "Attempting to initialize database..."

    # Create database (if it doesn't exist)
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -c "CREATE DATABASE $POSTGRES_DB;" 2>/dev/null

    # Initialize database table structure
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -f ../scripts/init_db.sql

    if [ $? -ne 0 ]; then
        echo "Database initialization failed, please check configuration"
        exit 1
    fi
fi

echo "Database connection successful"

# Update dependencies
echo "Updating Go dependencies..."
cd ..
go mod tidy

# Run integration tests
echo "Starting integration tests..."
cd test
go run main.go

echo "Integration tests completed"
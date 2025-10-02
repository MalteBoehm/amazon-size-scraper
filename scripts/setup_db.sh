#!/bin/bash

# Database setup script for Amazon Size Scraper

DB_NAME="${DB_NAME:-amazon_scraper}"
DB_USER="${DB_USER:-postgres}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"

echo "Setting up database: $DB_NAME"

# Create database if it doesn't exist
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -tc "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1 || \
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -c "CREATE DATABASE $DB_NAME"

# Run migrations
echo "Running migrations..."
for migration in migrations/*.up.sql; do
    if [ -f "$migration" ]; then
        echo "Applying $migration"
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$migration"
    fi
done

echo "Database setup complete!"
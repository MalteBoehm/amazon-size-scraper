#!/bin/sh
# Entrypoint script for amazon-scraper

set -e

echo "=========================================="
echo "🚀 Amazon Size Scraper Entry Point"
echo "=========================================="
echo "[ENTRYPOINT] Environment variables:"
echo "DB_HOST=${DB_HOST:-postgres}"
echo "DB_PORT=${DB_PORT:-5432}"
echo "DB_USER=${DB_USER:-postgres}"
echo "DB_PASSWORD=${DB_PASSWORD:-[NOT SET]}"
echo "DB_NAME=${DB_NAME:-amazon_scraper}"
echo "REDIS_ADDR=${REDIS_ADDR:-redis:6379}"
echo "DEBUG_MODE=${DEBUG_MODE:-false}"
echo "BROWSER_HEADLESS=${BROWSER_HEADLESS:-true}"
echo "USE_PROXIES=${USE_PROXIES:-false}"
echo "PROXY_LIST_FILE=${PROXY_LIST_FILE}"

# Determine which service to run based on the script name or SERVICE_TYPE env var
SERVICE_NAME=$(basename "$0")
if [ ! -z "$SERVICE_TYPE" ]; then
    SERVICE_NAME="$SERVICE_TYPE"
fi

echo "=========================================="
echo "[ENTRYPOINT] Service: $SERVICE_NAME"
echo "=========================================="

# Wait for PostgreSQL
echo "[ENTRYPOINT] Waiting for PostgreSQL to be ready..."
until nc -z ${DB_HOST:-postgres} ${DB_PORT:-5432}; do
    echo "[ENTRYPOINT] PostgreSQL is unavailable - sleeping"
    sleep 2
done
echo "[ENTRYPOINT] ✅ PostgreSQL is up"

# Wait for Redis (if Redis is configured)
if [ ! -z "$REDIS_ADDR" ] || [ "$SERVICE_NAME" = "lifecycle-consumer" ] || [ "$SERVICE_NAME" = "redis-job-consumer" ]; then
    # Parse Redis address
    REDIS_HOST="redis"
    REDIS_PORT="6379"
    
    # Use REDIS_HOST from environment if available
    if [ ! -z "$REDIS_HOST" ]; then
        REDIS_HOST="$REDIS_HOST"
    fi
    
    # Use REDIS_PORT from environment if available
    if [ ! -z "$REDIS_PORT" ]; then
        REDIS_PORT="$REDIS_PORT"
    fi
    
    # Parse REDIS_ADDR if available (overrides host/port)
    if [ ! -z "$REDIS_ADDR" ]; then
        REDIS_HOST="${REDIS_ADDR%%:*}"
        REDIS_PORT="${REDIS_ADDR##*:}"
    fi

    echo "[ENTRYPOINT] Waiting for Redis to be ready at $REDIS_HOST:$REDIS_PORT..."
    until nc -z $REDIS_HOST $REDIS_PORT; do
        echo "[ENTRYPOINT] Redis is unavailable - sleeping"
        sleep 2
    done
    echo "[ENTRYPOINT] ✅ Redis is up"
fi

# Give time for any database migrations to complete
echo "[ENTRYPOINT] Waiting for database migrations..."
echo "[ENTRYPOINT] DEBUG: Database name that will be used: ${DB_NAME}"
echo "[ENTRYPOINT] DEBUG: Full database connection info: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
sleep 10

# Normalize proxy list path if a directory was mounted
if [ "${USE_PROXIES}" = "true" ] && [ -n "${PROXY_LIST_FILE}" ] && [ -d "${PROXY_LIST_FILE}" ]; then
    if [ -f "${PROXY_LIST_FILE}/proxy_list" ]; then
        export PROXY_LIST_FILE="${PROXY_LIST_FILE}/proxy_list"
        echo "[ENTRYPOINT] Normalized PROXY_LIST_FILE to ${PROXY_LIST_FILE}"
    else
        FIRST_FILE=$(ls -1 "${PROXY_LIST_FILE}" 2>/dev/null | head -n1)
        if [ -n "$FIRST_FILE" ] && [ -f "${PROXY_LIST_FILE}/${FIRST_FILE}" ]; then
            export PROXY_LIST_FILE="${PROXY_LIST_FILE}/${FIRST_FILE}"
            echo "[ENTRYPOINT] Normalized PROXY_LIST_FILE to ${PROXY_LIST_FILE}"
        else
            echo "[ENTRYPOINT] WARN: proxy directory mounted but no file found; disabling proxies"
            export USE_PROXIES=false
        fi
    fi
fi

# Validate proxy list file or fallback/disable
if [ "${USE_PROXIES}" = "true" ]; then
    if [ -z "${PROXY_LIST_FILE}" ] || [ ! -f "${PROXY_LIST_FILE}" ]; then
        if [ -f "/app/proxy_data/proxy_list" ]; then
            export PROXY_LIST_FILE="/app/proxy_data/proxy_list"
            echo "[ENTRYPOINT] Using fallback PROXY_LIST_FILE=${PROXY_LIST_FILE}"
        elif [ -f "/app/proxy_list" ]; then
            export PROXY_LIST_FILE="/app/proxy_list"
            echo "[ENTRYPOINT] Using fallback PROXY_LIST_FILE=${PROXY_LIST_FILE}"
        else
            echo "[ENTRYPOINT] Proxy list not found; disabling proxies"
            export USE_PROXIES=false
        fi
    fi
fi

# Check which binary to execute
BINARY="./size-scraper"
if [ "$SERVICE_NAME" = "lifecycle-consumer" ]; then
    BINARY="./lifecycle-consumer"
elif [ "$SERVICE_NAME" = "redis-job-consumer" ]; then
    BINARY="./redis-job-consumer"
elif [ "$SERVICE_NAME" = "job-scraper" ]; then
    BINARY="./job-scraper"
elif [ -f "./amazon-scraper" ]; then
    BINARY="./amazon-scraper"
fi

echo "[ENTRYPOINT] Checking if binary exists: $BINARY"
if [ ! -f "$BINARY" ]; then
    echo "[ENTRYPOINT] ❌ ERROR: Binary not found!"
    echo "[ENTRYPOINT] Available files:"
    ls -la
    exit 1
fi

echo "[ENTRYPOINT] ✅ Binary found: $BINARY"
echo "[ENTRYPOINT] Starting service..."
echo "=========================================="

# Execute the binary with all passed arguments
exec "$BINARY" "$@"
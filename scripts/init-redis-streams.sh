#!/bin/bash

# Redis Stream Initialization Script
# This script sets up the Redis streams and consumer groups for the Amazon Size Scraper

echo "🔧 Initializing Redis streams and consumer groups..."

# Redis connection parameters
REDIS_HOST="49.13.49.90"
REDIS_PORT="3020"
REDIS_PASSWORD="pass"

# Stream and group configuration
PRODUCT_STREAM="stream:product_lifecycle"
PRODUCT_CONSUMER_GROUP="lifecycle-consumer-group"
SCRAPER_STREAM="stream:scraper_jobs"
SCRAPER_CONSUMER_GROUP="scraper-consumer-group"

# Test Redis connection
echo "📡 Testing Redis connection..."
if ! redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" ping > /dev/null 2>&1; then
    echo "❌ Failed to connect to Redis at $REDIS_HOST:$REDIS_PORT"
    exit 1
fi

echo "✅ Redis connection successful"

# Function to initialize a stream and consumer group
init_stream_and_group() {
    local stream_name=$1
    local consumer_group=$2
    local description=$3

    echo ""
    echo "📊 Checking stream: $stream_name ($description)"
    STREAM_INFO=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO STREAM "$stream_name" 2>/dev/null)

    if [ $? -ne 0 ]; then
        echo "⚠️  Stream does not exist. Creating initial stream entry..."
        # Create stream with an initial message
        redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XADD "$stream_name" "*" \
            init "true" \
            event_type "SYSTEM_INIT" \
            timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
            message "Redis stream initialized for $description" > /dev/null

        if [ $? -eq 0 ]; then
            echo "✅ Stream created successfully"
        else
            echo "❌ Failed to create stream"
            return 1
        fi
    else
        STREAM_LENGTH=$(echo "$STREAM_INFO" | grep "length" | awk '{print $2}')
        echo "✅ Stream exists with $STREAM_LENGTH entries"
    fi

    # Check if consumer group exists, create if it doesn't
    echo "👥 Checking consumer group: $consumer_group"
    GROUP_INFO=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO GROUPS "$stream_name" 2>/dev/null)

    if [ $? -ne 0 ]; then
        echo "⚠️  Consumer group does not exist. Creating group..."
        # Create consumer group starting from the beginning of the stream
        redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XGROUP CREATE "$stream_name" "$consumer_group" "0" > /dev/null 2>&1

        if [ $? -eq 0 ]; then
            echo "✅ Consumer group created successfully"
        else
            # Try with MKSTREAM option if stream doesn't exist
            redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XGROUP CREATE "$stream_name" "$consumer_group" "0" MKSTREAM > /dev/null 2>&1
            if [ $? -eq 0 ]; then
                echo "✅ Consumer group and stream created successfully"
            else
                echo "❌ Failed to create consumer group"
                return 1
            fi
        fi
    else
        echo "✅ Consumer group exists"
        CONSUMER_COUNT=$(echo "$GROUP_INFO" | grep "consumers" | awk '{print $2}')
        PENDING_COUNT=$(echo "$GROUP_INFO" | grep "pending" | awk '{print $2}')
        echo "   📊 Consumers: $CONSUMER_COUNT, Pending: $PENDING_COUNT"
    fi
}

# Initialize Product Lifecycle Stream
init_stream_and_group "$PRODUCT_STREAM" "$PRODUCT_CONSUMER_GROUP" "Product Lifecycle Events"

# Initialize Scraper Jobs Stream
init_stream_and_group "$SCRAPER_STREAM" "$SCRAPER_CONSUMER_GROUP" "Scraper Jobs"

echo ""
echo "📋 Final Stream Status:"
echo "Product Lifecycle Stream:"
redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO STREAM "$PRODUCT_STREAM" | head -5

echo ""
echo "Scraper Jobs Stream:"
redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO STREAM "$SCRAPER_STREAM" | head -5

echo ""
echo "👥 Final Consumer Group Status:"
echo "Product Lifecycle Consumer Groups:"
redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO GROUPS "$PRODUCT_STREAM"

echo ""
echo "Scraper Jobs Consumer Groups:"
redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO GROUPS "$SCRAPER_STREAM"

echo ""
echo "🎉 Redis stream initialization complete!"
echo "💡 Scraper jobs can now be processed and lifecycle consumers can run"
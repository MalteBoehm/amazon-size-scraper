#!/bin/bash

# Redis Stream Initialization Script
# This script sets up the Redis streams and consumer groups for the Amazon Size Scraper

echo "🔧 Initializing Redis streams and consumer groups..."

# Redis connection parameters
REDIS_HOST="49.13.49.90"
REDIS_PORT="3020"
REDIS_PASSWORD="pass"

# Stream and group configuration
STREAM_NAME="stream:product_lifecycle"
CONSUMER_GROUP="lifecycle-consumer-group"

# Test Redis connection
echo "📡 Testing Redis connection..."
if ! redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" ping > /dev/null 2>&1; then
    echo "❌ Failed to connect to Redis at $REDIS_HOST:$REDIS_PORT"
    exit 1
fi

echo "✅ Redis connection successful"

# Check if stream exists, create if it doesn't
echo "📊 Checking stream: $STREAM_NAME"
STREAM_INFO=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO STREAM "$STREAM_NAME" 2>/dev/null)

if [ $? -ne 0 ]; then
    echo "⚠️  Stream does not exist. Creating initial stream entry..."
    # Create stream with an initial message
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XADD "$STREAM_NAME" "*" \
        init "true" \
        event_type "SYSTEM_INIT" \
        timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        message "Redis stream initialized" > /dev/null

    if [ $? -eq 0 ]; then
        echo "✅ Stream created successfully"
    else
        echo "❌ Failed to create stream"
        exit 1
    fi
else
    STREAM_LENGTH=$(echo "$STREAM_INFO" | grep "length" | awk '{print $2}')
    echo "✅ Stream exists with $STREAM_LENGTH entries"
fi

# Check if consumer group exists, create if it doesn't
echo "👥 Checking consumer group: $CONSUMER_GROUP"
GROUP_INFO=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO GROUPS "$STREAM_NAME" 2>/dev/null)

if [ $? -ne 0 ]; then
    echo "⚠️  Consumer group does not exist. Creating group..."
    # Create consumer group starting from the beginning of the stream
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XGROUP CREATE "$STREAM_NAME" "$CONSUMER_GROUP" "0" > /dev/null 2>&1

    if [ $? -eq 0 ]; then
        echo "✅ Consumer group created successfully"
    else
        # Try with MKSTREAM option if stream doesn't exist
        redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XGROUP CREATE "$STREAM_NAME" "$CONSUMER_GROUP" "0" MKSTREAM > /dev/null 2>&1
        if [ $? -eq 0 ]; then
            echo "✅ Consumer group and stream created successfully"
        else
            echo "❌ Failed to create consumer group"
            exit 1
        fi
    fi
else
    echo "✅ Consumer group exists"
    CONSUMER_COUNT=$(echo "$GROUP_INFO" | grep "consumers" | awk '{print $2}')
    PENDING_COUNT=$(echo "$GROUP_INFO" | grep "pending" | awk '{print $2}')
    echo "   📊 Consumers: $CONSUMER_COUNT, Pending: $PENDING_COUNT"
fi

# Show final stream status
echo ""
echo "📋 Final Stream Status:"
redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO STREAM "$STREAM_NAME" | head -10

echo ""
echo "👥 Final Consumer Group Status:"
redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" XINFO GROUPS "$STREAM_NAME"

echo ""
echo "🎉 Redis stream initialization complete!"
echo "💡 You can now start the lifecycle consumer"
#!/bin/bash

# Ensure we are running from the project root
cd "$(dirname "$0")/.."

# Function to run a test for a specific mode
test_mode() {
    MODE=$1
    echo "=================================================="
    echo "Testing Mode: $MODE"
    echo "=================================================="
    
    # Cleanup previous runs
    pkill -f bin/worker 2>/dev/null
    pkill -f bin/master 2>/dev/null
    sleep 1

    # Start Workers
    echo "Starting 5 Workers..."
    mkdir -p logs
    ./bin/worker 9001 > logs/worker1_$MODE.log 2>&1 &
    ./bin/worker 9002 > logs/worker2_$MODE.log 2>&1 &
    ./bin/worker 9003 > logs/worker3_$MODE.log 2>&1 &
    ./bin/worker 9004 > logs/worker4_$MODE.log 2>&1 &
    ./bin/worker 9005 > logs/worker5_$MODE.log 2>&1 &
    sleep 1

    # Start Master
    echo "Starting Master..."
    ./bin/master -mode=$MODE 9000 localhost:9001 localhost:9002 localhost:9003 localhost:9004 localhost:9005 &
    sleep 1

    # Write Data
    echo "Writing 'key:$MODE' -> 'val:$MODE'..."
    curl -s "http://localhost:8080/put?key=key:$MODE&value=val:$MODE"
    echo ""

    # Read Data
    echo "Reading 'key:$MODE'..."
    RESULT=$(curl -s "http://localhost:8080/get?key=key:$MODE")
    echo "Result: $RESULT"

    if [[ "$RESULT" == "val:$MODE" ]]; then
        echo "SUCCESS for $MODE"
    else
        echo "FAILURE for $MODE"
        exit 1
    fi

    # Cleanup
    pkill -f bin/worker
    pkill -f bin/master
    sleep 1
}

echo "Building..."
go build -o bin/master ./cmd/master
go build -o bin/worker ./cmd/worker

test_mode "sync"
test_mode "async"
test_mode "chain"
test_mode "quorum"

echo "ALL TESTS PASSED"

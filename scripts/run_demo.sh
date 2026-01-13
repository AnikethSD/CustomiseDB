#!/bin/bash

# Ensure we are running from the project root
cd "$(dirname "$0")/.."

MODE=${1:-chain}

cleanup() {
    echo "Stopping servers..."
    pkill -f bin/worker 2>/dev/null
    pkill -f bin/master 2>/dev/null
}
trap cleanup EXIT

echo "Building..."
go build -o bin/master ./cmd/master
go build -o bin/worker ./cmd/worker

echo "Starting 5 Workers..."
mkdir -p logs
./bin/worker 8001 > logs/worker1.log 2>&1 &
./bin/worker 8002 > logs/worker2.log 2>&1 &
./bin/worker 8003 > logs/worker3.log 2>&1 &
./bin/worker 8004 > logs/worker4.log 2>&1 &
./bin/worker 8005 > logs/worker5.log 2>&1 &

sleep 1

echo "Starting Master managing 5 workers (Mode: $MODE)..."
./bin/master -mode=$MODE 8000 localhost:8001 localhost:8002 localhost:8003 localhost:8004 localhost:8005 &

sleep 1

echo -e "\n--- Testing Replication (Mode: $MODE) ---"
echo "1. Writing 'user:100' -> 'Gold' (Will be stored on replicas)"
curl "http://localhost:8080/put?key=user:100&value=Gold"

echo -e "\n2. Reading 'user:100'..."
curl "http://localhost:8080/get?key=user:100"

echo -e "\n3. Killing Worker 8001 (A potential replica)..."
pkill -f "bin/worker 8001"

echo "4. Reading 'user:100' again (Should still work via failover/replication!)"
curl "http://localhost:8080/get?key=user:100"

echo -e "\n\n--- Done. Logs are in logs/*.log. Press Enter to exit ---"
read

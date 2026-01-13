# A customizable distributed key-value store

An educational implementation of a distributed key-value store in Go, demonstrating fundamental distributed systems concepts like Master-Worker architecture, Consistent Hashing, Replication, and RPC.

## 🏗️ Architecture

The system consists of three main components:

1.  **Master Node**: The coordinator. It maintains a **Consistent Hash Ring** to map keys to workers and manages replication strategies. It routes requests to the appropriate Worker node(s).
2.  **Worker Nodes**: Storage nodes that hold a portion of the total dataset in an in-memory map. They also handle forwarding requests for Chain Replication.
3.  **Client**: Sends `Put` and `Get` requests to the Master via HTTP.

### Key Concepts Demonstrated

-   **Consistent Hashing**: Data is distributed using a hash ring with virtual nodes (CRC32), minimizing data movement when nodes are added/removed.
-   **Replication Strategies**:
    -   **Synchronous**: Writes to all replicas before confirming success (Strong Consistency).
    -   **Asynchronous**: Writes to Primary, replicates in background (Low Latency, Eventual Consistency).
    -   **Chain Replication**: Writes flow through a chain of workers (Head -> Next -> Tail). Reads can be done from the Tail for strong consistency.
    -   **Quorum**: Writes/Reads require acknowledgement from a majority `(N/2 + 1)` of replicas (Partition Tolerance).
-   **Sharding (Partitioning)**: Keys are automatically partitioned across available workers.
-   **RPC (Remote Procedure Call)**: Nodes communicate using Go's `net/rpc`.

### ⚖️ CAP Theorem & Trade-offs

The **CAP Theorem** states that a distributed data store can only provide two of the following three guarantees:

1.  **Consistency (C)**: Every read receives the most recent write or an error.
2.  **Availability (A)**: Every request receives a (non-error) response, without the guarantee that it contains the most recent write.
3.  **Partition Tolerance (P)**: The system continues to operate despite an arbitrary number of messages being dropped (or delayed) by the network between nodes.

In a distributed system, **Partition Tolerance (P) is mandatory** because network failures are inevitable. Therefore, you must choose between:

1.  **Stop serving (CP)**: Fail the request to ensure data doesn't diverge (what `sync`, `chain`, and `quorum` do).
2.  **Serve potentially old data (AP)**: Return what you have, even if it's wrong (what `async` effectively does).

This application allows you to toggle this trade-off using the `-mode` flag:

| Mode | Type | Trade-off Description |
| :--- | :--- | :--- |
| **`sync`** | **CP** | **Consistency over Availability.** If a replica is unreachable (partitioned), the write fails to ensure all copies remain identical. |
| **`chain`** | **CP** | **Consistency over Availability.** Similar to Sync, if any link in the chain is broken, the write cannot complete successfully. |
| **`quorum`** | **CP** | **Consistency over Availability.** Requires a majority agreement. If you lose too many nodes (can't form a quorum), the system becomes unavailable for writes. |
| **`async`** | **AP** | **Availability over Consistency.** The system accepts writes even if backups are down. The primary acknowledges immediately, but data might be lost if the primary crashes before replicating. |

## 📂 Project Structure

```text
customise-db/
├── cmd/
│   ├── master/    # Master node with Consistent Hashing & Replication logic
│   ├── worker/    # Worker node with storage & forwarding logic
│   └── client/    # Simple RPC client
├── common/        # Shared RPC argument/reply structures
├── scripts/       # Helper scripts
│   ├── run_demo.sh      # Demo script (default: chain replication)
│   └── test_all_modes.sh # Integration tests for all 4 modes
└── bin/           # Compiled binaries (generated)
```

## 🚀 Getting Started

### Prerequisites
- [Go](https://golang.org/doc/install) (1.18 or later recommended)

### Running the Demo
The easiest way to see the system in action is using the provided bash script. You can specify the replication mode (`sync`, `async`, `chain`, `quorum`).

```bash
# Default mode (Chain Replication)
./scripts/run_demo.sh

# Specific mode
./scripts/run_demo.sh quorum
```

This script will:
1. Build all binaries.
2. Start 5 Workers.
3. Start the Master with the selected replication mode.
4. Perform `Put` and `Get` operations via `curl`.
5. Demonstrate failover by killing a worker node.

### Running Integration Tests
To verify all replication modes in sequence:

```bash
./scripts/test_all_modes.sh
```

### Running Manually

If you want to run components manually in different terminals:

**1. Start Workers**
```bash
mkdir -p logs
go run ./cmd/worker/main.go 8001 > logs/w1.log 2>&1 &
go run ./cmd/worker/main.go 8002 > logs/w2.log 2>&1 &
go run ./cmd/worker/main.go 8003 > logs/w3.log 2>&1 &
```

**2. Start Master**
Use the `-mode` flag to select the strategy (`sync`, `async`, `chain`, `quorum`). Default is `sync`.
```bash
go run ./cmd/master/main.go -mode=chain 8000 localhost:8001 localhost:8002 localhost:8003
```

**3. Client Operations**
The Master exposes an HTTP interface for simple interaction:

-   **Put**: `curl "http://localhost:8080/put?key=foo&value=bar"`
-   **Get**: `curl "http://localhost:8080/get?key=foo"`

## 🖥️ Web Dashboard (New!)

A real-time dashboard is available at **http://localhost:8080** when the Master is running.

### Features
-   **Visual Hash Ring**: See how virtual nodes map to physical workers.
-   **Live Metrics**: Monitor key counts and request rates per node.
-   **CAP Tuning**: Switch Replication Modes (Sync/Async/Chain/Quorum) dynamically and see the **CP vs AP** trade-off.
-   **Auto-Scaling**: Watch new nodes appear on the ring as load increases.

## 🧪 Cleanup
To stop all background processes (master/workers) and clean logs:

```bash
pkill -f "bin/worker"; pkill -f "bin/master"; rm -rf logs/*.log
```

# MizuFlow Hybrid Load Test

This directory contains the necessary scripts to perform a high-concurrency load test on the MizuFlow Server-Sent Events (SSE) implementation.

## Strategy
We use a **Hybrid Approach** because standard CLI tools have limitations:
1. **k6**: Excellent for generating HTTP traffic (Admin updates) and monitoring metrics, but weak at maintaining thousands of persistent SSE connections.
2. **Go (main.go)**: Native Go performance is used to maintain 2000+ lighter-weight SSE connections and measure end-to-end latency with microsecond precision.

## Prerequisites
- **Docker**: The server must be running (`docker-compose up -d`).
- **Go**: local installation (to run the client).
- **k6**: local installation (to trigger updates).

## How to Run

### Step 1: Start the Listeners (The Audience)
Open a terminal and run the Go Load Generator. This simulates thousands of clients connecting to the stream.

```bash
# Default: 2000 concurrent clients
go run loadtest/main.go -c 2000
```
*Wait until you see "Connected Clients: 2000" before proceeding.*

### Step 2: Start the Broadcaster (The Speaker)
Open a **second terminal** and run the k6 script. This simulates the Admin API pushing config changes.

```bash
k6 run loadtest/k6-sse.js
```

## Interpreting Results

**In the Go Terminal:**
Look for the `Latency` logs. This measures the time from "Admin triggers update" (timestamp payload) to "Client receives event".
```text
[E2E Latency] Count: 1950 | Mean: 15ms | P95: 42ms | P99: 110ms
```

**In the k6 Terminal:**
Look for the server health metrics:
- `server_heap_alloc_bytes`: Memory usage.
- `server_goroutines`: Should match (2000 clients + overhead).

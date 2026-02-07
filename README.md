# K8S Live Ticker Pipeline

A real-time price ticker running on Kubernetes with NATS messaging and **Protocol Buffers** for high-speed binary encoding.

## Architecture

```
┌─────────────┐     UDP      ┌─────────────┐   Protobuf    ┌─────────────┐
│  PowerShell │ ──────────►  │ Go Ingester │ ──────────►   │    NATS     │
│   or Bash   │   :5005      │  (Binary)   │   :4222       │   Server    │
└─────────────┘              └─────────────┘               └──────┬──────┘
                                                                  │
                                                           WebSocket :8080
                                                                  │
                                                                  ▼
                             ┌─────────────┐               ┌─────────────┐
                             │   Browser   │ ◄──────────── │   Browser   │
                             │  (UI Page)  │   :8080       │  via nginx  │
                             └─────────────┘               └─────────────┘
```

## Components

| Component | Technology | Purpose |
|-----------|------------|---------|
| **NATS Server** | `nats:latest` | Message broker with WebSocket support |
| **Go Ingester** | Go + Protobuf | Receives UDP, encodes to Protobuf, publishes to NATS |
| **UI (nginx)** | `nginx:alpine` | Serves the HTML ticker page |
| **Browser** | JavaScript | Decodes Protobuf, displays live prices |

## Files

| File | Contents |
|------|----------|
| `main.go` | Go ingester with Protobuf encoding |
| `ticker.proto` | Protocol Buffers schema definition |
| `pb/ticker.pb.go` | Generated Go Protobuf code |
| `configmaps.yaml` | UI HTML code + NATS server config |
| `deployments.yaml` | NATS and ticker-app deployments |
| `services.yaml` | All services (internal + external) |

---

## Platform Support

This project works on **both Windows and Linux**:

| Platform | Kubernetes | Go Ingester | UDP Sender |
|----------|------------|-------------|------------|
| **Windows** | Docker Desktop / Rancher Desktop | Native or WSL | PowerShell |
| **Linux** | k3s / minikube / kind | Native | Bash (netcat) |
| **macOS** | Docker Desktop | Native | Bash (netcat) |

---

## Prerequisites

- Kubernetes cluster (k3s, Docker Desktop, minikube, etc.)
- Go 1.21+ with protoc compiler (for development)
- kubectl configured

---

## Setup

### 1. Deploy to Kubernetes
```bash
kubectl apply -f configmaps.yaml -f deployments.yaml -f services.yaml
```

### 2. Verify pods are running
```bash
kubectl get pods
# Wait for: nats-system (1/1) and ticker-app (1/1)
```

### 3. Generate Protobuf Code (Required for Go Ingester)

The Go ingester uses Protocol Buffers to encode messages. You need to generate the Go code from `ticker.proto`.

#### Step 3a: Install the Protobuf Compiler

**Windows (using WSL/Ubuntu):**
```bash
# Install protoc
sudo apt update && sudo apt install -y protobuf-compiler

# Verify installation
protoc --version
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt update && sudo apt install -y protobuf-compiler
```

**macOS:**
```bash
brew install protobuf
```

#### Step 3b: Install the Go Protobuf Plugin

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

This installs `protoc-gen-go` to `$HOME/go/bin/`. Make sure this is in your PATH:
```bash
export PATH="$PATH:$HOME/go/bin"
```

#### Step 3c: Create the pb Directory

```bash
mkdir -p pb
```

#### Step 3d: Generate the Go Code

**If protoc is in your PATH:**
```bash
protoc --go_out=. --go_opt=paths=source_relative ticker.proto
mv ticker.pb.go pb/
```

**If you need to specify the plugin path explicitly (Windows/WSL):**
```bash
protoc --go_out=. --go_opt=paths=source_relative \
  --plugin=protoc-gen-go=$HOME/go/bin/protoc-gen-go \
  ticker.proto
mv ticker.pb.go pb/
```

#### Step 3e: Update Go Dependencies

```bash
go mod tidy
```

This adds the required `google.golang.org/protobuf` dependency to your `go.mod`.

#### Verify Setup

You should now have:
```
K8S/
├── main.go           # Imports "k8s-ingester/pb"
├── ticker.proto      # Protobuf schema
├── pb/
│   └── ticker.pb.go  # Generated Go code (contains TickerUpdate struct)
├── go.mod            # Should include protobuf dependency
└── go.sum
```

Test the build:
```bash
go build -o ingester .
```

If it compiles without errors, protobuf is set up correctly!

---

## Running (Local Development Mode)

You need **5 terminals** when running the Go ingester locally:

### Terminal 1: Port Forward UI
```bash
kubectl port-forward svc/ticker-ui-service 8080:80
```

### Terminal 2: Port Forward NATS WebSocket
```bash
kubectl port-forward svc/nats-gateway-service 30080:8080
```

### Terminal 3: Port Forward NATS (for local Go ingester)
```bash
kubectl port-forward svc/nats-service 4222:4222
```

### Terminal 4: Run Go Ingester
```bash
go run main.go
```

### Terminal 5: Open Browser
Open http://localhost:8080 in your browser

---

## Sending Price Updates

### Windows (PowerShell) - Single Pair
```powershell
$udp = New-Object System.Net.Sockets.UdpClient

# Single price update
$bytes = [Text.Encoding]::ASCII.GetBytes("BTC-USD,74250.65")
$udp.Send($bytes, $bytes.Length, "127.0.0.1", 5005)
```

### Windows (PowerShell) - Multi-Pair (BTC, ETH, SOL)
```powershell
$udp = New-Object System.Net.Sockets.UdpClient
while($true) { 
    # BTC-USD
    $btc = 74000 + (Get-Random -Maximum 500)
    $bytes = [Text.Encoding]::ASCII.GetBytes("BTC-USD,$btc")
    $udp.Send($bytes, $bytes.Length, "127.0.0.1", 5005)
    
    # ETH-USD
    $eth = 2800 + (Get-Random -Maximum 100)
    $bytes = [Text.Encoding]::ASCII.GetBytes("ETH-USD,$eth")
    $udp.Send($bytes, $bytes.Length, "127.0.0.1", 5005)
    
    # SOL-USD
    $sol = 180 + (Get-Random -Maximum 20)
    $bytes = [Text.Encoding]::ASCII.GetBytes("SOL-USD,$sol")
    $udp.Send($bytes, $bytes.Length, "127.0.0.1", 5005)
    
    Start-Sleep -Milliseconds 500
}
```

### Linux/macOS (Bash) - Multi-Pair
```bash
while true; do
    BTC=$((74000 + RANDOM % 500))
    ETH=$((2800 + RANDOM % 100))
    SOL=$((180 + RANDOM % 20))
    
    echo "BTC-USD,$BTC" | nc -u -w0 127.0.0.1 5005
    echo "ETH-USD,$ETH" | nc -u -w0 127.0.0.1 5005
    echo "SOL-USD,$SOL" | nc -u -w0 127.0.0.1 5005
    
    sleep 0.5
done
```

The prices will appear instantly on the browser with individual cards for each pair!

---

## Message Format

### Input (UDP)
Plain text CSV format:
```
SYMBOL,PRICE
```
Example: `BTC-USD,74250.65`

### Internal (NATS)
Protocol Buffers binary format defined in `ticker.proto`:
```protobuf
message TickerUpdate {
    string symbol = 1;    // e.g., "BTC-USD"
    double price = 2;     // e.g., 74250.65
    uint64 volume = 3;    // Trading volume
    int64 timestamp = 4;  // Unix timestamp (ms)
}
```

---

## Understanding Protobuf (Why It's Faster)

### File Roles

| File | Purpose |
|------|---------|
| `ticker.proto` | **Schema definition** - human-readable description of your message structure |
| `pb/ticker.pb.go` | **Generated code** - Go structs + encoding/decoding functions (auto-generated from .proto) |
| `main.go` | **Your application** - uses the generated code to create and serialize messages |

### Binary vs Text (JSON)

```
JSON (text):     {"symbol":"BTC-USD","price":74250.65,"volume":500,"timestamp":1234567890}
                 → 73 bytes

Protobuf (binary): 0A 07 42 54 43 2D 55 53 44 11 CD CC CC CC CC 1E F2 40 18 F4 03 20 D2 85 D4 F4 04
                 → 27 bytes (~3x smaller)
```

**Why Protobuf is smaller:**
- No field names in the data (just field numbers: 1, 2, 3, 4)
- No quotes, colons, or braces
- Numbers stored as binary (not text)

### Why `proto.Marshal` is Faster

**JSON encoding:**
```go
// Must: quote strings, escape special chars, convert numbers to text
json.Marshal(data)  // → {"price":74250.65}
```

**Protobuf encoding:**
```go
// Just: write tag byte + raw bytes
proto.Marshal(update)  // → 0x11 + 8 bytes (IEEE 754 float)
```

### Performance Comparison

| Operation | JSON | Protobuf | Improvement |
|-----------|------|----------|-------------|
| Serialize 1M messages | ~2.5 sec | ~0.3 sec | **8x faster** |
| Deserialize 1M messages | ~3.0 sec | ~0.4 sec | **7x faster** |
| Message size | 73 bytes | 27 bytes | **63% smaller** |

This matters when sending millions of price updates per second!

---

## Services & Ports

| Service | Type | Port | Purpose |
|---------|------|------|---------|
| `nats-service` | ClusterIP | 4222 | Internal NATS communication |
| `nats-gateway-service` | NodePort | 30080→8080 | Browser WebSocket |
| `ticker-ui-service` | NodePort | 30007→80 | Browser HTTP (UI) |
| `ticker-ingester-service` | NodePort | 30005→5005 | UDP ingestion |

---

## Troubleshooting

### Browser stuck on "Connecting..."
- Ensure `kubectl port-forward svc/nats-gateway-service 30080:8080` is running
- Check NATS logs: `kubectl logs -l app=nats`

### Go ingester "NATS Error: connection refused"
- Ensure `kubectl port-forward svc/nats-service 4222:4222` is running

### Price not updating
- Verify Go ingester shows "Sent: ..." messages
- Check that UDP is being sent to port 5005 (not 30005 when running locally)

---

## Historical Data & Charts

The Go ingester writes price history to local JSON files, enabling the UI to display live-updating charts.

### How It Works
1. Each tick is appended to `data/{symbol}.json`
2. Files are capped at **200 entries** (configurable via `maxHistory` in `main.go`)
3. UI fetches history on page load and renders sparkline charts
4. Live ticks are appended to charts in real-time

### File Structure
```
data/
├── BTC-USD.json   # [{"t": 1234567890, "p": 74250}, ...]
├── ETH-USD.json
└── SOL-USD.json
```

### Configuration
```go
// main.go
maxHistory = 200   // Entries to keep per symbol (adjust as needed)

// index.html (in configmaps.yaml)
const MAX_POINTS = 50;  // Points to display on chart
```

---

## Production Alternatives

For production deployments with millions of data points, consider replacing file-based storage with a time-series database:

| Database | Type | Best For |
|----------|------|----------|
| **InfluxDB** | NoSQL | High write throughput, simple queries |
| **TimescaleDB** | SQL (PostgreSQL) | Complex queries, existing SQL skills |
| **QuestDB** | SQL | Extreme performance, columnar storage |

### Migration Path
1. Add a **db-writer service** that subscribes to NATS and writes to the database
2. Add a **REST API** to query historical data
3. Update UI to fetch from API instead of JSON files

```
UDP → Ingester → NATS ─┬→ Browser (live via WebSocket)
                       └→ DB Writer → TimescaleDB/InfluxDB
                                            ↓
                       Browser ←── REST API ←─┘
```

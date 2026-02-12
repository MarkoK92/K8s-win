# K8S Live Ticker Pipeline

A real-time price ticker running on Kubernetes with NATS messaging and **Protocol Buffers** for high-speed binary encoding.

## Architecture

```
                    ┌───────────────────────────────────────────────────────────┐
                    │                   Kubernetes Cluster                      │
                    │                                                           │
 UDP :5005          │  ┌───────────┐ publish  ┌──────┐                          │
────────────────────►  │ Ingester  │────────► │ NATS │──► WebSocket (live)      │
                    │  │   (Go)    │          └──────┘                          │
                    │  │           │                                            │
                    │  │           │──write──► ┌──────────┐                     │
                    │  └───────────┘           │ InfluxDB │ ← 24h retention     │
                    │                          └────┬─────┘                     │
                    │                               │ query                     │
                    │  ┌───────────┐           ┌────┴─────┐                     │
 HTTP :80           │  │  nginx    │           │ API (Go) │ ← /api/history      │
◄───────────────────│  │  (UI)     │           └──────────┘                     │
                    │  └───────────┘                                            │
                    └───────────────────────────────────────────────────────────┘
```

Browser flow:
1. `GET /api/history/BTC-USD` → API queries InfluxDB → returns last 24h of ticks as JSON
2. WebSocket → NATS → live ticks appended to chart

## Understanding the Services

There are **5 Kubernetes Services**, each with a specific role:

### 1. `ticker-ingester-service` (NodePort :30005 → :5005, UDP)
- **Receives** raw UDP text from outside the cluster (e.g. `BTC-USD,74250.65`)
- The Go ingester parses the text, encodes it with `proto.Marshal()` into binary Protobuf, and publishes to NATS
- Written in Go (`main.go`)

### 2. `nats-service` (ClusterIP :4222, TCP)
- **Internal only** — not accessible from outside the cluster
- The Go ingester connects here to **publish** messages to NATS
- Uses the native NATS TCP protocol (fast, binary)

### 3. `nats-gateway-service` (NodePort :30080 → :8080, WebSocket)
- Exposes the **same NATS server** but on its WebSocket port
- The browser's JavaScript connects here to **subscribe** to live price updates
- Browsers can't use raw TCP, so NATS provides a WebSocket interface on a separate port

### 4. `ticker-ui-service` (NodePort :30007 → :80, HTTP)
- Serves the HTML page via nginx
- The browser loads the page from here, then the page's JavaScript connects **directly to NATS** via `nats-gateway-service` for live data

### 5. `ticker-api-service` (NodePort :30090 → :8090, HTTP)
- **History API** — browser calls `GET /api/history/BTC-USD` on page load
- Queries InfluxDB and returns the last 24h of price data as JSON
- Enables chart backfill before live NATS data starts streaming

### Key Insight: NATS = One Server, Two Ports

| Port | Protocol | Service | Who Connects |
|------|----------|---------|-------------|
| 4222 | TCP (native NATS) | `nats-service` | Go ingester (publisher) |
| 8080 | WebSocket | `nats-gateway-service` | Browser JavaScript (subscriber) |

The browser does **not** get data through the UI service — it connects directly to NATS over WebSocket after loading the HTML page.

## Components

| Component | Technology | Purpose |
|-----------|------------|---------|
| **NATS Server** | `nats:latest` | Message broker with WebSocket support |
| **Go Ingester** | Go + Protobuf + InfluxDB | Receives UDP, encodes to Protobuf, publishes to NATS, writes to InfluxDB |
| **InfluxDB** | `influxdb:2.7-alpine` | Time-series database for price history (24h retention) |
| **Ticker API** | Go | HTTP API querying InfluxDB for historical data |
| **UI (nginx)** | `nginx:alpine` | Serves the HTML ticker page |
| **Browser** | JavaScript + Chart.js | Decodes Protobuf, displays live prices with charts |

## Files

| File | Contents |
|------|----------|
| `main.go` | Go ingester — Protobuf + NATS + InfluxDB writes |
| `api/main.go` | Go API server — queries InfluxDB for history |
| `ticker.proto` | Protocol Buffers schema definition |
| `pb/ticker.pb.go` | Generated Go Protobuf code |
| `configmaps.yaml` | UI HTML code + NATS server config |
| `deployments.yaml` | NATS and ticker-app deployments |
| `services.yaml` | All services (internal + external) |
| `charts/` | Helm charts (nats + ticker-app + influxdb) |
| `argocd/` | Argo CD Application manifests (GitOps) |

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

## Helm Charts

The project includes **two independent Helm charts** under `charts/`, allowing you to deploy NATS and the ticker app separately.

### Install NATS (infrastructure)
```bash
helm install nats charts/nats/
```

### Install Ticker App (application)
```bash
helm install ticker charts/ticker-app/
```

### Uninstall independently
```bash
helm uninstall ticker    # removes ticker, keeps NATS running
helm uninstall nats      # removes NATS
```

### Override values
```bash
# Scale NATS
helm install nats charts/nats/ --set replicas=3

# Change NodePorts
helm install ticker charts/ticker-app/ --set ui.nodePort=31000 --set ingester.udpNodePort=31005

# Point ticker at a different NATS host
helm install ticker charts/ticker-app/ --set natsHost=my-custom-nats
```

### Chart structure
```
charts/
├── nats/            # NATS messaging server (config, deployment, services)
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── ticker-app/      # Ticker UI + ingester + API (config, deployments, services)
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
└── influxdb/        # InfluxDB time-series database (deployment, service, PVC)
    ├── Chart.yaml
    ├── values.yaml
    └── templates/
```

> **Note:** The ticker-app chart's `natsHost` value defaults to `nats-service` (the ClusterIP created by the nats chart). If you change the NATS release name, override this value.

### Migrating from raw manifests to Helm

Helm creates its **own tracked resources**. It will **not** auto-replace existing `kubectl apply` resources. Remove them first:
```bash
# Remove old resources
kubectl delete -f configmaps.yaml -f deployments.yaml -f services.yaml

# Then install via Helm
helm install nats charts/nats/
helm install ticker charts/ticker-app/
```

### Post-Install Output

After `helm install`, Helm shows the NOTES.txt with access instructions:

**NATS chart:**
```
🚀 NATS has been deployed!

Internal access:
  nats-service:4222 (from within the cluster)

WebSocket access (for browsers):
  NodePort 30080 → port 8080

Verify:
  kubectl get pods -l app=nats
  kubectl logs -l app=nats
```

**Ticker-app chart:**
```
🚀 Ticker App has been deployed!

UI access:
  NodePort 30007 → port 80

UDP ingestion:
  NodePort 30005 → port 5005

NATS host: nats-service

Verify:
  kubectl get pods -l app=ticker
  kubectl logs -l app=ticker -c ingester
```

> **Note:** Port-forwarding is still required for local development — see [Running (Local Development Mode)](#running-local-development-mode) above.

---

## Understanding Helm

### Installing Helm

Helm is a one-time install — no server component needed:

| Platform | Command |
|----------|---------|
| **Windows** | `winget install Helm.Helm` or `choco install kubernetes-helm` |
| **Linux** | `curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 \| bash` |
| **macOS** | `brew install helm` |

To scaffold a new chart from scratch: `helm create my-chart` (creates a default template structure).

### File Roles

| File | Purpose | Analogy |
|------|---------|---------|
| `Chart.yaml` | Chart metadata (name, version) | `package.json` |
| `values.yaml` | All configurable parameters | Config file — the **only file you edit** for changes |
| `templates/*.yaml` | K8s manifests with `{{ }}` placeholders | Code templates |
| `_helpers.tpl` | Reusable label/name snippets | Shared utility functions |

### `apiVersion: v2` vs `apps/v1` — Not the Same Thing

| File | `apiVersion` refers to | Meaning |
|------|------------------------|---------|
| `Chart.yaml` | **Helm** chart API | `v2` = Helm 3 format (current) |
| `deployment.yaml` | **Kubernetes** resource API | `apps/v1` = stable K8s Deployment |

These are completely unrelated version numbers from different systems.

### How `values.yaml` Links to Templates

Every `{{ .Values.xxx }}` in a template is replaced with the matching value from `values.yaml`. The dot notation follows the YAML hierarchy:

```yaml
# values.yaml
ingester:
  udpNodePort: 30005    # ← This value...
```

```yaml
# templates/service-ingester.yaml
nodePort: {{ .Values.ingester.udpNodePort }}    # ← ...goes here
#          └── .Values ─► ingester ─► udpNodePort = 30005
```

You can also override values at install time without editing the file:
```bash
helm install ticker charts/ticker-app/ --set ingester.udpNodePort=31005
```

### What `_helpers.tpl` Does

It defines **reusable snippets** so labels stay consistent across all templates. Instead of copy-pasting the same labels into every file:

```yaml
# _helpers.tpl — DEFINE once
{{- define "ticker-app.selectorLabels" -}}
app: ticker
app.kubernetes.io/name: {{ include "ticker-app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
```

```yaml
# deployment.yaml — USE it
matchLabels:
  {{- include "ticker-app.selectorLabels" . | nindent 6 }}

# service-ui.yaml — REUSE the same labels
selector:
  {{- include "ticker-app.selectorLabels" . | nindent 4 }}
```

This guarantees the Deployment and Service selectors always match (if they don't match, the Service can't find the pods). The `| nindent 6` just adds 6 spaces of indentation.

---

## InfluxDB (Time-Series History)

### Why InfluxDB?

Previously, the ingester wrote price history to `data/*.json` files. This had problems:
- **No retention** — files grew forever
- **Pod restarts = data loss** — files stored inside the container
- **Not queryable** — to get "last 1 hour of BTC", you had to load the entire file

InfluxDB solves all of these:
- **Retention policies** — data older than 24h is auto-deleted
- **Persistent storage** — data survives pod restarts via PersistentVolumeClaim (1Gi)
- **Time-range queries** — `range(start: -24h)` is built-in
- **Built-in HTTP API** — no middleware needed

### Data Flow

```
UDP tick → Go Ingester → proto.Marshal → NATS publish (live)
                       → InfluxDB write  (history)

Browser   → GET /api/history/BTC-USD → API → InfluxDB query → JSON response
          → WebSocket → NATS → live updates appended to chart
```

### Data Volume

| Metric | Value |
|--------|-------|
| Ticks/second | ~10 (5 pairs × 2/sec) |
| Ticks/day | ~864,000 |
| Raw size/day | ~86 MB |
| Compressed (InfluxDB) | **~10 MB** |
| PVC size | 1 Gi (enough for weeks) |
| Retention | 24 hours (configurable) |

### Configuration

All settings are in `charts/influxdb/values.yaml`:

```yaml
setup:
  username: admin
  password: tickerpass123
  org: ticker                # Organization name
  bucket: ticks              # Where price data is stored
  retention: 24h             # Auto-delete older data
  token: ticker-secret-token # API token for read/write

storage:
  size: 1Gi                  # PersistentVolumeClaim size
```

> **Note:** The `token`, `org`, and `bucket` values must match in both `charts/influxdb/values.yaml` and `charts/ticker-app/values.yaml` (under the `influxdb:` section). They come pre-configured to match.

### How the Ingester Writes to InfluxDB

In `main.go`, after publishing to NATS, each tick is also written to InfluxDB:

```go
p := influxdb2.NewPoint("ticks",
    map[string]string{"symbol": parts[0]},        // tag: BTC-USD
    map[string]interface{}{"price": price},        // field: 74250.65
    now,                                            // timestamp
)
writeAPI.WritePoint(context.Background(), p)
```

Connection is configured via environment variables: `INFLUXDB_URL`, `INFLUXDB_TOKEN`, `INFLUXDB_ORG`, `INFLUXDB_BUCKET`.
For local development, defaults fall back to `http://localhost:8086`.

---

## Ticker API (History Service)

### What It Does

The Ticker API is a small Go HTTP server that queries InfluxDB and returns price history as JSON. The browser calls it on page load to backfill the chart before NATS live data starts streaming.

### Endpoint

```
GET /api/history/{symbol}
```

**Example:**
```bash
curl http://127.0.0.1:30090/api/history/BTC-USD
```

**Response:**
```json
[
  {"t": 1707700000000, "p": 74250.65},
  {"t": 1707700000500, "p": 74251.10},
  ...
]
```

- `t` = timestamp in milliseconds (Unix epoch)
- `p` = price
- Returns the last 24 hours of data (matching InfluxDB's retention policy)

### How the UI Uses It

In the browser JavaScript:
```javascript
// On page load — fill charts with historical data
const res = await fetch('http://127.0.0.1:30090/api/history/BTC-USD');
const history = await res.json();
for (const pt of history.slice(-50)) {
    updateChart('BTC-USD', pt.p, pt.t);
}

// Then switch to live NATS WebSocket for real-time updates
```

### Port-Forward for Local Development

```bash
kubectl port-forward svc/ticker-api-service 30090:8090
```

---

## When to Use Redis or ClickHouse

We chose InfluxDB alone for this project. Here's when the other databases become worth adding:

### Redis (In-Memory Cache)

**What it does:** Stores recent data in RAM for sub-millisecond reads.

**When to add it:**
- **Thousands of concurrent users** hitting the same `/api/history/BTC-USD` endpoint — Redis serves cached results instantly instead of re-querying InfluxDB every time
- **Real-time leaderboards or aggregations** — e.g. "top 5 movers in the last 5 minutes" computed and cached
- **Session state** — if you add user accounts with watchlists or alerts

**Why not now:**
- InfluxDB handles our query load easily (~5 queries on page load, one per symbol)
- Adding Redis means another Deployment, another Service, cache invalidation logic, and double-writes from the ingester
- At our scale (~10 ticks/sec, 1 user), the complexity isn't justified

**Architecture if added:**
```
Browser → API → Redis (cache hit? → return) → InfluxDB (cache miss? → query, cache, return)
```

### ClickHouse (Analytics Engine)

**What it does:** Columnar database optimized for analytical queries across billions of rows.

**When to add it:**
- **Months or years of historical data** that you need to query fast (InfluxDB retention keeps only 24h)
- **Complex aggregations** — e.g. "average daily closing price for the last 6 months, grouped by week"
- **Multi-dimensional analytics** — e.g. correlating price moves with volume, volatility scoring
- **Hundreds of symbols** instead of 5 — ClickHouse handles massive cardinality better

**Why not now:**
- We only store 24h of data for 5 symbols — InfluxDB handles this with ~10 MB
- ClickHouse requires SQL knowledge and more cluster management
- It doesn't have built-in retention policies like InfluxDB — you manage TTLs yourself
- Overkill for a real-time dashboard; it shines for historical analytics dashboards

### Decision Matrix

| Criteria | InfluxDB ✅ | Redis | ClickHouse |
|----------|-----------|-------|------------|
| **Best for** | Time-series (our case) | Hot cache | Analytics at scale |
| **Query speed** | Fast for recent data | Sub-millisecond | Fast for aggregations |
| **Data volume** | MBs to low GBs | MBs (RAM-limited) | GBs to TBs |
| **Retention** | Built-in (`24h`) | TTL per key | Manual / TTL tables |
| **Complexity** | Low (single binary) | Low (but adds a layer) | Medium-High |
| **Add when** | ✅ Now (default) | 1000+ users | Months of history / analytics |

> **Rule of thumb:** Start with InfluxDB alone. Add Redis when read traffic is the bottleneck. Add ClickHouse when you need long-term analytics that InfluxDB's retention window can't cover.

---

## Argo CD (GitOps)

### Why Helm + Argo CD?

**Helm** handles **packaging** — charts, templates, values. But you still run `helm install` manually.

**Argo CD** handles **delivery** — it watches your Git repo and auto-deploys whenever you push. Together they form a full GitOps pipeline:

```
Developer pushes code
         │
         ▼
   GitHub repo (K8s-win)
         │
         ▼
   Argo CD (watches repo)
         │ detects change in charts/
         ▼
   helm template → kubectl apply
         │
         ▼
   Kubernetes cluster updated automatically
```

You never run `helm install` or `kubectl apply` manually again — just `git push`.

### What is Argo CD?

Argo CD is a **Kubernetes controller** that runs inside your cluster. It:
- Continuously polls your Git repo (every ~3 minutes by default)
- Compares what's **in Git** vs what's **running in the cluster**
- If they differ, it syncs the cluster to match Git (auto-sync)
- Provides a **web UI** and a **CLI tool** to manage everything

Argo CD is **not UI-only** — you can use it three ways:

| Access Method | When to Use |
|---------------|-----------|
| **Web UI** (https://localhost:8443) | Visual dashboard — see app status, sync history, resource tree |
| **CLI** (`argocd` command) | Scripting, CI/CD pipelines, terminal power users |
| **kubectl** | Apply Application manifests directly (what we do in Step 3) |

### Step 1: Install Argo CD into Your Cluster

```bash
kubectl create namespace argocd
```
> **Why:** Argo CD needs its own namespace to keep its controller, server, and repo-server pods separate from your application pods.

```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```
> **Why:** This downloads and deploys the official Argo CD manifests directly from GitHub. It installs ~7 components (API server, repo server, controller, Redis, etc.) into the `argocd` namespace. The `-n argocd` flag ensures everything goes into that namespace.

```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=argocd-server -n argocd --timeout=120s
```
> **Why:** The Argo CD server pod takes a few seconds to start. This command blocks until the API server pod is ready, so you don't try to log in before it's available. The `--timeout=120s` gives it up to 2 minutes.

Verify all pods are running:
```bash
kubectl get pods -n argocd
# You should see ~7 pods, all Running
```

### Step 2: Access Argo CD

#### Option A: Web UI (recommended for beginners)

```bash
kubectl port-forward svc/argocd-server -n argocd 8443:443
```
> **Why:** The Argo CD server runs on HTTPS (port 443) inside the cluster. Port-forwarding maps your local port 8443 to it. We use 8443 to avoid conflicts with other services.

Open **https://localhost:8443** in your browser (accept the self-signed certificate warning).

**Login credentials:**
- **Username:** `admin`
- **Password:** Auto-generated during install. Retrieve it:

Linux/macOS/WSL:
```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```
> **Why:** Argo CD stores the initial admin password as a base64-encoded Kubernetes Secret. This command reads the secret and decodes it.

Windows (PowerShell):
```powershell
[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String((kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}")))
```

#### Option B: CLI (`argocd` command)

Install the CLI:

| Platform | Command |
|----------|---------|
| **Windows** | `winget install Argo.ArgoCD.CLI` or `choco install argocd-cli` |
| **Linux** | `curl -sSL -o argocd https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64 && chmod +x argocd && sudo mv argocd /usr/local/bin/` |
| **macOS** | `brew install argocd` |

Login (while port-forward is running):
```bash
argocd login localhost:8443 --username admin --password <YOUR_PASSWORD> --insecure
```
> **Why:** `--insecure` skips TLS certificate verification (since we're using a self-signed cert). In production you'd use a real certificate.

Useful CLI commands:
```bash
argocd app list                        # List all applications
argocd app get nats                    # Show NATS app status
argocd app sync nats                   # Force sync right now (don't wait for poll)
argocd app history ticker-app          # Show deployment history
argocd app rollback ticker-app 1       # Rollback to revision 1
```

### Step 3: Register Your Applications

The `argocd/` folder contains Application manifests that tell Argo CD **what to watch and deploy**:

```bash
kubectl apply -f argocd/nats-app.yaml
kubectl apply -f argocd/ticker-app.yaml
```
> **Why:** An Argo CD `Application` is a custom Kubernetes resource (CRD). When you `kubectl apply` it, Argo CD's controller picks it up and starts watching the specified Git repo path. We use `kubectl apply` (not the `argocd` CLI) because the manifests are already in our repo — this is the GitOps way.

Each Application manifest tells Argo CD:
- **Where to look**: `https://github.com/MarkoK92/K8s-win.git` → `charts/nats/` or `charts/ticker-app/`
- **What to do**: Render the Helm chart using `values.yaml`
- **When to deploy**: Automatically on every `git push` (auto-sync enabled)
- **Self-heal**: If someone manually changes a resource in the cluster, Argo CD reverts it to match Git

### Understanding the Application Manifest

Here's what each field in `argocd/nats-app.yaml` does:

```yaml
apiVersion: argoproj.io/v1alpha1        # Argo CD custom resource API
kind: Application                        # Type of resource
metadata:
  name: nats                            # Name shown in Argo CD UI
  namespace: argocd                     # Must be in the argocd namespace
spec:
  project: default                      # Argo CD project (default = unrestricted)
  source:
    repoURL: https://github.com/MarkoK92/K8s-win.git   # Git repo to watch
    targetRevision: main                # Branch to track
    path: charts/nats                   # Folder containing the Helm chart
    helm:
      valueFiles:
        - values.yaml                   # Which values file to use
  destination:
    server: https://kubernetes.default.svc   # Deploy to THIS cluster
    namespace: default                       # Into this namespace
  syncPolicy:
    automated:
      prune: true                       # Delete resources that were removed from Git
      selfHeal: true                    # If someone runs kubectl edit, revert it
    syncOptions:
      - CreateNamespace=true            # Create the namespace if it doesn't exist
```

### Step 4: The GitOps Workflow

Once set up, your workflow becomes:

1. Edit `charts/ticker-app/values.yaml` (e.g., change `replicas: 2`)
2. `git add . && git commit -m "scale ticker" && git push`
3. Argo CD detects the change within ~3 minutes (or click **Sync** in the UI / run `argocd app sync ticker-app`)
4. Kubernetes cluster is updated automatically

### Argo CD Files

| File | Purpose |
|------|---------|
| `argocd/nats-app.yaml` | Argo CD Application — watches `charts/nats/` in Git |
| `argocd/ticker-app.yaml` | Argo CD Application — watches `charts/ticker-app/` in Git |

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

### Argo CD sync fails: "field is immutable" or selector mismatch

This happens when you have **existing Helm releases** and then add Argo CD. Both try to manage the same resources but with different labels:

- `helm install ticker` creates resources with `app.kubernetes.io/instance: ticker`
- Argo CD Application named `ticker-app` tries to set `app.kubernetes.io/instance: ticker-app`
- Kubernetes rejects this because **Deployment selectors are immutable** — you can't change them on an existing Deployment

**Fix — remove old Helm releases first:**
```bash
helm uninstall ticker
helm uninstall nats
```

If Argo CD still shows the cached error after uninstalling, delete and re-apply the Application:
```bash
kubectl delete application ticker-app -n argocd
kubectl apply -f argocd/ticker-app.yaml
```

> **Key lesson:** You cannot have both manual `helm install` and Argo CD managing the same resources. Pick one. Once Argo CD is set up, it is the sole owner — deploy everything via `git push`.

### Argo CD: "app path does not exist"

This means the `charts/` folder hasn't been pushed to GitHub yet. Argo CD looks at your **remote repo**, not your local files:
```bash
git add .
git commit -m "Add Helm charts and Argo CD"
git push
```

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

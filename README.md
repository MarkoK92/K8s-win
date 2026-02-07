# K8S Live Ticker Pipeline

A real-time price ticker running on Kubernetes with NATS messaging.

## Architecture

```
┌─────────────┐     UDP      ┌─────────────┐     NATS      ┌─────────────┐
│  PowerShell │ ──────────►  │  Ingester   │ ──────────►   │    NATS     │
│   (Client)  │   :30005     │  (Node.js)  │   :4222       │   Server    │
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

| Component | Image | Purpose |
|-----------|-------|---------|
| **NATS Server** | `nats:latest` | Message broker with WebSocket support |
| **UI (nginx)** | `nginx:alpine` | Serves the HTML ticker page |
| **Ingester** | `node:alpine` | Receives UDP, publishes to NATS |

## Files

| File | Contents |
|------|----------|
| `configmaps.yaml` | UI HTML code + NATS server config |
| `deployments.yaml` | NATS and ticker-app deployments |
| `services.yaml` | All 4 services (internal + external) |

---

## Setup

### 1. Deploy to Kubernetes
```powershell
kubectl apply -f configmaps.yaml -f deployments.yaml -f services.yaml
```

### 2. Verify pods are running
```powershell
kubectl get pods
# Wait for: nats-system (1/1) and ticker-app (2/2)
```

---

## Running (3 Terminals Required)

### Terminal 1: Port Forward UI
```powershell
kubectl port-forward svc/ticker-ui-service 8080:80
```

### Terminal 2: Port Forward NATS WebSocket
```powershell
kubectl port-forward svc/nats-gateway-service 30080:8080
```

### Terminal 3: Open Browser
Open http://localhost:8080 in your browser

---

## Sending Price Updates

```powershell
# Create UDP client
$udp = New-Object System.Net.Sockets.UdpClient
$udp.Connect("127.0.0.1", 30005)

# Send a price (any string)
$bytes = [System.Text.Encoding]::ASCII.GetBytes("74250.65")
$udp.Send($bytes, $bytes.Length)

# Close when done
$udp.Close()
```

The price will appear instantly on the browser!

---

## Services & Ports

| Service | Type | Port | Purpose |
|---------|------|------|---------|
| `nats-service` | ClusterIP | 4222 | Internal NATS communication |
| `nats-gateway-service` | NodePort | 30080→8080 | Browser WebSocket |
| `ticker-ui-service` | NodePort | 30007→80 | Browser HTTP (UI) |
| `ticker-ingester-service` | NodePort | 30005→5005 | UDP ingestion |

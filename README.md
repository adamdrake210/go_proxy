# Go Proxy

A simple HTTP/HTTPS proxy for capturing and inspecting network requests on your Mac.

## Features

- **HTTP Proxy**: Full request/response capture with headers and body
- **HTTPS Tunneling**: See CONNECT requests (metadata only, not encrypted content)
- **REST API**: Query captured requests via JSON API
- **Real-time Streaming**: Server-Sent Events (SSE) for live request updates
- **In-memory Storage**: Configurable request history size

## Quick Start

```bash
# Build the proxy
go build -o proxy ./cmd/proxy

# Run with default settings (proxy on :8080, API on :8081)
./proxy

# Or run directly
go run ./cmd/proxy
```

## Configuration

```bash
# Custom ports
./proxy -proxy :9090 -api :9091

# Increase stored request limit
./proxy -max-requests 5000

# Show all options
./proxy -help
```

## Configure Your Mac to Use the Proxy

### System-wide (System Preferences)

1. Open **System Preferences** → **Network**
2. Select your active network (Wi-Fi or Ethernet)
3. Click **Advanced** → **Proxies**
4. Enable **Web Proxy (HTTP)** and **Secure Web Proxy (HTTPS)**
5. Set both to `127.0.0.1` port `8080`
6. Click **OK** and **Apply**

### Browser Only (Firefox)

1. Open **Settings** → **Network Settings**
2. Select **Manual proxy configuration**
3. Set HTTP Proxy and HTTPS Proxy to `127.0.0.1` port `8080`

### Command Line (curl)

```bash
# HTTP request through proxy
curl -x http://localhost:8080 http://example.com

# HTTPS request through proxy (tunnel only)
curl -x http://localhost:8080 https://example.com
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/requests` | GET | Get all captured requests |
| `/api/requests?limit=N` | GET | Get last N requests |
| `/api/requests/{id}` | GET | Get specific request by ID |
| `/api/requests/stream` | GET | SSE stream of new requests |
| `/api/clear` | POST/DELETE | Clear all stored requests |
| `/api/stats` | GET | Get request statistics |
| `/health` | GET | Health check |

## Examples

### Get Recent Requests
```bash
curl http://localhost:8081/api/requests?limit=10
```

### Get Request Statistics
```bash
curl http://localhost:8081/api/stats
```

### Stream Requests in Real-time
```bash
curl http://localhost:8081/api/requests/stream
```

### Clear Request History
```bash
curl -X POST http://localhost:8081/api/clear
```

## Project Structure

```
go_proxy/
├── cmd/
│   └── proxy/
│       └── main.go          # Entry point
├── internal/
│   ├── proxy/
│   │   ├── proxy.go         # Main proxy server
│   │   ├── handler.go       # HTTP request handling
│   │   └── https.go         # CONNECT/tunneling
│   ├── capture/
│   │   ├── request.go       # Request/Response models
│   │   └── store.go         # In-memory storage
│   └── api/
│       └── server.go        # REST API server
├── go.mod
└── README.md
```

## Request Data Model

Each captured request contains:

```json
{
  "id": "uuid",
  "timestamp": "2024-01-15T10:30:00Z",
  "method": "GET",
  "url": "http://example.com/api/data",
  "host": "example.com",
  "path": "/api/data",
  "request_headers": {"User-Agent": ["curl/8.0"]},
  "request_body": null,
  "status_code": 200,
  "response_headers": {"Content-Type": ["application/json"]},
  "response_body": "...",
  "duration_ms": 150,
  "is_https": false,
  "is_tunnel": false
}
```

## Phase 2: TLS Interception (Future)

The current implementation uses HTTPS tunneling, which means you can see:
- Which hosts are being connected to
- Connection timing

To see the actual content of HTTPS requests, TLS interception would need to be added, requiring:
1. Generate a CA certificate
2. Install the CA certificate on your Mac
3. Dynamically generate certificates for each host

This is planned for a future phase.

## License

MIT

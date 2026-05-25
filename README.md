# P2P Zero-Trust Security Mesh

> **A self-healing, decentralized network where every node is its own SOC analyst.**

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://golang.org)
[![Python](https://img.shields.io/badge/Python-3.11-3776AB?logo=python)](https://python.org)
[![gRPC](https://img.shields.io/badge/gRPC-1.64-00897B?logo=grpc)](https://grpc.io)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker)](https://docker.com)

---

## The Problem It Solves

In a traditional enterprise network, one firewall breach gives an attacker **free lateral movement**. Every other machine trusts its neighbours by default.

This project eliminates that assumption. Every node **constantly watches itself and its neighbours**. When a node detects a compromise — via CPU spikes, honeypot triggers, or suspicious traffic — it uses a local LLM to score the threat and then **cryptographically broadcasts a quarantine vote** to the mesh. Peers verify the signature and sever connections within seconds, **without any central authority**.

---

## Architecture

```
                    ┌───────────────┐
                    │  Coordinator  │  ← HTTP REST (port 8090)
                    │  (Phonebook)  │    Only introduces nodes.
                    └──────┬────────┘    Steps back immediately.
                           │ register
              ┌────────────┼────────────┐
              │            │            │
         ┌────▼────┐  ┌────▼────┐  ┌────▼────┐
         │ Node A  │  │ Node B  │  │ Node C  │
         │  Go+AI  │◄─┤  Go+AI  ├─►│  Go+AI  │
         └─────────┘  └─────────┘  └─────────┘
              │            │            │
              └────────────┴────────────┘
                     Direct P2P gRPC (mTLS)
```

Each node runs two processes (Sidecar Architecture):

| Process | Language | Responsibility |
|---------|----------|----------------|
| **Peer Node** | Go | Networking, honeypot, telemetry, gossip, cryptography |
| **AI Sidecar** | Python | LLM queries, threat scoring, response validation |

---

## The Threat Lifecycle (Step-by-Step)

```
                                          ┌─────────────────────┐
 Attacker ──► port 2222 ──► Honeypot ──►  │ TelemetryEvent      │
                                          │ (core.EventCh)      │
                                          └────────┬────────────┘
                                                   │ gRPC
                                          ┌────────▼────────────┐
                                          │  Python AI Sidecar  │
                                          │  (Ollama LLM)       │
                                          │  threat_score: 8    │
                                          └────────┬────────────┘
                                                   │ > 7 threshold
                                          ┌────────▼────────────┐
                                          │  Sign QuarantineVote│
                                          │  (Ed25519)          │
                                          └────────┬────────────┘
                                                   │ broadcast
                                    ┌──────────────┴──────────────┐
                               ┌────▼────┐                   ┌────▼────┐
                               │ Node A  │  verify signature │ Node C  │
                               │ sever ✓ │                   │ sever ✓ │
                               └─────────┘                   └─────────┘
```

1. **Infiltration** — Attacker connects to the honeypot on port 2222 (mock SSH).
2. **Telemetry** — Go layer detects the unauthorized attempt and queues a `TelemetryEvent`.
3. **Inference** — The Go sidecar bridge forwards it to the Python AI via gRPC.
4. **Analysis** — Python queries Ollama (Llama 3) with a strict JSON-only system prompt.
5. **Gossip** — If `threat_score > 7`, Node B signs a `QuarantineVote` with its Ed25519 private key and broadcasts it.
6. **Consensus** — Nodes A and C verify the cryptographic signature before accepting the vote.
7. **Self-Healing** — On majority consensus, the mesh severs all TCP connections to Node B. A and C stay connected.

---

## Technology Stack

| Component | Technology | Why |
|-----------|-----------|-----|
| **P2P Networking** | Go (Goroutines) | Thousands of concurrent connections, <1ms goroutine spawn |
| **AI Inference** | Python + Ollama | Native LLM ecosystem, easy prompt engineering |
| **Bridge Protocol** | gRPC | Type-safe, millisecond IPC between Go and Python |
| **Transport Security** | mTLS (mutual TLS) | Both client and server verify identity — rogue nodes rejected |
| **Identity / Signing** | Ed25519 | 64-byte signatures, fast verification, immune to timing attacks |
| **Rate Limiting** | Token-Bucket (x/time/rate) | Prevents P2P packet-flood memory exhaustion |
| **Anti-Injection** | Regex sanitization | Strips injection keywords, control chars before LLM input |
| **Containerization** | Docker Compose | One-command reproducible 3-node cluster |

---

## Security Hardening

### mTLS (Mutual TLS)
All P2P gRPC connections require a certificate signed by the project's local CA. A rogue container that doesn't present a valid cert is **immediately rejected** at the handshake layer — no application code ever sees its packets.

### Token-Bucket Rate Limiting
Each peer's incoming gRPC port runs a `golang.org/x/time/rate` limiter. 10 requests/second per source IP. Packet floods are dropped before deserialization, preventing memory exhaustion.

### Ed25519 Signature Verification
Every `QuarantineVote` is signed with the originating node's Ed25519 private key. Receiving nodes verify the signature before tallying the vote. A forged quarantine vote — even one that looks legitimate — will be **rejected**.

### Prompt Injection Sanitization
The Python sidecar strips injection keywords (`ignore`, `disregard`, `system prompt`, etc.), control characters, and backticks/braces before embedding telemetry into the LLM prompt. Attacker-controlled input in a packet's source IP cannot manipulate the LLM's output.

### Honeypot Architecture
Fake SSH (2222) and HTTP (8888) listeners accept connections but **serve no data** and immediately close. Any connection is treated as a critical intrusion — the ports have no legitimate use.

---

## Project Structure

```
p2p-sec-mesh/
├── coordinator/            # HTTP REST peer registry (Go)
│   ├── main.go             # Registry + HTTP handlers
│   └── Dockerfile
│
├── peer-node/              # P2P node (Go)
│   ├── main.go             # Entry point — wires all subsystems
│   ├── node_keys.go        # Ed25519 key generation
│   ├── core/
│   │   ├── network.go      # Coordinator registration, mTLS gRPC, heartbeat
│   │   ├── gossip.go       # Vote signing, signature verification, quarantine
│   │   └── sidecar.go      # Go→Python gRPC bridge, threat score routing
│   ├── telemetry/
│   │   ├── monitor.go      # CPU/RAM monitoring (gopsutil)
│   │   └── honeypot.go     # Fake SSH/HTTP listeners
│   └── Dockerfile
│
├── analyzer/               # AI Sidecar (Python)
│   ├── app.py              # gRPC server (port 50052)
│   ├── llm_client.py       # Ollama integration + sanitization + Pydantic validation
│   ├── stubs/              # Auto-generated gRPC Python stubs
│   ├── requirements.txt
│   └── Dockerfile
│
├── protos/
│   └── sidecar.proto       # Shared message definitions
│
├── gen/go/                 # Auto-generated Go gRPC stubs
│
├── certs/                  # mTLS certificates (CA + per-service)
├── generate_certs.sh       # mTLS CA + certificate generation script
├── docker-compose.yml      # Full cluster orchestration
├── go.mod                  # Go module
└── go.work                 # Go workspace
```

---

## Quick Start

### Prerequisites
- Docker Desktop (with Compose v2)
- At least 8 GB RAM (Ollama needs ~4 GB for Llama 3)

### 1. Clone and generate certificates

```bash
git clone https://github.com/BadBoy0170/p2p-sec-mesh
cd p2p-sec-mesh
chmod +x generate_certs.sh && ./generate_certs.sh
```

### 2. Start the full cluster

```bash
docker compose up --build
```

This starts:
- `coordinator` — peer phonebook
- `ollama` — local LLM engine  
- `node-a-go`, `node-b-go`, `node-c-go` — Go peer nodes
- `node-a-ai`, `node-b-ai`, `node-c-ai` — Python AI sidecars

### 3. Pull the LLM model (first run only)

```bash
docker exec ollama ollama pull llama3
```

### 4. Trigger an attack simulation

In a separate terminal, connect to Node B's honeypot:

```bash
# Simulate an SSH brute-force attempt
nc localhost 2223
```

Watch the logs:
```
node-b-go  | [HONEYPOT]   INTRUSION DETECTED on mock-SSH port 2222 from 172.20.0.1
node-b-ai  | [SIDECAR]    Received telemetry from node: <node-b-id>
node-b-ai  | [SIDECAR]    Threat score for node-b: 9/10
node-b-go  | [SIDECAR]    CRITICAL THREAT (9.0) — initiating self-quarantine broadcast
node-b-go  | [GOSSIP]     Broadcasting quarantine vote for <id> to 2 peer(s)
node-a-go  | [GOSSIP]     Vote 1 for quarantine of <id> (from <id>)
node-a-go  | [GOSSIP]     QUARANTINING NODE <id> — severing all connections
node-c-go  | [GOSSIP]     QUARANTINING NODE <id> — severing all connections
```

Node B is isolated. Nodes A and C remain connected to each other. 

### 5. Verify the network healed

```bash
# Node A can still reach Node C
curl http://localhost:8090/peers
```

---

## Running Locally (Without Docker)

### Go Peer Node

```bash
# Terminal 1 — Start coordinator
cd coordinator && go run .

# Terminal 2 — Start Node A
export NODE_ID=node-a
export NODE_HOSTNAME=localhost
export P2P_PORT=9001
export COORDINATOR_URL=http://localhost:8090
export SIDECAR_ADDR=localhost:50052
cd peer-node && go run .

# Terminal 3 — Node B on port 9002
export P2P_PORT=9002
export SIDECAR_ADDR=localhost:50053
cd peer-node && go run .
```

### Python AI Sidecar

```bash
cd analyzer
python -m venv venv && source venv/bin/activate
pip install -r requirements.txt

# Regenerate stubs if needed:
python -m grpc_tools.protoc -I../protos \
  --python_out=./stubs \
  --grpc_python_out=./stubs \
  ../protos/sidecar.proto

# Start sidecar
SIDECAR_PORT=50052 OLLAMA_URL=http://localhost:11434 python app.py
```

---

## Configuration Reference

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `COORDINATOR_URL` | `http://localhost:8090` | Coordinator REST endpoint |
| `COORDINATOR_PORT` | `8090` | Coordinator listen port |
| `P2P_PORT` | `9000` | Node's P2P gRPC listen port |
| `NODE_HOSTNAME` | `localhost` | Hostname advertised to coordinator |
| `NODE_ID` | — | Friendly name (for logging) |
| `SIDECAR_ADDR` | `localhost:50052` | Python AI sidecar gRPC address |
| `SIDECAR_PORT` | `50052` | AI sidecar listen port |
| `CERT_FILE` | `certs/peer.pem` | mTLS certificate |
| `KEY_FILE` | `certs/peer.key` | mTLS private key |
| `CA_FILE` | `certs/ca.pem` | Certificate Authority cert |
| `HONEYPOT_SSH_PORT` | `2222` | Mock SSH honeypot port |
| `HONEYPOT_HTTP_PORT` | `8888` | Mock HTTP honeypot port |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama API endpoint |
| `OLLAMA_MODEL` | `llama3` | LLM model to use for analysis |

---

## Design Decisions & Trade-offs

### Why HTTP for the Coordinator instead of gRPC?
The coordinator is a **temporary phonebook**, not a data plane. Using plain HTTP keeps it zero-dependency (no shared proto files), easier to curl/debug, and eliminates mTLS cert management overhead for a non-critical component.

### Why a single shared `peer.pem` certificate?
This is a **demo setup**. In production, each node would get a unique certificate with its own SAN. The current setup demonstrates mTLS mechanics without requiring a full PKI.

### Why Ed25519 over RSA/ECDSA?
Ed25519 keys are smaller (32 bytes vs 256 bytes RSA), signing is ~3x faster than ECDSA-256, and it's immune to implementation-level timing attacks due to its constant-time design.

### Why majority consensus instead of unanimous vote?
A quarantined node that has already lost connectivity can't vote. Requiring unanimity would deadlock the quarantine process. Strict majority (>50%) ensures liveness even when some peers are unreachable.

---

## Security Caveats

> This is a **portfolio/research project**. Before deploying in production:

- Replace the shared `peer.pem` with per-node certificates (unique SANs).
- Add certificate rotation via a proper PKI (e.g., Vault, cert-manager).
- Use authenticated (HMAC or signed) coordinator responses to prevent MITM peer injection.
- Add reputation scoring to weight votes from long-established peers higher.
- Implement vote replay prevention (nonce or monotonic counter).

---

## Author


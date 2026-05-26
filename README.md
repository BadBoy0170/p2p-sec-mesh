# P2P Zero-Trust Security Mesh

> **A self-healing, decentralized network where every node is its own SOC analyst.**

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)](https://golang.org)
[![Python](https://img.shields.io/badge/Python-3.11-3776AB?logo=python)](https://python.org)
[![gRPC](https://img.shields.io/badge/gRPC-1.64-00897B?logo=grpc)](https://grpc.io)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker)](https://docker.com)

---

## The Problem It Solves

In a traditional enterprise network, one firewall breach gives an attacker **free lateral movement**. Every other machine trusts its neighbours by default.

This project eliminates that assumption. Every node **constantly watches itself and its neighbours**. When a node detects a compromise — via CPU spikes, honeypot triggers, or suspicious traffic — it uses a local LLM to score the threat, then **cryptographically broadcasts a quarantine vote** to the mesh. Peers verify the signature and sever connections within seconds, **without any central authority**.

---

## Architecture

```
                    ┌───────────────┐
                    │  Coordinator  │  ← HTTP REST + Web Dashboard (port 8090)
                    │  (Phonebook)  │    Introduces nodes, serves topology UI.
                    └──────┬────────┘    Steps back from security decisions.
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
| **AI Sidecar** | Python | LLM queries via Ollama, threat scoring, Pydantic response validation |

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
                                          │  threat_score: 8    │  ← fallback: rule-based
                                          └────────┬────────────┘    if Ollama unavailable
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
                                                   │
                                          ┌────────▼────────────┐
                                          │  /api/report        │  ← visible in dashboard
                                          │  AI Events feed     │
                                          └─────────────────────┘
```

1. **Infiltration** — Attacker connects to the honeypot on port 2222 (mock SSH).
2. **Telemetry** — Go layer detects the unauthorized attempt and queues a `TelemetryEvent`.
3. **Inference** — The Go sidecar bridge forwards it to the Python AI via gRPC.
4. **Analysis** — Python queries Ollama (`dolphin-llama3`) with a strict JSON-only system prompt. If Ollama is unavailable, a deterministic rule-based fallback scores `HONEYPOT_TRIPPED` at 9/10 automatically.
5. **Gossip** — If `threat_score > 7`, the compromised node signs a `QuarantineVote` with its Ed25519 private key and broadcasts it to all peers.
6. **Consensus** — Peers verify the cryptographic signature before tallying the vote. Self-votes trigger immediate quarantine; external votes require a strict majority (> 50%).
7. **Self-Healing** — On consensus, the mesh severs all TCP connections to the quarantined node. Healthy nodes remain connected to each other.
8. **Reporting** — The decision (score, method `ai`/`rule-based`, decision `quarantine`/`monitor`) is pushed to the coordinator's `/api/report` endpoint and shown live in the web dashboard.

---

## Technology Stack

| Component | Technology | Why |
|-----------|-----------|-----|
| **P2P Networking** | Go (Goroutines) | Thousands of concurrent connections, <1ms goroutine spawn |
| **AI Inference** | Python + Ollama | Native LLM ecosystem, easy prompt engineering |
| **Bridge Protocol** | gRPC (protobuf) | Type-safe, millisecond IPC between Go and Python |
| **Transport Security** | mTLS (mutual TLS) | Both client and server verify identity — rogue nodes rejected at handshake |
| **Identity / Signing** | Ed25519 | 64-byte signatures, fast verification, immune to timing attacks |
| **Rate Limiting** | Token-Bucket (`x/time/rate`) | Prevents P2P packet-flood memory exhaustion |
| **Anti-Injection** | Regex sanitization + truncation | Strips injection keywords, control chars before LLM input |
| **Containerization** | Docker Compose | One-command reproducible 5-node cluster |
| **Web Dashboard** | vis.js + Vanilla JS | Live topology graph with animated glow on anomaly/quarantined nodes |

---

## Security Hardening

### mTLS (Mutual TLS)
All P2P gRPC connections require a certificate signed by the project's local CA. A rogue container that doesn't present a valid cert is **immediately rejected** at the handshake layer — no application code ever sees its packets.

### Token-Bucket Rate Limiting
Each peer's incoming gRPC port runs a `golang.org/x/time/rate` limiter: 10 requests/second per source IP. Packet floods are dropped before deserialization, preventing memory exhaustion.

### Ed25519 Signature Verification
Every `QuarantineVote` is signed with the originating node's Ed25519 private key. Receiving nodes verify the signature before tallying the vote. A forged quarantine vote — even one that looks legitimate — will be **rejected**.

### Prompt Injection Sanitization
The Python sidecar strips injection keywords (`ignore`, `disregard`, `system prompt`, etc.), control characters, and backticks/braces before embedding telemetry into the LLM prompt. Attacker-controlled input in a packet's source IP cannot manipulate the LLM's output.

### Honeypot Architecture
Fake SSH (2222) and HTTP (8888) listeners accept connections but **serve no data** and immediately close. Any connection is treated as a critical intrusion — the ports have no legitimate use.

### Rule-Based Fallback
If the AI sidecar or Ollama is unreachable, `HONEYPOT_TRIPPED` events still trigger quarantine via deterministic rules (score = 9/10). The system never silently ignores an intrusion.

---

## Project Structure

```
p2p-sec-mesh/
├── coordinator/            # HTTP REST peer registry + web dashboard (Go)
│   ├── main.go             # Registry, /api/topology, /api/report, HTTP handlers
│   ├── index.html          # Web dashboard (topology graph + AI events feed)
│   ├── styles.css          # Minimal system-font dark UI
│   ├── js/
│   │   ├── app.js          # Dashboard controller (polls topology + report API)
│   │   ├── NetworkGraph.js # vis.js graph with animated pulse for anomaly nodes
│   │   └── ApiService.js   # Fetch wrapper
│   └── Dockerfile
│
├── peer-node/              # P2P node (Go)
│   ├── main.go             # Entry point — wires all subsystems
│   ├── node_keys.go        # Ed25519 key generation
│   ├── core/
│   │   ├── network.go      # Coordinator registration, mTLS gRPC, heartbeat
│   │   ├── gossip.go       # Vote signing, signature verification, quarantine
│   │   └── sidecar.go      # Go→Python gRPC bridge, AI + rule-based fallback, report push
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
├── generate_certs.sh       # mTLS CA + certificate generation script
├── docker-compose.yml      # 5-node cluster orchestration
├── run_self_healing_cluster.sh  # 30-node local bash demo (no Docker)
├── go.mod                  # Go module
└── go.work                 # Go workspace
```

---

## Quick Start (Docker — Recommended)

### Prerequisites
- Docker Desktop with Compose v2
- `make` (pre-installed on macOS/Linux)
- ~6 GB RAM free (Ollama needs ~4 GB for `dolphin-llama3`)

### One command — that's it

```bash
git clone https://github.com/BadBoy0170/p2p-sec-mesh
cd p2p-sec-mesh
make up
```

> mTLS certificates are generated **inside the Docker build** — no manual steps needed. Nothing sensitive is committed to git.

This starts:
| Container | Role |
|-----------|------|
| `coordinator` | Peer phonebook + web dashboard |
| `ollama` | Local LLM engine |
| `ollama-init` | One-shot model pull (`dolphin-llama3`), then exits |
| `node-{a..e}-go` | 5 Go peer nodes |
| `node-{a..e}-ai` | 5 Python AI sidecars |

### Open the dashboard

```
http://localhost:8090
```

The topology graph auto-updates every 5 seconds. The **AI Events** panel in the sidebar shows every threat detection in real time.

### Trigger an attack

```bash
# Attack a single node
make attack NODE=a

# Attack all 5 nodes simultaneously (stress-tests consensus voting)
make attack-all
```

### Watch the logs

```bash
make logs              # all containers
make logs-node NODE=a  # just node-a
```

Expected sequence:
```
node-a-go  | [HONEYPOT]  INTRUSION DETECTED on mock-SSH port 2222 from 172.20.0.1
node-a-ai  | [LLM]       Model returned threat_score=9
node-a-go  | [SIDECAR]   CRITICAL THREAT (9.0) — initiating self-quarantine broadcast
node-a-go  | [GOSSIP]    Broadcasting quarantine vote for <id> to 4 peer(s)
node-b-go  | [GOSSIP]    Vote 1 for quarantine of <id> (from <id>)
node-b-go  | [GOSSIP]    QUARANTINING NODE <id> — severing all connections
```

Node-a is isolated. Nodes B–E remain connected. Dashboard shows node-a glowing red.

### Check mesh status

```bash
make topology   # current graph JSON
make report     # AI threat event log
make status     # container health
```

### Tear down

```bash
make down        # stop containers
make clean       # also remove images + volumes
```

### All Makefile targets

| Target | Description |
|--------|-------------|
| `make up` | Build + start everything |
| `make down` | Stop all containers |
| `make restart` | Rebuild and restart |
| `make logs` | Stream all logs |
| `make logs-node NODE=b` | Logs for a specific node |
| `make attack NODE=a` | Trigger honeypot on node-a |
| `make attack-all` | Attack all 5 nodes simultaneously |
| `make topology` | Print current mesh topology JSON |
| `make report` | Print AI threat event log |
| `make status` | Show container health |
| `make clean` | Remove containers, images, volumes |

---

## Quick Start (Local — No Docker)

### Prerequisites
- Go 1.22+
- Python 3.11+
- [Ollama](https://ollama.com) running locally with `dolphin-llama3` pulled

```bash
ollama pull dolphin-llama3
```

### Run the 30-node simulation

```bash
chmod +x run_self_healing_cluster.sh
./run_self_healing_cluster.sh
```

Open `http://localhost:8090`. The script boots 30 nodes, attacks node-14, waits for quarantine, then spins up a replacement replica automatically.

### Manual setup

```bash
# Terminal 1 — Coordinator
go run ./coordinator

# Terminal 2 — AI Sidecar
cd analyzer && python -m venv venv && source venv/bin/activate
pip install -r requirements.txt
SIDECAR_PORT=50051 OLLAMA_URL=http://localhost:11434 python app.py

# Terminal 3+ — Peer nodes (repeat with different ports)
NODE_ID=node-a P2P_PORT=9001 LOCAL_MODE=true \
  HONEYPOT_SSH_PORT=2222 SIDECAR_ADDR=localhost:50051 \
  go run ./peer-node
```

---

## Configuration Reference

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `COORDINATOR_URL` | `http://localhost:8090` | Coordinator REST endpoint |
| `COORDINATOR_PORT` | `8090` | Coordinator listen port |
| `P2P_PORT` | `9000` | Node's P2P gRPC listen port |
| `NODE_HOSTNAME` | `localhost` | Hostname advertised to peers |
| `NODE_ID` | — | Friendly name (for logging + dashboard) |
| `SIDECAR_ADDR` | `localhost:50052` | Python AI sidecar gRPC address |
| `SIDECAR_PORT` | `50052` | AI sidecar listen port |
| `LOCAL_MODE` | `false` | Skip mTLS (for local dev without certs) |
| `CERT_FILE` | `certs/peer.pem` | mTLS certificate |
| `KEY_FILE` | `certs/peer.key` | mTLS private key |
| `CA_FILE` | `certs/ca.pem` | Certificate Authority cert |
| `HONEYPOT_SSH_PORT` | `2222` | Mock SSH honeypot port |
| `HONEYPOT_HTTP_PORT` | `8888` | Mock HTTP honeypot port |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama API endpoint |
| `OLLAMA_MODEL` | `dolphin-llama3` | LLM model for threat analysis |

---

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web dashboard |
| `/healthz` | GET | Health check (`ok`) |
| `/register` | POST | Node self-registration |
| `/peers` | GET | All active peers |
| `/api/telemetry` | POST | Node status push (status, CPU, RAM, peer list) |
| `/api/topology` | GET | Current graph state for the dashboard |
| `/api/report` | GET | AI/rule-based event log (JSON) |
| `/api/report/submit` | POST | Nodes push threat decisions here |

---

## Design Decisions & Trade-offs

### Why HTTP for the Coordinator?
The coordinator is a **temporary phonebook**, not a data plane. Plain HTTP keeps it zero-dependency, easy to `curl`/debug, and eliminates mTLS cert management overhead for a non-critical component.

### Why a single shared `peer.pem`?
This is a **demo/research project**. In production, each node gets a unique certificate with its own SAN. The current setup demonstrates mTLS mechanics without requiring a full PKI.

### Why Ed25519 over RSA/ECDSA?
Keys are smaller (32 bytes vs 256 bytes RSA), signing is ~3× faster than ECDSA-256, and it's immune to timing attacks due to constant-time design.

### Why majority consensus instead of unanimous vote?
A quarantined node that has already lost connectivity can't vote. Requiring unanimity would deadlock. Strict majority (> 50%) ensures liveness even when some peers are unreachable.

### Why a rule-based fallback?
LLMs can be unavailable (cold start, network issue). `HONEYPOT_TRIPPED` has zero false positive rate — there is no legitimate reason to connect to a honeypot port. The fallback ensures the mesh always responds, even without AI.

---

## Security Caveats

> This is a **portfolio/research project**. Before deploying in production:

- Replace the shared `peer.pem` with per-node certificates (unique SANs per container).
- Add certificate rotation via a proper PKI (e.g., Vault, cert-manager).
- Use signed/authenticated coordinator responses to prevent MITM peer injection.
- Add reputation scoring to weight votes from long-established peers higher.
- Implement vote replay prevention (nonce or monotonic timestamp counter).
- Move Ollama behind an auth proxy if it's network-accessible.

---



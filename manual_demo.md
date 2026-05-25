# 🏦 The Zero-Trust Bank: Manual Interview Demo Guide

If you are presenting this in an interview, running a single automated script (`run_showcase.sh`) hides all the complex engineering happening under the hood. 

Follow this guide to manually boot up the network step-by-step in separate terminal tabs. This allows you to explain exactly what each component does to the interviewer as you turn them on.

---

## 🛠️ Step 1: Start the Coordinator & Web Dashboard
The Coordinator is a lightweight discovery service (like a DNS seed node) and serves the Web UI. It does **not** route traffic (preserving the P2P architecture).

**Open Terminal 1:**
```bash
cd /Users/badboy17/p2p-sec-mesh
# Build the binaries first
go build -o /tmp/peer-node ./peer-node
go build -o /tmp/coordinator-bin ./coordinator

# Run the Coordinator
COORDINATOR_PORT=8090 /tmp/coordinator-bin
```
> 👉 *At this point, tell the interviewer to open `http://localhost:8090`. The dashboard will be empty.*

---

## 🧠 Step 2: Start the AI Sidecar (Ollama)
Instead of starting 5 sidecars, for a manual demo, we will start **one** AI sidecar that all our local nodes will share. This sidecar connects to your local Llama3 model.

**Open Terminal 2:**
```bash
cd /Users/badboy17/p2p-sec-mesh/analyzer
source venv/bin/activate

# Start the Python AI Sidecar on port 50051
SIDECAR_PORT=50051 OLLAMA_URL=http://localhost:11434 OLLAMA_MODEL=dolphin-llama3 python3 app.py
```
> 👉 *Explain that in a real Kubernetes environment, this Python process runs as a sidecar container inside the same Pod as the Go microservice.*

---

## 🌐 Step 3: Booting the Microservices (The P2P Mesh)
Now we will bring the banking microservices online one by one. Watch the Web UI as you run these!

**Open Terminal 3 (API Gateway):**
```bash
cd /Users/badboy17/p2p-sec-mesh

NODE_ID="api-gateway" \
NODE_HOSTNAME="localhost" \
P2P_PORT=9001 \
LOCAL_MODE=true \
HONEYPOT_SSH_PORT=2221 \
COORDINATOR_URL="http://localhost:8090" \
SIDECAR_ADDR="localhost:50051" \
/tmp/peer-node
```

**Open Terminal 4 (Transaction Engine):**
```bash
cd /Users/badboy17/p2p-sec-mesh

NODE_ID="transaction-engine" \
NODE_HOSTNAME="localhost" \
P2P_PORT=9002 \
LOCAL_MODE=true \
HONEYPOT_SSH_PORT=2222 \
COORDINATOR_URL="http://localhost:8090" \
SIDECAR_ADDR="localhost:50051" \
/tmp/peer-node
```

**Open Terminal 5 (Notification Service - The Compromised Node):**
```bash
cd /Users/badboy17/p2p-sec-mesh

NODE_ID="notification-service" \
NODE_HOSTNAME="localhost" \
P2P_PORT=9003 \
LOCAL_MODE=true \
HONEYPOT_SSH_PORT=2223 \
COORDINATOR_URL="http://localhost:8090" \
SIDECAR_ADDR="localhost:50051" \
/tmp/peer-node
```

> 👉 *Explain to the interviewer: "You can see on the dashboard that the nodes have discovered each other via the Coordinator, but they have now established direct, peer-to-peer mTLS connections. No traffic goes through a central proxy."*

---

## 💥 Step 4: The Attack & AI Quarantine
Explain the scenario: The `notification-service` has been compromised via a supply chain attack (like SolarWinds). The attacker is now inside the perimeter and starts port-scanning the internal network to find the `transaction-engine`. 

They accidentally hit the Honeypot port (2222) on the `transaction-engine`.

**Open Terminal 6 (The Hacker):**
```bash
# Simulate an internal network scan hitting the Transaction Engine's honeypot
echo 'SCAN' | nc -w 1 localhost 2222
```

### What to show the interviewer:
1. **The Terminal:** Switch to Terminal 4 (Transaction Engine). You will see the Go node catch the scan, send the TCP payload to the AI Sidecar, and the AI sidecar will reply with a high threat score.
2. **The Dashboard:** The `transaction-engine` will broadcast a Gossip Protocol message alerting the network. You will see the `notification-service` turn **RED (Quarantined)** on the web UI, and its connections to the other nodes will be mathematically severed!

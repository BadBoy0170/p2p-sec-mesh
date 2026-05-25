#!/bin/bash
# -----------------------------------------------------------------------------
# 5-Node Zero-Trust Security Mesh Showcase
# "The Banking Outbreak"
# -----------------------------------------------------------------------------

# Cleanup any old runs
echo "🧹 Cleaning up old processes..."
pkill -f "peer-node" 2>/dev/null
pkill -f "app.py" 2>/dev/null
pkill -f "coordinator" 2>/dev/null
sleep 2

echo "🔨 Building latest Go binary..."
cd /Users/badboy17/p2p-sec-mesh
go build -o /tmp/peer-node ./peer-node

# Create logs directory
mkdir -p /tmp/mesh-logs
rm -f /tmp/mesh-logs/*.log

echo "🚀 Starting Coordinator (The Phonebook)..."
COORDINATOR_PORT=8090 go run ./coordinator > /tmp/mesh-logs/coordinator.log 2>&1 &
sleep 2

echo "🧠 Starting 5 AI Sidecars (Connecting to Ollama)..."
cd /Users/badboy17/p2p-sec-mesh/analyzer
source venv/bin/activate

for i in {1..5}; do
  SIDECAR_PORT=$((50050 + i))
  SIDECAR_PORT=$SIDECAR_PORT OLLAMA_URL=http://localhost:11434 OLLAMA_MODEL=dolphin-llama3 python3 app.py > "/tmp/mesh-logs/sidecar_${i}.log" 2>&1 &
done
sleep 3

echo "🌐 Starting 5 P2P Microservice Nodes..."
cd /Users/badboy17/p2p-sec-mesh

NAMES=("api-gateway" "auth-service" "transaction-engine" "profile-service" "notification-service")

for i in {1..5}; do
  NODE_NAME=${NAMES[$i-1]}
  P2P_PORT=$((9000 + i))
  SSH_PORT=$((2220 + i))
  HTTP_PORT=$((8880 + i))
  SIDECAR_PORT=$((50050 + i))
  
  NODE_ID=$NODE_NAME \
  NODE_HOSTNAME=localhost \
  P2P_PORT=$P2P_PORT \
  LOCAL_MODE=true \
  HONEYPOT_SSH_PORT=$SSH_PORT \
  HONEYPOT_HTTP_PORT=$HTTP_PORT \
  COORDINATOR_URL=http://localhost:8090 \
  SIDECAR_ADDR=localhost:$SIDECAR_PORT \
  /tmp/peer-node > "/tmp/mesh-logs/node_${i}_${NODE_NAME}.log" 2>&1 &
done

echo ""
echo "==========================================================================="
echo "THE 5-NODE BANKING NETWORK IS LIVE!"
echo "==========================================================================="
echo "Node 1: api-gateway          (Honeypot: 2221)"
echo "Node 2: auth-service         (Honeypot: 2222)"
echo "Node 3: transaction-engine   (Honeypot: 2223)"
echo "Node 4: profile-service      (Honeypot: 2224)"
echo "Node 5: notification-service (Honeypot: 2225)  <-- THE COMPROMISED NODE"
echo "==========================================================================="
echo ""
echo "THE SETUP:"
echo "The 'notification-service' (Node 5) has been silently compromised via a"
echo "supply chain attack. The hacker is now using it to scan the internal network,"
echo "looking for the 'transaction-engine' (Node 3)."
echo ""
echo "THE ACTION:"
echo "In a new terminal, execute the hacker's port scan against the Profile Service (Node 4)."
echo "Run this command:"
echo ""
echo "    echo 'SCAN' | nc -w 1 localhost 2224"
echo ""
echo "Then, immediately watch the logs of Node 4 and Node 1 to see the AI quarantine kick in:"
echo "    tail -f /tmp/mesh-logs/node_4_profile-service.log"
echo "    tail -f /tmp/mesh-logs/node_1_api-gateway.log"
echo ""
echo "To shut everything down when you are done, just run:"
echo "    pkill -f peer-node && pkill -f app.py && pkill -f coordinator"
echo "==========================================================================="

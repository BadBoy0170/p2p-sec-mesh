#!/bin/bash
# -----------------------------------------------------------------------------
# 30-Node Zero-Trust Security Mesh: The Self-Healing Cluster
# -----------------------------------------------------------------------------

echo "🧹 Cleaning up old processes..."
pkill -f "peer-node" 2>/dev/null
pkill -f "app.py" 2>/dev/null
pkill -f "coordinator" 2>/dev/null
sleep 2

echo "🔨 Building latest Go binaries..."
cd /Users/badboy17/p2p-sec-mesh
go build -o /tmp/peer-node ./peer-node
go build -o /tmp/coordinator-bin ./coordinator

mkdir -p /tmp/mesh-logs
rm -f /tmp/mesh-logs/*.log

echo "🚀 Starting Coordinator and Web Dashboard..."
COORDINATOR_PORT=8090 /tmp/coordinator-bin > /tmp/mesh-logs/coordinator.log 2>&1 &
sleep 2

echo "🧠 Starting AI Sidecar..."
cd /Users/badboy17/p2p-sec-mesh/analyzer
source venv/bin/activate
SIDECAR_PORT=50051 OLLAMA_URL=http://localhost:11434 OLLAMA_MODEL=dolphin-llama3 python3 app.py > "/tmp/mesh-logs/sidecar.log" 2>&1 &
sleep 3

echo "🌐 Booting Massive 30-Node P2P Cluster..."
cd /Users/badboy17/p2p-sec-mesh

for i in {1..30}; do
  NODE_ID="node-worker-${i}"
  P2P_PORT=$((9000 + i))
  SSH_PORT=$((2220 + i))
  
  # Start Node
  NODE_ID=$NODE_ID \
  NODE_HOSTNAME=localhost \
  P2P_PORT=$P2P_PORT \
  LOCAL_MODE=true \
  HONEYPOT_SSH_PORT=$SSH_PORT \
  COORDINATOR_URL=http://localhost:8090 \
  SIDECAR_ADDR=localhost:50051 \
  /tmp/peer-node > "/tmp/mesh-logs/${NODE_ID}.log" 2>&1 &
  
  # Stagger boot to not overwhelm CPU
  sleep 0.2
done

echo ""
echo "==========================================================================="
echo "🌍 30-NODE CLUSTER IS LIVE"
echo "Open your browser to: http://localhost:8090"
echo "==========================================================================="
echo ""
echo "⏳ Waiting 15 seconds for the network to stabilize..."
sleep 15

# Pick a target (e.g. node 14)
TARGET_NODE="node-worker-14"
TARGET_PORT=$((2220 + 14))

echo "💥 INITIATING SUPPLY CHAIN ATTACK ON ${TARGET_NODE}..."
echo "Hacker is scanning the internal network..."
echo 'SCAN' | nc -w 1 localhost $TARGET_PORT

echo ""
echo "🤖 Auto-Healing Orchestrator is now actively monitoring the network..."
echo "Press Ctrl+C to stop the simulation."

# Auto-healing loop
while true; do
  # Use python to fetch topology and find any quarantined nodes
  QUARANTINED_NODE=$(python3 -c "
import urllib.request, json, sys
try:
    req = urllib.request.urlopen('http://localhost:8090/api/topology')
    data = json.loads(req.read())
    for n in data.get('nodes', []):
        if n.get('status') == 'quarantined' and 'REPLICA' not in n.get('node_id'):
            print(n.get('node_id'))
            sys.exit(0)
except:
    pass
print('')
")

  if [ -n "$QUARANTINED_NODE" ]; then
    echo "🚨 ORCHESTRATOR ALERT: Detected compromised and quarantined node -> $QUARANTINED_NODE"
    
    # 1. Kill the infected process
    echo "🔪 Assassinating infected process ($QUARANTINED_NODE)..."
    pkill -f "NODE_ID=$QUARANTINED_NODE"
    
    sleep 3
    
    # 2. Spin up a replacement!
    REPLICA_ID="${QUARANTINED_NODE}-REPLICA"
    echo "🌱 Spinning up pristine replacement: $REPLICA_ID..."
    
    # Assign a random high port so it doesn't conflict
    NEW_P2P=$((10000 + $RANDOM % 1000))
    NEW_SSH=$((20000 + $RANDOM % 1000))

    NODE_ID=$REPLICA_ID \
    NODE_HOSTNAME=localhost \
    P2P_PORT=$NEW_P2P \
    LOCAL_MODE=true \
    HONEYPOT_SSH_PORT=$NEW_SSH \
    COORDINATOR_URL=http://localhost:8090 \
    SIDECAR_ADDR=localhost:50051 \
    /tmp/peer-node > "/tmp/mesh-logs/${REPLICA_ID}.log" 2>&1 &
    
    echo "✅ Healing complete. Watch the dashboard to see $REPLICA_ID wire into the mesh."
    
    # Break so we don't infinitely spawn replicas (for demo purposes)
    break
  fi

  sleep 2
done

# Keep script running so user can observe
wait

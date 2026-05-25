# The Banking Outbreak Showcase 🚀

This document explains the 5-node showcase, the story behind it, and the exact steps to execute it perfectly in front of an audience.

---

## 🎭 The Story (The Pitch)

Imagine this mesh is running the backend of a FinTech or Banking app. The application is broken down into 5 interconnected microservices (Nodes):

1. **Node 1: API Gateway** *(The front door for the mobile app)*
2. **Node 2: Auth Service** *(Handles user logins)*
3. **Node 3: Transaction Engine** *(The "Crown Jewels" — processes the actual money)*
4. **Node 4: Profile Service** *(Handles user addresses & settings)*
5. **Node 5: Notification Service** *(Sends SMS and Email alerts)*

### The Breach
An attacker finds a vulnerability in a third-party email library used by the **Notification Service (Node 5)**. They successfully hack Node 5. 

However, the attacker doesn't care about sending emails. They want the money. To get it, they need to pivot from the Notification Service into the **Transaction Engine (Node 3)**.

### The Reconnaissance
The attacker starts silently scanning the internal network from Node 5, looking for open ports on the other internal servers to find their way to the Transaction Engine.

### The Trap
While scanning the internal IP addresses, the attacker's automated scan accidentally touches the honeypot port on the **Profile Service (Node 4)**. Node 4's internal monitor triggers: `🚨 INTRUSION DETECTED`.

### The AI Evaluation
Node 4 sends the telemetry to its local AI. A traditional firewall would normally ignore internal traffic (because it trusts internal servers). But our AI analyzes it and realizes: *"Why is the Notification Service trying to SSH into the Profile Service? That makes no sense."* 

The AI scores it `9.0/10 — CRITICAL`. Node 4 mathematically signs a "Quarantine Vote" against Node 5 and broadcasts it to the mesh.

### The Network Heals Itself
The vote hits the network. The API Gateway, Auth Service, and Transaction Engine receive the vote from Node 4. They instantly verify the cryptography. Because they trust the signature, they **all immediately sever their network connections to Node 5**.

The attacker is trapped inside the useless Notification Service. The core banking functions continue operating perfectly without human intervention. The network survived by sacrificing a compromised limb.

---

## 💻 How to Execute the Demo

We have created an automated script that spins up this exact 5-node environment on your machine.

### Step 1: Start the Environment
Open a terminal and run the launch script:
```bash
cd /Users/badboy17/p2p-sec-mesh
./run_showcase.sh
```
*Wait about 10-15 seconds for all 5 Go nodes and 5 Python AIs to connect.*

### Step 2: Open Your Logs (The Dashboards)
Open two new terminal windows so the audience can see the network reacting.

**Terminal A (The Trap - Node 4):**
```bash
tail -f /tmp/mesh-logs/node_4_profile-service.log
```

**Terminal B (The Rest of the Network - Node 1):**
```bash
tail -f /tmp/mesh-logs/node_1_api-gateway.log
```

### Step 3: Launch the Hacker's Attack
In a completely separate terminal (representing the hacker on Node 5), run the port scan against Node 4's honeypot port (2224):
```bash
echo 'SCAN' | nc -w 1 localhost 2224
```

### Step 4: The Climax
Tell the audience to look at **Terminal A**. They will see:
1. `🚨 INTRUSION DETECTED`
2. `🤖 AI Threat Score: 8.0/10` (Takes about ~15-30 seconds for the LLM to process)
3. `📢 Broadcasting quarantine vote...`

Then, tell them to look at **Terminal B**. They will see:
1. `✅ Vote 1 for quarantine verified`
2. `🔴 QUARANTINING NODE notification-service — severing all connections`

### Step 5: Clean Up
When you are done with the demo, simply run this command to kill all the background processes:
```bash
pkill -f peer-node && pkill -f app.py && pkill -f coordinator
```

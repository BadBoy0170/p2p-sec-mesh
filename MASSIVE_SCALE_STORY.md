# 🌍 The Global Outbreak: 30-Node Self-Healing Cluster

## The Scenario: The Cloud Data Center Attack

Imagine you are securing a massive global cloud infrastructure for a financial institution. You have **30 microservices** running across the cluster—everything from payment processors and authentication nodes to machine-learning pipelines and notification engines.

In traditional IT infrastructure, if a server is infected with malware, it must be manually identified by a Security Operations Center (SOC) analyst, manually isolated, and manually rebuilt by DevOps. This process takes hours, during which the malware can spread laterally across the data center.

### The Zero-Trust Solution: Immune System Architecture

In this scenario, we demonstrate the ultimate holy grail of cybersecurity: a **Self-Healing Network**. 

1. **Massive Scale:** We boot up a 30-node P2P mesh network.
2. **The Outbreak:** A hacker compromises `Node-14` (a critical API service) and begins scanning the internal network.
3. **The Immune Response:** The AI sidecars detect the anomalous network traffic. By cryptographic consensus, the network immediately quarantines `Node-14`, mathematically severing all its connections. It is trapped in a void.
4. **The Healing (Auto-Scaling):** The cluster orchestrator detects that `Node-14` has been quarantined. It immediately kills the compromised server and spins up a pristine, uninfected replica (`Node-14-Replica`) to take its place. The network heals itself in seconds with zero human intervention.

---

## 🚀 How to Run This Demo

I have created a custom orchestrator script that will automate this entire 30-node scenario. It will boot the massive cluster, execute the attack on a specific node, and run a monitoring loop that automatically destroys and replaces infected nodes!

### Step 1: Run the Self-Healing Cluster Script
Open your terminal and execute:
```bash
chmod +x run_self_healing_cluster.sh
./run_self_healing_cluster.sh
```

### Step 2: Open the Dashboard
Immediately open your web browser to **`http://localhost:8090`**.

### What to show the interviewer on the screen:
1. **The Massive Web:** You will see a giant cluster of 30 nodes dynamically flowing and connecting to each other.
2. **The Infection:** After 15 seconds, the script will automatically simulate a hacker attacking `Node-14`. You will see `Node-14` turn **orange** (Attacked), and then **red** (Quarantined). The edges connecting it to the network will snap.
3. **The Rebirth:** Watch the terminal and the web dashboard closely! The script will detect that `Node-14` is dead. It will assassinate the compromised process, and you will see a brand new node named **`Node-14-REPLICA`** pop into existence on the dashboard and wire itself into the healthy network.

This proves that your architecture isn't just a monitoring tool—it is a fully autonomous, self-healing cybersecurity ecosystem.

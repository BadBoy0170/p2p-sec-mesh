package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// PeerInfo holds information about a registered peer.
type PeerInfo struct {
	ID      string    `json:"id"`
	Address string    `json:"address"` // P2P gRPC address host:port
	PubKey  string    `json:"pub_key"` // hex-encoded Ed25519 public key
	LastSeen time.Time `json:"last_seen"`
}

// Registry manages the active peer list.
type Registry struct {
	mu        sync.RWMutex
	peers     map[string]PeerInfo
	telemetry map[string]NodeTelemetry // For GUI dashboard
}

// NodeTelemetry holds the latest state reported by a peer for the GUI
type NodeTelemetry struct {
	NodeID         string    `json:"node_id"`
	Status         string    `json:"status"` // "healthy", "attacked", "quarantined"
	Peers          []string  `json:"peers"`
	CpuPct         float64   `json:"cpu_pct"`
	RamPct         float64   `json:"ram_pct"`
	LastReportedAt time.Time `json:"last_reported_at"`
}

// AIReport holds a structured threat event submitted by a peer node.
type AIReport struct {
	NodeID     string    `json:"node_id"`
	Score      float64   `json:"score"`
	EventType  string    `json:"event_type"`
	SourceIP   string    `json:"source_ip"`
	Decision   string    `json:"decision"`
	Method     string    `json:"method"` // "ai" or "rule-based"
	ReportedAt time.Time `json:"reported_at"`
}

func NewRegistry() *Registry {
	r := &Registry{
		peers:     make(map[string]PeerInfo),
		telemetry: make(map[string]NodeTelemetry),
	}
	go r.pruneLoop()
	return r
}

// ── AI Report store ───────────────────────────────────────────────────────────

var (
	reportsMu sync.RWMutex
	reports   []AIReport
)

// addReport appends a new AI event report (capped at 200 entries).
func addReport(r AIReport) {
	r.ReportedAt = time.Now()
	reportsMu.Lock()
	reports = append(reports, r)
	if len(reports) > 200 {
		reports = reports[len(reports)-200:]
	}
	reportsMu.Unlock()
}

func (r *Registry) pruneLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		for id, p := range r.peers {
			if time.Since(p.LastSeen) > 15*time.Second {
				log.Printf("[COORDINATOR] Pruning inactive peer: %s", id)
				delete(r.peers, id)
			}
		}
		r.mu.Unlock()
	}
}

// POST /register  — body: {"id":"...","address":"host:port","pub_key":"hex"}
// Returns: {"peers": [...]}
func (r *Registry) handleRegister(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body PeerInfo
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.ID == "" || body.Address == "" {
		http.Error(w, "id and address required", http.StatusBadRequest)
		return
	}

	r.mu.Lock()
	body.LastSeen = time.Now()
	r.peers[body.ID] = body
	activePeers := r.peerList(body.ID) // return everyone except caller
	r.mu.Unlock()

	log.Printf("[COORDINATOR] Registered peer %s @ %s (public_key: %s...)", body.ID, body.Address, body.PubKey[:min(8, len(body.PubKey))])

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"peers": activePeers})
}

// peerList returns all peers except the one with the given ID.
// Caller must hold the read lock (or write lock).
func (r *Registry) peerList(excludeID string) []PeerInfo {
	var list []PeerInfo
	for id, p := range r.peers {
		if id != excludeID {
			list = append(list, p)
		}
	}
	return list
}

// GET /peers — returns all active peers as JSON array
func (r *Registry) handlePeers(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.mu.RLock()
	list := r.peerList("")
	r.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// POST /api/telemetry — Nodes report their status here
func (r *Registry) handleTelemetry(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body NodeTelemetry
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	
	r.mu.Lock()
	body.LastReportedAt = time.Now()
	r.telemetry[body.NodeID] = body
	r.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

// GET /api/topology — Returns the graph state for the Vis.js frontend
func (r *Registry) handleTopology(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Add CORS headers so frontend can read it if served from somewhere else
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	r.mu.RLock()
	var nodes []NodeTelemetry
	for _, t := range r.telemetry {
		// Prune old telemetry (older than 10s)
		if time.Since(t.LastReportedAt) < 10*time.Second {
			nodes = append(nodes, t)
		}
	}
	r.mu.RUnlock()

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
	})
}

// POST /api/report/submit — Peer nodes push AI/rule-based threat decisions here
func handleReportSubmit(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var r AIReport
	if err := json.NewDecoder(req.Body).Decode(&r); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	addReport(r)
	log.Printf("[COORDINATOR] AI Report: node=%s score=%.1f decision=%s method=%s",
		r.NodeID, r.Score, r.Decision, r.Method)
	w.WriteHeader(http.StatusOK)
}

// GET /api/report — Dashboard fetches the latest AI event reports
func handleReport(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	reportsMu.RLock()
	out := make([]AIReport, len(reports))
	copy(out, reports)
	reportsMu.RUnlock()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"events": out})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	port := os.Getenv("COORDINATOR_PORT")
	if port == "" {
		port = "8090"
	}

	reg := NewRegistry()
	mux := http.NewServeMux()
	mux.HandleFunc("/register", reg.handleRegister)
	mux.HandleFunc("/peers", reg.handlePeers)
	mux.HandleFunc("/api/telemetry", reg.handleTelemetry)
	mux.HandleFunc("/api/topology", reg.handleTopology)
	mux.HandleFunc("/api/report/submit", handleReportSubmit)
	mux.HandleFunc("/api/report", handleReport)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	
	// Serve the static HTML dashboard
	mux.Handle("/", http.FileServer(http.Dir("./coordinator")))

	addr := ":" + port
	log.Printf("[COORDINATOR] Listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

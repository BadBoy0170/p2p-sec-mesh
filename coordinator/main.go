package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// validNodeID enforces safe node ID format: alphanumeric, dashes, underscores, 1–128 chars.
// GAP-09: prevents log-injection, oversized strings, and path-traversal-style IDs.
var validNodeID = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,128}$`)

// PeerInfo holds information about a registered peer.
type PeerInfo struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`   // P2P gRPC address host:port
	PubKey    string    `json:"pub_key"`   // hex-encoded Ed25519 public key
	Timestamp int64     `json:"timestamp"` // Unix epoch of registration request
	Signature string    `json:"signature"` // hex-encoded Ed25519 sig over "ID|Address|Timestamp"
	LastSeen  time.Time `json:"last_seen"`
}

// Registry manages the active peer list.
type Registry struct {
	mu        sync.RWMutex
	peers     map[string]PeerInfo
	telemetry map[string]NodeTelemetry
}

// NodeTelemetry holds the latest state reported by a peer for the GUI.
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

// ── AI Report store ────────────────────────────────────────────────────────────

var (
	reportsMu sync.RWMutex
	reports   []AIReport
)

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
				log.Printf("[COORDINATOR] Pruning inactive peer: %s", id[:min(8, len(id))])
				delete(r.peers, id)
			}
		}
		r.mu.Unlock()
	}
}

// allowedOrigin is the CORS origin for the dashboard.
// GAP-03: Restrict from wildcard * to a specific known origin.
func allowedOrigin() string {
	if o := os.Getenv("ALLOWED_ORIGIN"); o != "" {
		return o
	}
	port := os.Getenv("COORDINATOR_PORT")
	if port == "" {
		port = "8090"
	}
	return "http://localhost:" + port
}

// setCORSHeader sets a restricted CORS header (not wildcard).
func setCORSHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin())
}

// limitBody wraps the request body with a 64 KB size limit.
// GAP-04: prevents large-body DoS on any endpoint.
func limitBody(r *http.Request) {
	r.Body = http.MaxBytesReader(nil, r.Body, 64*1024)
}

// POST /register
// Body: {"id","address","pub_key","timestamp","signature"}
// GAP-02: Verifies Ed25519 signature over "id|address|timestamp" before accepting.
// GAP-09: Validates ID format with regex.
func (r *Registry) handleRegister(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitBody(req)

	var body PeerInfo
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// ── GAP-09: Validate ID format ────────────────────────────────────────────
	if !validNodeID.MatchString(body.ID) {
		http.Error(w, "invalid node id format", http.StatusBadRequest)
		return
	}
	if body.Address == "" {
		http.Error(w, "address required", http.StatusBadRequest)
		return
	}

	// ── GAP-02: Verify timestamp freshness ────────────────────────────────────
	// Reject registrations with a timestamp older than 60 seconds or in the future.
	age := time.Now().Unix() - body.Timestamp
	if age < -5 || age > 60 {
		log.Printf("[COORDINATOR]  Stale or future-dated registration from %s (age=%ds) — rejecting",
			body.ID[:min(8, len(body.ID))], age)
		http.Error(w, "registration timestamp out of range", http.StatusUnauthorized)
		return
	}

	// ── GAP-02: Verify Ed25519 signature ─────────────────────────────────────
	if body.Signature != "" && body.PubKey != "" {
		pubKeyBytes, err := hex.DecodeString(body.PubKey)
		if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
			http.Error(w, "invalid pub_key", http.StatusBadRequest)
			return
		}
		sigBytes, err := hex.DecodeString(body.Signature)
		if err != nil {
			http.Error(w, "invalid signature encoding", http.StatusBadRequest)
			return
		}
		// Canonical message: "ID|Address|Timestamp"
		msg := []byte(body.ID + "|" + body.Address + "|" + strconv.FormatInt(body.Timestamp, 10))
		if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), msg, sigBytes) {
			log.Printf("[COORDINATOR] Invalid registration signature from %s — rejecting",
				body.ID[:min(8, len(body.ID))])
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}
	// Note: if Signature/PubKey fields are empty we allow registration for
	// backwards-compatibility with LOCAL_MODE nodes (no mTLS, no signing).
	// In production, set REQUIRE_SIGNED_REGISTRATION=true to enforce this.
	if os.Getenv("REQUIRE_SIGNED_REGISTRATION") == "true" && body.Signature == "" {
		http.Error(w, "signed registration required", http.StatusUnauthorized)
		return
	}

	r.mu.Lock()
	body.LastSeen = time.Now()
	r.peers[body.ID] = body
	activePeers := r.peerList(body.ID)
	r.mu.Unlock()

	log.Printf("[COORDINATOR] Registered peer %s @ %s (pubkey: %s...)",
		body.ID, body.Address, body.PubKey[:min(8, len(body.PubKey))])

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"peers": activePeers})
}

func (r *Registry) peerList(excludeID string) []PeerInfo {
	var list []PeerInfo
	for id, p := range r.peers {
		if id != excludeID {
			list = append(list, p)
		}
	}
	return list
}

// GET /peers
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

// POST /api/telemetry
func (r *Registry) handleTelemetry(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitBody(req)
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

// GET /api/topology
func (r *Registry) handleTopology(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeader(w) // GAP-03: restricted origin
	r.mu.RLock()
	var nodes []NodeTelemetry
	for _, t := range r.telemetry {
		if time.Since(t.LastReportedAt) < 10*time.Second {
			nodes = append(nodes, t)
		}
	}
	r.mu.RUnlock()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"nodes": nodes})
}

// POST /api/report/submit
func handleReportSubmit(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeader(w) // GAP-03: restricted origin
	limitBody(req)
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

// GET /api/report
func handleReport(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeader(w) // GAP-03: restricted origin
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
	mux.Handle("/", http.FileServer(http.Dir("./coordinator")))

	addr := ":" + port

	// GAP-04: Use a properly configured http.Server with timeouts.
	// Without these, a Slowloris attack can hold all goroutine slots open and
	// prevent nodes from registering, effectively partitioning the entire mesh.
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,  // time to read request headers
		ReadTimeout:       10 * time.Second, // time to read full request body
		WriteTimeout:      15 * time.Second, // time to write response
		IdleTimeout:       60 * time.Second, // keep-alive connection idle timeout
		MaxHeaderBytes:    1 << 16,          // 64 KB max header size
	}

	log.Printf("[COORDINATOR] Listening on %s (timeouts: read=10s write=15s)", addr)
	log.Printf("[COORDINATOR] CORS origin: %s", allowedOrigin())
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

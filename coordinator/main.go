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
	mu    sync.RWMutex
	peers map[string]PeerInfo
}

func NewRegistry() *Registry {
	r := &Registry{peers: make(map[string]PeerInfo)}
	go r.pruneLoop()
	return r
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
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	addr := ":" + port
	log.Printf("[COORDINATOR] Listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

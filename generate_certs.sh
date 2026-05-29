#!/bin/bash
# =============================================================================
# generate_certs.sh — Per-Node mTLS PKI for P2P Zero-Trust Security Mesh
#
# F-01 FIX: Previously all peer nodes shared one 'peer.pem' certificate.
# If a single node was compromised, its certificate could impersonate any other
# node because they all had the same CN and SAN. This script generates a
# UNIQUE certificate for every node (node-a … node-e) plus the coordinator
# and analyzer, each with its own CN and SAN. The CA cert remains shared and
# is the only trust anchor that needs distributing.
#
# Usage:
#   bash generate_certs.sh          # generates all certs
#   bash generate_certs.sh --clean  # removes old certs and regenerates
#
# Output layout:
#   certs/
#     ca.pem          — shared CA (distribute to all containers)
#     ca.key          — CA private key (keep offline / secret)
#     coordinator/
#       cert.pem  key.pem
#     node-a/ node-b/ node-c/ node-d/ node-e/
#       cert.pem  key.pem
#     analyzer/
#       cert.pem  key.pem
# =============================================================================

set -euo pipefail

CERT_DIR="certs"
DAYS_CA=3650    # 10-year CA validity
DAYS_NODE=365   # 1-year node cert validity

if [[ "${1:-}" == "--clean" ]]; then
  echo "[PKI] Cleaning old certificates..."
  rm -rf "$CERT_DIR"
fi

mkdir -p "$CERT_DIR"

# ── Step 1: Generate CA ───────────────────────────────────────────────────────
if [ ! -f "$CERT_DIR/ca.pem" ]; then
  echo "[PKI] Generating Certificate Authority..."
  openssl genpkey -algorithm RSA -out "$CERT_DIR/ca.key" -pkeyopt rsa_keygen_bits:4096 2>/dev/null
  openssl req -new -x509 -days "$DAYS_CA" \
    -key "$CERT_DIR/ca.key" \
    -out "$CERT_DIR/ca.pem" \
    -subj "/CN=P2PSecMeshCA/O=ZeroTrustMesh" 2>/dev/null
  echo "[PKI] ✅ CA generated: $CERT_DIR/ca.pem"
else
  echo "[PKI] ♻️  Reusing existing CA: $CERT_DIR/ca.pem"
fi

echo "01" > "$CERT_DIR/ca.srl" 2>/dev/null || true

# ── Step 2: Helper function to generate a per-node cert ───────────────────────
# F-01: Each node gets a unique CN (= its service hostname in Docker) and a
# matching SAN so the TLS handshake verifies the actual connecting hostname.
# A compromised node's cert cannot impersonate any other node.
generate_node_cert() {
  local name="$1"       # e.g. "node-a-go"
  local cn="$2"         # Common Name used in the cert
  local san="$3"        # Subject Alternative Names (comma-separated DNS names)
  local outdir="$CERT_DIR/$name"

  mkdir -p "$outdir"

  if [ -f "$outdir/cert.pem" ]; then
    echo "[PKI] ♻️  Cert already exists for $name — skipping (use --clean to regenerate)"
    return
  fi

  # Write SAN config inline
  local san_cfg=$(mktemp)
  cat > "$san_cfg" <<EOF
[req]
req_extensions = v3_req
distinguished_name = dn
[dn]
[v3_req]
subjectAltName = $san
[v3_ca]
EOF

  # Generate private key
  openssl genpkey -algorithm RSA -out "$outdir/key.pem" -pkeyopt rsa_keygen_bits:2048 2>/dev/null

  # Generate CSR
  openssl req -new \
    -key "$outdir/key.pem" \
    -out "$outdir/cert.csr" \
    -subj "/CN=$cn/O=ZeroTrustMesh" 2>/dev/null

  # Sign with CA, embedding the SAN extension
  openssl x509 -req \
    -in "$outdir/cert.csr" \
    -CA "$CERT_DIR/ca.pem" \
    -CAkey "$CERT_DIR/ca.key" \
    -CAserial "$CERT_DIR/ca.srl" \
    -out "$outdir/cert.pem" \
    -days "$DAYS_NODE" \
    -extensions v3_req \
    -extfile "$san_cfg" 2>/dev/null

  rm -f "$outdir/cert.csr" "$san_cfg"
  echo "[PKI] ✅ Cert generated for $name (CN=$cn, SAN=$san)"
}

# ── Step 3: Generate per-node certificates ────────────────────────────────────
echo ""
echo "[PKI] Generating per-node certificates (F-01: unique cert per node)..."

generate_node_cert "coordinator" \
  "coordinator" \
  "DNS:coordinator,DNS:localhost,IP:127.0.0.1"

generate_node_cert "node-a" \
  "node-a-go" \
  "DNS:node-a-go,DNS:node-a,DNS:localhost,IP:127.0.0.1"

generate_node_cert "node-b" \
  "node-b-go" \
  "DNS:node-b-go,DNS:node-b,DNS:localhost,IP:127.0.0.1"

generate_node_cert "node-c" \
  "node-c-go" \
  "DNS:node-c-go,DNS:node-c,DNS:localhost,IP:127.0.0.1"

generate_node_cert "node-d" \
  "node-d-go" \
  "DNS:node-d-go,DNS:node-d,DNS:localhost,IP:127.0.0.1"

generate_node_cert "node-e" \
  "node-e-go" \
  "DNS:node-e-go,DNS:node-e,DNS:localhost,IP:127.0.0.1"

generate_node_cert "analyzer" \
  "analyzer" \
  "DNS:analyzer,DNS:localhost,IP:127.0.0.1"

# ── Step 4: Backwards-compatible symlinks ─────────────────────────────────────
# Keep certs/peer.pem pointing at node-a's cert so LOCAL_MODE still works.
# (LOCAL_MODE ignores certs anyway, but this prevents path errors on startup.)
ln -sf "node-a/cert.pem" "$CERT_DIR/peer.pem" 2>/dev/null || true
ln -sf "node-a/key.pem"  "$CERT_DIR/peer.key" 2>/dev/null || true

echo ""
echo "========================================================"
echo " Per-node mTLS certificates generated successfully"
echo "========================================================"
echo " CA:          $CERT_DIR/ca.pem"
echo " Nodes:       $CERT_DIR/node-{a..e}/"
echo " Coordinator: $CERT_DIR/coordinator/"
echo ""
echo " Each node gets a UNIQUE cert — a compromised node's"
echo " certificate cannot impersonate any other node."
echo "========================================================"

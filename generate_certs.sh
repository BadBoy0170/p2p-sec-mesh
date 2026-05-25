#!/bin/bash

# Directory to store certificates
CERT_DIR="certs"
mkdir -p $CERT_DIR

# CA private key and certificate
openssl genpkey -algorithm RSA -out $CERT_DIR/ca.key
openssl req -new -x509 -days 3650 -key $CERT_DIR/ca.key -out $CERT_DIR/ca.pem -subj "/CN=P2PSecMeshCA"

# Services to generate certs for
SERVICES=("coordinator" "peer" "analyzer")

for service in "${SERVICES[@]}"; do
  echo "Generating certs for $service..."

  # Private key
  openssl genpkey -algorithm RSA -out $CERT_DIR/$service.key

  # Certificate Signing Request (CSR)
  openssl req -new -key $CERT_DIR/$service.key -out $CERT_DIR/$service.csr -subj "/CN=$service"

  # Sign the CSR with the CA
  openssl x509 -req -in $CERT_DIR/$service.csr -CA $CERT_DIR/ca.pem -CAkey $CERT_DIR/ca.key -CAcreateserial -out $CERT_DIR/$service.pem -days 365

  # Clean up CSR
  rm $CERT_DIR/$service.csr
done

# Create a serial file for the CA if it doesn't exist, to avoid errors on re-runs
if [ ! -f "$CERT_DIR/ca.srl" ]; then
    echo "01" > $CERT_DIR/ca.srl
fi

echo "mTLS certificates generated successfully in $CERT_DIR/"

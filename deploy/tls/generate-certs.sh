#!/bin/bash
# Generate self-signed certificates for K8sWatch mTLS (Development Only)
# For production, use cert-manager with a proper CA

set -e

OUTPUT_DIR="${1:-./tls-certs}"
VALIDITY_DAYS=365

echo "Creating certificate output directory: ${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"

# Generate CA private key and certificate
echo "Generating CA key and certificate..."
openssl genrsa -out "${OUTPUT_DIR}/ca.key" 4096
openssl req -x509 -new -nodes -sha256 -days "${VALIDITY_DAYS}" \
  -key "${OUTPUT_DIR}/ca.key" \
  -out "${OUTPUT_DIR}/ca.crt" \
  -subj "/O=k8swatch/CN=k8swatch-ca"

# Generate aggregator server certificate
echo "Generating aggregator server certificate..."
openssl genrsa -out "${OUTPUT_DIR}/aggregator.key" 2048

cat > "${OUTPUT_DIR}/aggregator.cnf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = k8swatch-aggregator

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = k8swatch-aggregator.k8swatch.svc.cluster.local
DNS.2 = k8swatch-aggregator.k8swatch.svc
DNS.3 = k8swatch-aggregator
EOF

openssl req -new -sha256 \
  -key "${OUTPUT_DIR}/aggregator.key" \
  -out "${OUTPUT_DIR}/aggregator.csr" \
  -config "${OUTPUT_DIR}/aggregator.cnf"

openssl x509 -req -sha256 -days "${VALIDITY_DAYS}" \
  -in "${OUTPUT_DIR}/aggregator.csr" \
  -CA "${OUTPUT_DIR}/ca.crt" \
  -CAkey "${OUTPUT_DIR}/ca.key" \
  -CAcreateserial \
  -out "${OUTPUT_DIR}/aggregator.crt" \
  -extensions v3_req \
  -extfile "${OUTPUT_DIR}/aggregator.cnf"

# Generate agent client certificate
echo "Generating agent client certificate..."
openssl genrsa -out "${OUTPUT_DIR}/agent.key" 2048

cat > "${OUTPUT_DIR}/agent.cnf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = k8swatch-agent

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = clientAuth
EOF

openssl req -new -sha256 \
  -key "${OUTPUT_DIR}/agent.key" \
  -out "${OUTPUT_DIR}/agent.csr" \
  -subj "/O=k8swatch/CN=k8swatch-agent" \
  -config "${OUTPUT_DIR}/agent.cnf"

openssl x509 -req -sha256 -days "${VALIDITY_DAYS}" \
  -in "${OUTPUT_DIR}/agent.csr" \
  -CA "${OUTPUT_DIR}/ca.crt" \
  -CAkey "${OUTPUT_DIR}/ca.key" \
  -CAcreateserial \
  -out "${OUTPUT_DIR}/agent.crt" \
  -extensions v3_req \
  -extfile "${OUTPUT_DIR}/agent.cnf"

# Generate alertmanager certificate
echo "Generating alertmanager certificate..."
openssl genrsa -out "${OUTPUT_DIR}/alertmanager.key" 2048

cat > "${OUTPUT_DIR}/alertmanager.cnf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = k8swatch-alertmanager

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = k8swatch-alertmanager.k8swatch.svc.cluster.local
DNS.2 = k8swatch-alertmanager.k8swatch.svc
DNS.3 = k8swatch-alertmanager
EOF

openssl req -new -sha256 \
  -key "${OUTPUT_DIR}/alertmanager.key" \
  -out "${OUTPUT_DIR}/alertmanager.csr" \
  -config "${OUTPUT_DIR}/alertmanager.cnf"

openssl x509 -req -sha256 -days "${VALIDITY_DAYS}" \
  -in "${OUTPUT_DIR}/alertmanager.csr" \
  -CA "${OUTPUT_DIR}/ca.crt" \
  -CAkey "${OUTPUT_DIR}/ca.key" \
  -CAcreateserial \
  -out "${OUTPUT_DIR}/alertmanager.crt" \
  -extensions v3_req \
  -extfile "${OUTPUT_DIR}/alertmanager.cnf"

# Verify certificates
echo ""
echo "Verifying certificates..."
echo ""

echo "CA Certificate:"
openssl x509 -in "${OUTPUT_DIR}/ca.crt" -noout -subject -issuer -dates
echo ""

echo "Aggregator Certificate:"
openssl x509 -in "${OUTPUT_DIR}/aggregator.crt" -noout -subject -issuer -dates -ext subjectAltName
echo ""

echo "Agent Certificate:"
openssl x509 -in "${OUTPUT_DIR}/agent.crt" -noout -subject -issuer -dates
echo ""

echo "AlertManager Certificate:"
openssl x509 -in "${OUTPUT_DIR}/alertmanager.crt" -noout -subject -issuer -dates -ext subjectAltName
echo ""

# Verify certificate chain
echo "Verifying certificate chain..."
openssl verify -CAfile "${OUTPUT_DIR}/ca.crt" "${OUTPUT_DIR}/aggregator.crt"
openssl verify -CAfile "${OUTPUT_DIR}/ca.crt" "${OUTPUT_DIR}/agent.crt"
openssl verify -CAfile "${OUTPUT_DIR}/ca.crt" "${OUTPUT_DIR}/alertmanager.crt"

# Create Kubernetes secrets
echo ""
echo "To create Kubernetes secrets, run:"
echo ""
echo "kubectl create secret tls k8swatch-aggregator-tls \\"
echo "  --cert=${OUTPUT_DIR}/aggregator.crt \\"
echo "  --key=${OUTPUT_DIR}/aggregator.key \\"
echo "  -n k8swatch"
echo ""
echo "kubectl create secret tls k8swatch-agent-tls \\"
echo "  --cert=${OUTPUT_DIR}/agent.crt \\"
echo "  --key=${OUTPUT_DIR}/agent.key \\"
echo "  -n k8swatch"
echo ""
echo "kubectl create secret tls k8swatch-alertmanager-tls \\"
echo "  --cert=${OUTPUT_DIR}/alertmanager.crt \\"
echo "  --key=${OUTPUT_DIR}/alertmanager.key \\"
echo "  -n k8swatch"
echo ""
echo "kubectl create secret generic k8swatch-ca-cert \\"
echo "  --from-file=ca.crt=${OUTPUT_DIR}/ca.crt \\"
echo "  --from-file=ca.key=${OUTPUT_DIR}/ca.key \\"
echo "  -n k8swatch"
echo ""

echo "Certificate generation complete!"
echo "Output directory: ${OUTPUT_DIR}"

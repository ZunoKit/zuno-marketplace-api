#!/bin/bash

# Certificate Generation Script for mTLS
# This script generates CA, server, and client certificates for secure gRPC communication

set -e

# Configuration
CERT_DIR="$(dirname "$0")"
DAYS_VALID=3650  # 10 years
COUNTRY="US"
STATE="California"
CITY="San Francisco"
ORGANIZATION="Zuno Marketplace"
ORGANIZATIONAL_UNIT="Engineering"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting certificate generation for mTLS...${NC}"

# Function to generate certificate
generate_cert() {
    local name=$1
    local cn=$2
    local type=$3  # "ca", "server", or "client"
    
    echo -e "${YELLOW}Generating $type certificate for $name...${NC}"
    
    # Generate private key
    openssl genpkey -algorithm RSA -out "$CERT_DIR/$name.key" -pkeyopt rsa_keygen_bits:4096
    
    if [ "$type" == "ca" ]; then
        # Generate CA certificate
        openssl req -new -x509 -days $DAYS_VALID -key "$CERT_DIR/$name.key" \
            -out "$CERT_DIR/$name.crt" \
            -subj "/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORGANIZATION/OU=$ORGANIZATIONAL_UNIT/CN=$cn"
    else
        # Generate certificate signing request
        openssl req -new -key "$CERT_DIR/$name.key" -out "$CERT_DIR/$name.csr" \
            -subj "/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORGANIZATION/OU=$ORGANIZATIONAL_UNIT/CN=$cn"
        
        # Create extensions file for server certificates
        if [ "$type" == "server" ]; then
            cat > "$CERT_DIR/$name.ext" <<EOF
subjectAltName = DNS:$cn,DNS:localhost,DNS:*.zuno-marketplace.local,IP:127.0.0.1,IP:::1
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
EOF
        else
            # Client certificate extensions
            cat > "$CERT_DIR/$name.ext" <<EOF
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF
        fi
        
        # Sign certificate with CA
        openssl x509 -req -in "$CERT_DIR/$name.csr" \
            -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
            -out "$CERT_DIR/$name.crt" -days $DAYS_VALID \
            -extfile "$CERT_DIR/$name.ext"
        
        # Clean up temporary files
        rm -f "$CERT_DIR/$name.csr" "$CERT_DIR/$name.ext"
    fi
    
    # Set appropriate permissions
    chmod 400 "$CERT_DIR/$name.key"
    chmod 444 "$CERT_DIR/$name.crt"
    
    echo -e "${GREEN}✓ Generated $type certificate for $name${NC}"
}

# Step 1: Generate CA certificate
generate_cert "ca" "Zuno Marketplace CA" "ca"

# Step 2: Generate server certificates for each service
services=(
    "auth-service:50051"
    "user-service:50052"
    "wallet-service:50053"
    "orchestrator-service:50054"
    "media-service:50055"
    "chain-registry-service:50056"
    "catalog-service:50057"
    "indexer-service:50058"
    "graphql-gateway:8081"
)

for service_info in "${services[@]}"; do
    service_name="${service_info%%:*}"
    generate_cert "$service_name" "$service_name.zuno-marketplace.local" "server"
done

# Step 3: Generate client certificates
generate_cert "graphql-gateway-client" "graphql-gateway-client" "client"
generate_cert "admin-client" "admin-client" "client"

# Step 4: Create certificate bundles for easy distribution
echo -e "${YELLOW}Creating certificate bundles...${NC}"

# Bundle for servers (CA cert only, for validating clients)
cp "$CERT_DIR/ca.crt" "$CERT_DIR/ca-bundle.crt"

# Bundle for clients (includes CA cert)
cp "$CERT_DIR/ca.crt" "$CERT_DIR/client-ca-bundle.crt"

# Step 5: Generate certificate info file
cat > "$CERT_DIR/cert-info.json" <<EOF
{
  "generated_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "valid_days": $DAYS_VALID,
  "ca": {
    "cn": "Zuno Marketplace CA",
    "cert": "ca.crt",
    "key": "ca.key"
  },
  "servers": [
EOF

first=true
for service_info in "${services[@]}"; do
    service_name="${service_info%%:*}"
    port="${service_info##*:}"
    if [ "$first" = true ]; then
        first=false
    else
        echo "," >> "$CERT_DIR/cert-info.json"
    fi
    cat >> "$CERT_DIR/cert-info.json" <<EOF
    {
      "name": "$service_name",
      "port": "$port",
      "cert": "$service_name.crt",
      "key": "$service_name.key",
      "cn": "$service_name.zuno-marketplace.local"
    }
EOF
done

cat >> "$CERT_DIR/cert-info.json" <<EOF
  ],
  "clients": [
    {
      "name": "graphql-gateway-client",
      "cert": "graphql-gateway-client.crt",
      "key": "graphql-gateway-client.key"
    },
    {
      "name": "admin-client",
      "cert": "admin-client.crt",
      "key": "admin-client.key"
    }
  ]
}
EOF

# Step 6: Verify certificates
echo -e "${YELLOW}Verifying certificates...${NC}"

for service_info in "${services[@]}"; do
    service_name="${service_info%%:*}"
    if openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/$service_name.crt" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ $service_name certificate is valid${NC}"
    else
        echo -e "${RED}✗ $service_name certificate verification failed${NC}"
        exit 1
    fi
done

# Verify client certificates
if openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/graphql-gateway-client.crt" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ graphql-gateway-client certificate is valid${NC}"
else
    echo -e "${RED}✗ graphql-gateway-client certificate verification failed${NC}"
    exit 1
fi

echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}Certificate generation completed successfully!${NC}"
echo -e "${GREEN}=====================================${NC}"
echo ""
echo "Generated files in $CERT_DIR:"
ls -la "$CERT_DIR"/*.crt "$CERT_DIR"/*.key 2>/dev/null | awk '{print "  " $9}'
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Copy certificates to appropriate service directories"
echo "2. Update service configurations to use TLS"
echo "3. Set proper file permissions in production"
echo "4. Implement certificate rotation before expiry ($(date -d "+$DAYS_VALID days" +"%Y-%m-%d"))"

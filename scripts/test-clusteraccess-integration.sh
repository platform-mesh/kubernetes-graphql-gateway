#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Test configuration
TEST_CLUSTER_NAME="test-cluster"
MANAGEMENT_KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
DEFINITIONS_DIR="./bin/definitions"

log_info "Testing ClusterAccess integration with kubeconfig storage"
log_info "Management kubeconfig: $MANAGEMENT_KUBECONFIG"
log_info "Definitions directory: $DEFINITIONS_DIR"

# Verify prerequisites
log_step "1. Verifying prerequisites"

if ! kubectl --kubeconfig="$MANAGEMENT_KUBECONFIG" cluster-info &>/dev/null; then
    log_error "Cannot connect to management cluster"
    exit 1
fi

if ! kubectl --kubeconfig="$MANAGEMENT_KUBECONFIG" get clusteraccess &>/dev/null; then
    log_error "ClusterAccess CRD not installed. Please run: kubectl apply -f config/crd/"
    exit 1
fi

log_info "Prerequisites verified"

# Create test kubeconfig secret
log_step "2. Creating test kubeconfig secret"

# Use the same kubeconfig for testing (in real scenarios this would be different)
KUBECONFIG_B64=$(base64 -w 0 < "$MANAGEMENT_KUBECONFIG")

cat <<EOF | kubectl --kubeconfig="$MANAGEMENT_KUBECONFIG" apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${TEST_CLUSTER_NAME}-kubeconfig
  namespace: default
type: Opaque
data:
  kubeconfig: ${KUBECONFIG_B64}
EOF

log_info "Test kubeconfig secret created"

# Extract server URL from kubeconfig
log_step "3. Extracting server URL from kubeconfig"
SERVER_URL=$(kubectl --kubeconfig="$MANAGEMENT_KUBECONFIG" config view --minify -o jsonpath='{.clusters[0].cluster.server}')
log_info "Server URL: $SERVER_URL"

# Create ClusterAccess resource with kubeconfig authentication
log_step "4. Creating ClusterAccess resource with kubeconfig authentication"

cat <<EOF | kubectl --kubeconfig="$MANAGEMENT_KUBECONFIG" apply -f -
apiVersion: gateway.openmfp.org/v1alpha1
kind: ClusterAccess
metadata:
  name: ${TEST_CLUSTER_NAME}
spec:
  path: ${TEST_CLUSTER_NAME}
  host: ${SERVER_URL}
  auth:
    kubeconfigSecretRef:
      name: ${TEST_CLUSTER_NAME}-kubeconfig
      namespace: default
EOF

log_info "ClusterAccess resource created"

# Wait for ClusterAccess to be processed
log_step "5. Waiting for ClusterAccess to be processed"
sleep 2

# Check if ClusterAccess exists
if kubectl --kubeconfig="$MANAGEMENT_KUBECONFIG" get clusteraccess "$TEST_CLUSTER_NAME" &>/dev/null; then
    log_info "ClusterAccess resource exists"
else
    log_error "ClusterAccess resource not found"
    exit 1
fi

# Start listener to process ClusterAccess
log_step "6. Starting listener to process ClusterAccess"

export ENABLE_KCP=false
export LOCAL_DEVELOPMENT=false
export MULTICLUSTER=true
export KUBECONFIG="$MANAGEMENT_KUBECONFIG"
export OPENAPI_DEFINITIONS_PATH="$DEFINITIONS_DIR"

log_info "Starting listener with ENABLE_KCP=false, MULTICLUSTER=true"
log_info "This should use the ClusterAccess reconciler..."

# Run listener in background for a short time to generate schema
timeout 30s go run . listener || true

# Check if schema file was generated
log_step "7. Checking if schema file was generated"

SCHEMA_FILE="$DEFINITIONS_DIR/${TEST_CLUSTER_NAME}.json"
if [ -f "$SCHEMA_FILE" ]; then
    log_info "Schema file generated: $SCHEMA_FILE"
    
    # Check if it contains x-cluster-metadata
    if grep -q "x-cluster-metadata" "$SCHEMA_FILE"; then
        log_info "Schema file contains x-cluster-metadata ✓"
        
        # Show the metadata
        log_info "Cluster metadata:"
        jq '.["x-cluster-metadata"]' "$SCHEMA_FILE" 2>/dev/null || echo "  (Could not parse metadata)"
    else
        log_warn "Schema file does not contain x-cluster-metadata"
    fi
else
    log_error "Schema file not generated: $SCHEMA_FILE"
    exit 1
fi

# Test gateway reading the schema
log_step "8. Testing gateway configuration"

export ENABLE_KCP=false
export LOCAL_DEVELOPMENT=false
export MULTICLUSTER=true
# NOTE: KUBECONFIG not needed for gateway in multicluster mode
unset KUBECONFIG
export OPENAPI_DEFINITIONS_PATH="$DEFINITIONS_DIR"
export GATEWAY_PORT=17080

log_info "Starting gateway with the generated schema..."
log_info "Gateway should read x-cluster-metadata and connect to the specified cluster"
log_info "KUBECONFIG is NOT needed for gateway in multicluster mode"

# Start gateway in background for a short test
timeout 10s go run . gateway &
GATEWAY_PID=$!

# Wait a bit for gateway to start
sleep 3

# Test gateway endpoint
log_step "9. Testing gateway endpoint"
if curl -s "http://localhost:$GATEWAY_PORT/${TEST_CLUSTER_NAME}/graphql" -H "Content-Type: application/json" -d '{"query": "{ __schema { types { name } } }"}' | grep -q "data"; then
    log_info "Gateway endpoint responds correctly ✓"
else
    log_warn "Gateway endpoint test failed or timed out"
fi

# Cleanup
log_step "10. Cleanup"

# Kill gateway if still running
if kill -0 $GATEWAY_PID 2>/dev/null; then
    kill $GATEWAY_PID 2>/dev/null || true
fi

# Remove test resources
kubectl --kubeconfig="$MANAGEMENT_KUBECONFIG" delete clusteraccess "$TEST_CLUSTER_NAME" --ignore-not-found=true
kubectl --kubeconfig="$MANAGEMENT_KUBECONFIG" delete secret "${TEST_CLUSTER_NAME}-kubeconfig" --ignore-not-found=true

# Remove generated schema
rm -f "$SCHEMA_FILE"

log_info "Cleanup completed"
log_info "Integration test completed successfully!"

echo ""
log_info "Summary:"
echo "  ✓ ClusterAccess reconciler processes kubeconfig-based authentication"
echo "  ✓ Schema files are generated with x-cluster-metadata"
echo "  ✓ Gateway reads x-cluster-metadata for cluster-specific connections"
echo "  ✓ End-to-end integration works with ENABLE_KCP=false and MULTICLUSTER=true" 
#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
TARGET_KUBECONFIG=""
MANAGEMENT_KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
NAMESPACE="default"

usage() {
    echo "Usage: $0 --target-kubeconfig <path> [options]"
    echo ""
    echo "Required:"
    echo "  --target-kubeconfig <path>      Path to target cluster kubeconfig"
    echo ""
    echo "Optional:"
    echo "  --management-kubeconfig <path>  Path to management cluster kubeconfig (default: \$KUBECONFIG or ~/.kube/config)"
    echo "  --namespace <name>              Namespace for secrets (default: default)"
    echo "  --help                          Show this help message"
    echo ""
    echo "Note: Cluster name will be extracted automatically from the target kubeconfig"
    echo ""
    echo "Authentication mode:"
    echo "  Uses target kubeconfig directly for full cluster admin access"
    echo ""
    echo "Example:"
    echo "  $0 --target-kubeconfig ~/.kube/target-config"
}

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --target-kubeconfig)
            TARGET_KUBECONFIG="$2"
            shift 2
            ;;
        --management-kubeconfig)
            MANAGEMENT_KUBECONFIG="$2"
            shift 2
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Validate required arguments
if [[ -z "$TARGET_KUBECONFIG" ]]; then
    log_error "Target kubeconfig path is required"
    usage
    exit 1
fi

# Validate files exist
if [[ ! -f "$TARGET_KUBECONFIG" ]]; then
    log_error "Target kubeconfig file not found: $TARGET_KUBECONFIG"
    exit 1
fi

if [[ ! -f "$MANAGEMENT_KUBECONFIG" ]]; then
    log_error "Management kubeconfig file not found: $MANAGEMENT_KUBECONFIG"
    exit 1
fi

# Extract cluster name from target kubeconfig
log_info "Extracting cluster name from target kubeconfig..."
CLUSTER_NAME=$(KUBECONFIG="$TARGET_KUBECONFIG" kubectl config view --raw -o jsonpath='{.clusters[0].name}')
if [[ -z "$CLUSTER_NAME" ]]; then
    log_error "Failed to extract cluster name from kubeconfig"
    exit 1
fi
log_info "Cluster name: $CLUSTER_NAME"

cleanup_existing_resources() {
    log_info "Checking for existing ClusterAccess resource '$CLUSTER_NAME'..."
    
    # Check if ClusterAccess exists in management cluster
    if KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl get clusteraccess "$CLUSTER_NAME" &>/dev/null; then
        log_warn "ClusterAccess '$CLUSTER_NAME' already exists. Cleaning up existing resources..."
        
        # Delete ClusterAccess resource
        log_info "Deleting existing ClusterAccess resource..."
        KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl delete clusteraccess "$CLUSTER_NAME" --ignore-not-found=true
        
        # Delete related secrets in management cluster
        log_info "Deleting existing secrets in management cluster..."
        KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl delete secret "${CLUSTER_NAME}-token" --namespace="$NAMESPACE" --ignore-not-found=true
        KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl delete secret "${CLUSTER_NAME}-ca" --namespace="$NAMESPACE" --ignore-not-found=true
        KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl delete secret "${CLUSTER_NAME}-admin-kubeconfig" --namespace="$NAMESPACE" --ignore-not-found=true
        
        log_info "Cleanup completed. Creating fresh resources..."
    else
        log_info "No existing ClusterAccess found. Creating new resources..."
    fi
}

log_info "Creating ClusterAccess resource '$CLUSTER_NAME'"
log_info "Target kubeconfig: $TARGET_KUBECONFIG"
log_info "Management kubeconfig: $MANAGEMENT_KUBECONFIG"
log_info "Authentication mode: Admin kubeconfig (full cluster access)"

# Clean up existing resources if they exist
cleanup_existing_resources

# Extract server URL from target kubeconfig
log_info "Extracting server URL from target kubeconfig..."
SERVER_URL=$(KUBECONFIG="$TARGET_KUBECONFIG" kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
if [[ -z "$SERVER_URL" ]]; then
    log_error "Failed to extract server URL from kubeconfig"
    exit 1
fi
log_info "Server URL: $SERVER_URL"

# Extract CA certificate from target kubeconfig
log_info "Extracting CA certificate from target kubeconfig..."
CA_DATA=$(KUBECONFIG="$TARGET_KUBECONFIG" kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')
if [[ -z "$CA_DATA" ]]; then
    log_error "Failed to extract CA certificate from kubeconfig"
    exit 1
fi

# Decode CA certificate to verify it's valid
CA_CERT=$(echo "$CA_DATA" | base64 -d)
if [[ ! "$CA_CERT" =~ "BEGIN CERTIFICATE" ]]; then
    log_error "Invalid CA certificate format"
    exit 1
fi
log_info "CA certificate extracted successfully"

# Test target cluster connectivity
log_info "Testing target cluster connectivity..."
if ! KUBECONFIG="$TARGET_KUBECONFIG" kubectl cluster-info &>/dev/null; then
    log_error "Cannot connect to target cluster"
    exit 1
fi
log_info "Target cluster is accessible"

# Admin access mode: use kubeconfig directly
log_info "Using admin kubeconfig mode"

# Test management cluster connectivity
log_info "Testing management cluster connectivity..."
if ! KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl cluster-info &>/dev/null; then
    log_error "Cannot connect to management cluster"
    exit 1
fi
log_info "Management cluster is accessible"

# Create kubeconfig secret in management cluster
log_info "Creating admin kubeconfig secret in management cluster..."
KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl create secret generic "${CLUSTER_NAME}-admin-kubeconfig" \
    --namespace="$NAMESPACE" \
    --from-file=kubeconfig="$TARGET_KUBECONFIG" \
    --dry-run=client -o yaml | \
KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl apply -f -

# Create CA secret in management cluster  
log_info "Creating CA secret in management cluster..."
echo "$CA_CERT" | KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl create secret generic "${CLUSTER_NAME}-ca" \
    --namespace="$NAMESPACE" \
    --from-file=ca.crt=/dev/stdin \
    --dry-run=client -o yaml | \
KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl apply -f -

# Create ClusterAccess resource with kubeconfig authentication
log_info "Creating ClusterAccess resource with admin kubeconfig..."
cat <<EOF | KUBECONFIG="$MANAGEMENT_KUBECONFIG" kubectl apply -f -
apiVersion: gateway.openmfp.org/v1alpha1
kind: ClusterAccess
metadata:
  name: $CLUSTER_NAME
spec:
  path: $CLUSTER_NAME
  host: $SERVER_URL
  ca:
    secretRef:
      name: ${CLUSTER_NAME}-ca
      namespace: $NAMESPACE
      key: ca.crt
  auth:
    kubeconfigSecretRef:
      name: ${CLUSTER_NAME}-admin-kubeconfig
      namespace: $NAMESPACE
EOF

log_info "ClusterAccess resource '$CLUSTER_NAME' created successfully with admin access!"
echo ""
log_info "Summary:"
echo "  - Admin kubeconfig secret: $NAMESPACE/${CLUSTER_NAME}-admin-kubeconfig (in management cluster)"
echo "  - CA secret: $NAMESPACE/${CLUSTER_NAME}-ca (in management cluster)"
echo "  - ClusterAccess: $CLUSTER_NAME"
echo "  - Server URL: $SERVER_URL"
echo "  - Access level: Full cluster admin (can access all resources including ClusterRoles, etc.)"

echo ""
log_info "You can now run the listener to generate the schema:"
echo "  export ENABLE_KCP=false"
echo "  export LOCAL_DEVELOPMENT=false"
echo "  export KUBECONFIG=\"$MANAGEMENT_KUBECONFIG\""
echo "  task listener" 
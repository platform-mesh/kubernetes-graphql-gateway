# ClusterAccess Resource Setup

To enable the gateway to access external Kubernetes clusters, you need to create ClusterAccess resources. This section provides both automated script and manual step-by-step instructions.

## Quick Setup (Recommended)

For development purposes, use the provided script to automatically create ClusterAccess resources:

```bash
./scripts/create-clusteraccess.sh --cluster-name my-cluster --target-kubeconfig /path/to/target-cluster-config
```

**Example:**
```bash
./scripts/create-clusteraccess.sh \
  --cluster-name production-cluster \
  --target-kubeconfig ~/.kube/production-config \
  --management-kubeconfig ~/.kube/management-config
```

The script will:
- Extract server URL and CA certificate from the target kubeconfig
- Create a service account with proper permissions in the target cluster
- Generate a token for the service account
- Create the necessary secrets in the management cluster
- Create the ClusterAccess resource

## Manual Setup

## Prerequisites

- Access to the target cluster (cluster you want to expose via GraphQL)
- Access to the management cluster (cluster where the gateway runs)
- ClusterAccess CRDs installed in the management cluster

## Step 1: Extract Token from Target Cluster

```bash
# Switch to target cluster
export KUBECONFIG=/path/to/target-cluster-kubeconfig

# Create a service account token (24h duration)
kubectl create token default --duration=24h
```

Copy the output token - you'll need it for the secret.

## Step 2: Extract CA Certificate from Target Cluster

```bash
# Extract CA certificate from kubeconfig
kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' | base64 -d
```

Copy the output (should start with `-----BEGIN CERTIFICATE-----` and end with `-----END CERTIFICATE-----`).

## Step 3: Get Target Cluster Server URL

```bash
# Get the server URL
kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.server}'
```

Copy the server URL (e.g., `https://127.0.0.1:58308`).

## Step 4: Switch Back to Management Cluster

```bash
# Switch to the cluster where you'll create ClusterAccess
export KUBECONFIG=/path/to/management-cluster-kubeconfig

# Install ClusterAccess CRD if not already installed
kubectl apply -f config/crd/
```

## Step 5: Create Complete YAML File

Create a file called `my-cluster-access.yaml`:

```yaml
# Secret containing token for target-cluster
apiVersion: v1
kind: Secret
metadata:
  name: my-target-cluster-token
  namespace: default
type: Opaque
stringData:
  token: PASTE_TOKEN_FROM_STEP_1_HERE

---
# Secret containing CA certificate for target-cluster
apiVersion: v1
kind: Secret
metadata:
  name: my-target-cluster-ca
  namespace: default
type: Opaque
stringData:
  ca.crt: |
    PASTE_CA_CERTIFICATE_FROM_STEP_2_HERE

---
# ClusterAccess resource for target-cluster
apiVersion: gateway.openmfp.org/v1alpha1
kind: ClusterAccess
metadata:
  name: my-target-cluster
spec:
  path: my-target-cluster  # This becomes the filename in bin/definitions/
  host: PASTE_SERVER_URL_FROM_STEP_3_HERE
  ca:
    secretRef:
      name: my-target-cluster-ca
      namespace: default
      key: ca.crt
  auth:
    secretRef:
      name: my-target-cluster-token
      namespace: default
      key: token
```

## Step 6: Apply the Configuration

```bash
kubectl apply -f my-cluster-access.yaml
```

## Step 7: Verify Resources

```bash
# Check if ClusterAccess was created
kubectl get clusteraccess

# Check if secrets were created
kubectl get secret my-target-cluster-token my-target-cluster-ca
```

## Step 8: Test with Listener

```bash
export ENABLE_KCP=false
export LOCAL_DEVELOPMENT=false
export KUBECONFIG=/path/to/management-cluster-kubeconfig
task listener
```

## Key Points

- **Token**: Use `kubectl create token` for simplicity and automatic expiration
- **CA Certificate**: Essential for TLS verification - without it you'll get certificate errors
- **Server URL**: Must match exactly from the target cluster's kubeconfig
- **Path**: Becomes the schema filename (e.g., `my-target-cluster`) in `bin/definitions/`
- **Secrets**: Keep them in the same namespace as the ClusterAccess resource

The listener will detect the ClusterAccess resource and generate schema files with metadata that the gateway can use to access the target cluster. 
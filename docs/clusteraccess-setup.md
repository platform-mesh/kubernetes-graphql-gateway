# ClusterAccess Resource Setup

To enable the gateway to access external Kubernetes clusters, you need to create ClusterAccess resources. This section provides both an automated script and manual step-by-step instructions.

## Quick Setup (Recommended)

For development purposes, use the provided script to automatically create ClusterAccess resources:

**Example:**
```bash
./hack/create-clusteraccess.sh --target-kubeconfig ~/.kube/platform-mesh-config --management-kubeconfig ~/.kube/platform-mesh-config
```

The script will:
- Extract cluster name, server URL, and CA certificate from the target kubeconfig
- Create a ServiceAccount with cluster-admin access in the target cluster
- Generate a long-lived token for the ServiceAccount
- Create the admin kubeconfig and CA secrets in the management cluster
- Create the ClusterAccess resource with kubeconfig-based authentication
- Output a copy-paste ready bearer token for direct API access

## Manual Setup

## Prerequisites

- Access to the target cluster (the cluster you want to expose via GraphQL)
- Access to the management cluster (the cluster where the gateway runs)
- ClusterAccess CRDs installed in the management cluster
- Target cluster kubeconfig file

## Step 1: Create ServiceAccount with Admin Access in Target Cluster

```bash
# Switch to target cluster
export KUBECONFIG=/path/to/target-cluster-kubeconfig

# Create ServiceAccount with cluster-admin access
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubernetes-graphql-gateway-admin
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubernetes-graphql-gateway-admin-cluster-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: kubernetes-graphql-gateway-admin
  namespace: default
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-graphql-gateway-admin-token
  namespace: default
  annotations:
    kubernetes.io/service-account.name: kubernetes-graphql-gateway-admin
type: kubernetes.io/service-account-token
EOF
```

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

## Step 5: Create Secrets and ClusterAccess in Management Cluster

Create a file called `my-cluster-access.yaml`:

```yaml
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
# Secret containing admin kubeconfig for target-cluster
apiVersion: v1
kind: Secret
metadata:
  name: my-target-cluster-admin-kubeconfig
  namespace: default
type: Opaque
data:
  kubeconfig: BASE64_ENCODED_TARGET_KUBECONFIG_HERE

---
# ClusterAccess resource for target-cluster
apiVersion: gateway.platform-mesh.io/v1alpha1
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
    kubeconfigSecretRef:
      name: my-target-cluster-admin-kubeconfig
      namespace: default
```

To encode the kubeconfig:
```bash
cat /path/to/target-cluster-kubeconfig | base64
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
kubectl get secret my-target-cluster-admin-kubeconfig my-target-cluster-ca
```

## Step 8: Test with Listener

```bash
export ENABLE_KCP=false
export GATEWAY_SHOULD_IMPERSONATE=false
export LOCAL_DEVELOPMENT=false
export KUBECONFIG=/path/to/management-cluster-kubeconfig
task listener
```

## Key Points

- **ServiceAccount**: Created in the target cluster with cluster-admin access for full permissions
- **Kubeconfig**: Stored as a secret in the management cluster for authentication
- **CA Certificate**: Essential for TLS verification - without it you'll get certificate errors
- **Server URL**: Must match exactly from the target cluster's kubeconfig
- **Path**: Becomes the schema filename (e.g., `my-target-cluster`) in `bin/definitions/`
- **Secrets**: Keep them in the same namespace as the ClusterAccess resource

The listener will detect the ClusterAccess resource and generate schema files with metadata that the gateway can use to access the target cluster. 
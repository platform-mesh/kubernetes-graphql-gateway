# ClusterAccess

This flow is designed to support multiple standard Kubernetes clusters. Each ClusterAccess resource represents a single standard Kubernetes cluster.

## Glossary
- `management cluster` - Main (root) cluster that contains ClusterAccess resources. KUBECONFIG should point to this cluster.
- `target cluster` - The cluster we want to generate OpenAPI schema for.

### Quick Setup

```bash
# Create ClusterAccess resource with admin token authentication
./hack/create-clusteraccess.sh --target-kubeconfig ~/.kube/platform-mesh-config --management-kubeconfig ~/.kube/platform-mesh-config
```

This document explains how the kubeconfig storage and usage flow works in ClusterAccess mode (`ENABLE_KCP=false`).

## Overview

The system is designed to work in the following way:

1. **ClusterAccess Resources**: Store connection information for target clusters, including kubeconfig data
2. **Listener**: Processes ClusterAccess resources and generates schema files with embedded connection metadata
3. **Gateway**: Reads schema files and uses embedded metadata to connect to specific clusters

## Flow Details

### 1. ClusterAccess Resource Creation

```yaml
apiVersion: gateway.platform-mesh.io/v1alpha1
kind: ClusterAccess
metadata:
  name: my-target-cluster
spec:
  path: my-target-cluster  # Used as schema filename
  host: https://my-cluster-api-server:6443
  auth:
    kubeconfigSecretRef:
      name: my-cluster-kubeconfig
      namespace: default
  ca:
    secretRef:
      name: my-cluster-ca
      namespace: default
      key: ca.crt
```

### 2. Listener Processing

When running in ClusterAccess mode:

```bash
export ENABLE_KCP=false
```

The listener:
- Uses the `ClusterAccessReconciler` 
- Watches for ClusterAccess resources
- For each ClusterAccess:
  - Extracts cluster connection info (host, auth, CA)
  - Connects to the target cluster to discover API schema
  - Generates schema JSON with Kubernetes API definitions
  - Injects `x-cluster-metadata` with connection information
  - Saves schema file to `definitions/{cluster-name}.json`

### 3. Schema File Structure

Generated schema files contain:

```json
{
  "definitions": {
    // ... Kubernetes API definitions
  },
  "x-cluster-metadata": {
    "host": "https://my-cluster-api-server:6443",
    "path": "my-target-cluster",
    "auth": {
      "type": "kubeconfig",
      "kubeconfig": "base64-encoded-kubeconfig"
    },
    "ca": {
      "data": "base64-encoded-ca-cert"
    }
  }
}
```

### 4. Gateway Usage

When running the gateway in ClusterAccess mode:

```bash
export ENABLE_KCP=false
export GATEWAY_SHOULD_IMPERSONATE=false
```

The gateway:
- Watches the definitions directory for schema files
- For each schema file:
  - Reads the `x-cluster-metadata` section
  - Creates a `rest.Config` using the embedded connection info
  - Establishes a Kubernetes client connection to the target cluster
  - Serves GraphQL API at `/{cluster-name}/graphql`
- **Does NOT require KUBECONFIG** - all connection info comes from schema files

## Testing

Use the provided integration test:

```bash
./scripts/test-clusteraccess-integration.sh
```

This test verifies the end-to-end flow with kubeconfig-based authentication.

## Troubleshooting

### Schema files not generated
- Check that ClusterAccess CRD is installed: `kubectl apply -f config/crd/`
- Verify ClusterAccess resources exist: `kubectl get clusteraccess`
- Check listener logs for connection errors to target clusters

### Gateway not connecting to clusters
- Verify schema files contain `x-cluster-metadata`
- Check gateway logs for authentication errors
- Ensure credentials in secrets are valid

### Connection errors
- Verify target cluster URLs are accessible
- Check CA certificates are correct
- Validate authentication credentials have required permissions 
# Listener

The Listener component is responsible for watching Kubernetes clusters and generating OpenAPI specifications for discovered resources.
It stores these specifications in a directory, which can then be used by the [Gateway](./gateway.md) component to expose them as GraphQL endpoints.

In **KCP mode**, it creates a separate file for each KCP workspace. In **ClusterAccess mode**, it creates a file for each ClusterAccess resource representing a target cluster.

The Gateway watches this directory for changes and updates the GraphQL schema accordingly.

## Packages Overview

### Reconciler (`listener/reconciler/`)

Contains reconciliation logic for different operational modes:

#### ClusterAccess Reconciler (`reconciler/clusteraccess/`)
- Watches ClusterAccess resources in the management cluster
- Connects to target clusters using kubeconfig-based authentication
- Generates schema files with embedded cluster connection metadata
- Injects `x-cluster-metadata` into schema files for gateway consumption

## Custom OpenAPI properties used by the project

The project uses two categories of OpenAPI extensions:

- Kubernetes-standard extensions (consumed during schema building):
  - `x-kubernetes-group-version-kind`
  - `x-kubernetes-categories`
  - `x-kubernetes-scope`
  These are provided by kube-openapi and are not defined by this project.

- Project-defined extension (produced by the listener and consumed by the gateway):
  - `x-cluster-metadata`: carries connection information for the target cluster.

### `x-cluster-metadata`

The listener injects `x-cluster-metadata` into each schema file so the gateway can establish a connection to the referenced cluster without any external configuration.

Structure (minimal shape used by the gateway):

```
"x-cluster-metadata": {
  "host": "https://<api-server>",
  "auth": { /* one of: token | kubeconfig | clientCert */ },
  "ca": { "data": "<base64-PEM>" }
}
```

Notes:
- The `host` field is required.
- Exactly one authentication method should be provided under `auth`:
  - `{"type":"token","token":"<base64>"}`
  - `{"type":"kubeconfig","kubeconfig":"<base64>"}`
  - `{"type":"clientCert","certData":"<base64>","keyData":"<base64>"}`
- The `ca.data` field is optional but recommended; if omitted and `auth` contains a kubeconfig, the listener attempts to extract the CA from that kubeconfig automatically.

Why we keep it simple:
- All information needed to connect is either intrinsic to the target cluster (host, CA) or already available via selected auth material. We avoid injecting duplicate or derivable data.
- We do not replicate routing information in metadata; routing is defined by where the file is stored and how the gateway is addressed.

#### KCP Reconciler (`reconciler/kcp/`)
- Watches APIBinding resources in KCP workspaces
- Discovers virtual workspaces and their API resources
- Handles cluster path resolution for KCP workspace hierarchies
- Generates schema files for each workspace

### Packages (`listener/pkg/`)

Supporting packages for schema generation:

#### API Schema (`pkg/apischema/`)
- Builds OpenAPI specifications from Kubernetes API resources
- Resolves Custom Resource Definitions (CRDs)
- Converts Kubernetes API schemas to OpenAPI format
- Handles resource relationships and dependencies

#### Workspace File (`pkg/workspacefile/`)
- Manages reading and writing schema files to the definitions directory
- Handles file I/O operations for schema persistence

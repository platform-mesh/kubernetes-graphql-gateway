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

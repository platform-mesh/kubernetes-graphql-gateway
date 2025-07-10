# kubernetes-graphql-gateway

![Build Status](https://github.com/openmfp/kubernetes-graphql-gateway/actions/workflows/pipeline.yml/badge.svg)
[![REUSE status](
https://api.reuse.software/badge/github.com/openmfp/kubernetes-graphql-gateway)](https://api.reuse.software/info/github.com/openmfp/kubernetes-graphql-gateway)

The goal of this library is to provide a reusable and generic way of exposing k8s resources from within a cluster using GraphQL.
This enables UIs that need to consume these objects to do so in a developer-friendly way, leveraging a rich ecosystem.

## Overview

This repository contains two main components:
- [Listener](./docs/listener.md): watches a cluster and stores its openAPI spec in a directory.
- [Gateway](./docs/gateway.md): exposes the openAPI spec as a GraphQL endpoints.

## MultiCluster Support

The system supports three modes of operation:

1. **Single Cluster** (`ENABLE_KCP=false`, `MULTICLUSTER=false`): Gateway connects to the same cluster as the listener
2. **KCP Mode** (`ENABLE_KCP=true`): Designed for KCP-based multi-cluster scenarios  
3. **MultiCluster Mode** (`ENABLE_KCP=false`, `MULTICLUSTER=true`): Gateway connects to multiple external clusters via ClusterAccess resources

### MultiCluster with ClusterAccess

In MultiCluster mode, the system uses ClusterAccess resources to store kubeconfig data and connection information. The listener processes these resources and embeds connection metadata into schema files, which the gateway then uses to establish cluster-specific connections.

For complete setup instructions, see:
- [ClusterAccess documentation](./docs/clusteraccess.md) - Manual setup
- [MultiCluster Kubeconfig Flow](./docs/multicluster-kubeconfig-flow.md) - Detailed flow explanation

### Quick Setup Scripts

```bash
# Create ClusterAccess with secure token authentication  
./scripts/create-clusteraccess.sh --target-kubeconfig ~/.kube/prod-config

# Test end-to-end integration
./scripts/test-clusteraccess-integration.sh
```

### Gateway Requirements

- **Single Cluster Mode**: Requires KUBECONFIG to connect to the local cluster
- **KCP Mode**: Requires KUBECONFIG to connect to KCP management cluster  
- **MultiCluster Mode**: Does NOT require KUBECONFIG - gets all connection info from schema files

## Authorization

All information about authorization can be found in the [authorization](./docs/authorization.md) section.

## Quickstart

If you want to get started quickly, you can follow the [quickstart guide](./docs/quickstart.md).

## Contributing
Please refer to the [contributing](./docs/contributing.md) section for instructions on how to contribute to OpenMFP.

## Releasing

The release is performed automatically through a GitHub Actions Workflow. The resulting website will be available as Github page under the following URL: https://openmfp.github.io/openmfp.org/

## Security / Disclosure

If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/openmfp/openmfp.org/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Licensing

Copyright 2025 SAP SE or an SAP affiliate company and OpenMFP contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/openmfp/openmfp.org).

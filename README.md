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

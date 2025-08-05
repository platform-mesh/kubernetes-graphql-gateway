# Quick Start

This page shows you how to get started to use the GraphQL Gateway for Kubernetes.

## Prerequisites
- Installed [Golang](https://go.dev/doc/install)
- Installed [Taskfile](https://taskfile.dev/installation)
- A Kubernetes cluster to connect to (some options below)
  - Option A: Prexisting standard Kuberentes cluster
  - Option B: Preexisting Kuberentes cluster that is available through [Kuberentes Control Plane (KCP)](https://docs.kcp.io/kcp/main/setup/quickstart/)
  - Option C: Create your own locally running Kuberentes cluster using [kind](https://kind.sigs.k8s.io/)
- Clone the `kubernetes-graphql-gateway` repository and change to the root directory
```shell
git clone git@github.com:openmfp/kubernetes-graphql-gateway.git && cd kubernetes-graphql-gateway
```  


## Setup the environment:
```shell
# this will disable authorization
export LOCAL_DEVELOPMENT=true 
# kcp is enabled by default, in case you want to run it against a standard Kubernetes cluster
export ENABLE_KCP=false
# you must point to the config of the cluster you want to run against
export KUBECONFIG=YOUR_KUBECONFIG_PATH
```


## Running the Listener

Make sure you have done steps from the [setup section](#setup-the-environment).

```shell
task listener
```
This will create a directory `./bin/definitions` and start watching the cluster APIs for changes.
In that directory a file will be created for each workspace in KCP or a standard Kubernetes cluster.
The file will contain the API definitions for the resources in that workspace.

## Running the Gateway

Make sure you have done steps from the [setup section](#setup-the-environment).

In the root directory of the `kubernetes-graphql-gateway` repository, open a new shell and run the Graphql gateway as follows:
```shell
task gateway
```

The gateway will watch the `./bin/definitions` directory for changes and update the schema accordingly.
It will also spawn a GraphQL playground server that allows you to execute GraphQL queries via your browser.
Check the console output to get the localhost URL of the GraphQL playground.

## First Steps and Basic Examples

As said above, the GraphQL Gateway allows you do CRUD operations on any of the Kubernetes resources in the cluster.
You may checkout the following copy & paste examples to get started:
- Examples on [CRUD operations on ConfigMaps](./configmap_queries.md).
- Examples on [CRUD operations on Pods](./pod_queries.md).
- Subscribe to events using [Subscriptions](./subscriptions.md).
- There are also [Custom Queries](./custom_queries.md) that go beyond what.


## Authorization with Remote Kuberenetes Clusters

If you run the GraphQL gateway with an shell environment that sets `LOCAL_DEVELOPMENT=false`, you need to add the `Authorization` header to any of your GraphQL queries you are executing.
When using the GraphQL playground, you can add the header in the `Headers` section of the playground user interface like so:
```shell
{
  "Authorization": "YOUR_TOKEN"
}
```

## Working with Dotted Keys (Labels, Annotations, NodeSelector, MatchLabels)

Kubernetes extensively uses dotted keys (e.g., `app.kubernetes.io/name`) in labels, annotations, and other fields. Since GraphQL doesn't support dots in field names, the gateway provides a special `StringMapInput` scalar.

**Key Points:**
- **Input**: Use variables with arrays of `{key, value}` objects  
- **Output**: Returns direct maps like `{"app.kubernetes.io/name": "my-app"}`
- **Supported fields**: `metadata.labels`, `metadata.annotations`, `spec.nodeSelector`, `spec.selector.matchLabels`, and their nested equivalents in templates

**Quick Example:**
```graphql
mutation createPodWithLabels($labels: StringMapInput) {
  core {
    createPod(
      namespace: "default"
      object: {
        metadata: {
          name: "my-pod"
          labels: $labels
        }
        spec: {
          containers: [...]
        }
      }
    ) {
      metadata {
        labels  # Returns: {"app.kubernetes.io/name": "my-app"}
      }
    }
  }
}
```

**Variables:**
```json
{
  "labels": [
    {"key": "app.kubernetes.io/name", "value": "my-app"},
    {"key": "environment", "value": "production"}
  ]
}
```

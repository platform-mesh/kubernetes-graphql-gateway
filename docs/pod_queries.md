# Pod Queries and Mutations

This page shows you examples queries and mutations for GraphQL to perform operations on the `Pod` resource. 
For questions on how to execute them, please find our [Quick Start Guide](./quickstart.md).

## Create a Pod:
```shell
mutation {
  core {
    createPod(
      namespace: "default",
      object: {
        metadata: {
          name: "my-new-pod",
          labels: {
            app: "my-app"
          }
        }
        spec: {
          containers: [
            {
              name: "nginx-container"
              image: "nginx:latest"
              ports: [
                {
                  containerPort: 80
                }
              ]
            }
          ]
          restartPolicy: "Always"
        }
      }
    ) {
      metadata {
        name
        namespace
        labels
      }
      spec {
        containers {
          name
          image
          ports {
            containerPort
          }
        }
        restartPolicy
      }
      status {
        phase
      }
    }
  }
}
```

## Get the Created Pod:
```shell
query {
  core {
    Pod(name:"my-new-pod", namespace:"default") {
      metadata {
        name
      }
      spec{
        containers {
          image
          ports {
            containerPort
          }
        }
      }
    }
  }
}
```

## Delete the Created Pod:
```shell
mutation {
  core {
    deletePod(
      namespace: "default",
      name: "my-new-pod"
    )
  }
}
```

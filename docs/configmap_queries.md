# ConfigMap Queries

This page shows you examples queries and mutations for GraphQL to perform operations on the `ConfigMap` resource. 
For questions on how to execute them, please find our [Quick Start Guide](./quickstart.md).

## Create a ConfigMap:
```shell
mutation {
  core {
    createConfigMap(
      namespace: "default",
      object: {
        metadata: {
          name: "example-config"
        },
        data: { key: "val" }
      }
    ) {
      metadata {
        name
      }
      data
    }
  }
}
```

## List ConfigMaps:
```shell
{
  core {
    ConfigMaps {
      metadata {
        name
      }
      data
    }
  }
}
```

## Get a ConfigMap:
```shell
{
  core {
    ConfigMap(name: "example-config", namespace: "default") {
      metadata {
        name
      }
      data
    }
  }
}
```

## Update a ConfigMap:
```shell
mutation {
  core {
    updateConfigMap(
      name:"example-config"
      namespace: "default",
      object: {
        data: { key: "new-value" }
      }
    ) {
      metadata {
        name
        namespace
      }
      data
    }
  }
}
```

## Delete a ConfigMap:
```shell
mutation {
  core {
    deleteConfigMap(
      name: "example-config", 
      namespace: "default"
    )
  }
}
```

# Custom Queries

This page contains custom queries that **differ** from standard `kubectl` commands.  
For instructions on how to execute them, please refer to our [Quick Start Guide](./quickstart.md).

## typeByCategory

`typeByCategory` returns a list of resource **types**, grouped **by** a specified **category**.

Categories can be found in the  
[CRD spec](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#categories)  
and in the [ApiResourceSchema](https://github.com/kcp-dev/kcp/blob/7d0b5b21c51eac3f3666316956049e59e499f471/sdk/apis/apis/v1alpha1/types_apiresourceschema.go#L61)  
under the `categories` field.

Each entry in the list may include the following fields: `group`, `version`, `kind`, and `scope` (either `Cluster` or `Namespaced`).

```shell
{
  typeByCategory(name: "categoryName") {
    group
    version
    kind
    scope
  }
}
```

## applyYaml

`applyYaml` applies a Kubernetes resource from raw YAML using "apply" semantics:
- Creates the resource if it doesn't exist
- Updates the resource if it already exists

This is useful when you have YAML manifests and want to apply them directly without constructing type-specific GraphQL mutations.

### Arguments

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `yaml` | String! | Yes | YAML representation of the Kubernetes resource |
| `dryRun` | Boolean | No | If true, validates without persisting changes |

### Example

```graphql
mutation {
  applyYaml(yaml: """
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  key: value
""")
}
```

With dry run:

```graphql
mutation {
  applyYaml(
    yaml: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n  namespace: default"
    dryRun: true
  )
}
```

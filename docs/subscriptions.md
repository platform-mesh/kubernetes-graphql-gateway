# Subscriptions

To subscribe to events, you should use the SSE (Server-Sent Events) protocol.
Since GraphiQL doesn't support SSE (see [Quick Start Guide](./quickstart.md)), we won't use the GraphQL playground to execute the queries.
Instead, we use the `curl` command line tool to execute the queries.

## Prerequisites
```shell
export GRAPHQL_URL=http://localhost:8080/root/graphql # Update with your actual GraphQL endpoint
export AUTHORIZATION_TOKEN="Bearer <your-token>" # Update with your token if LOCAL_DEVELOPMENT=false
```

## Parameters
- `subscribeToAll`: If true, any field change will be sent to the client.
Otherwise, only fields defined within the `{}` brackets will be listened to.

**Note:** Only fields specified in `{}` brackets will be returned, even if `subscribeToAll: true`.

## Naming scheme (subscriptions)

Subscriptions are flat but versioned. To align with the removal of the artificial core group:

- Core resources use `v<version>_<resource>`.
  - All ConfigMaps (core/v1): `v1_configmaps`
  - Single ConfigMap: `v1_configmap`
- Non‑core (CRDs) remain `<group>_<version>_<resource>` (dots in group replaced with underscores):
  - Accounts (openmfp.org/v1alpha1): `openmfp_org_v1alpha1_accounts` and `openmfp_org_v1alpha1_account`

Group and version match the hierarchical query/mutation path. For example, the corresponding list query is:
```
query {
  v1 { ConfigMaps { resourceVersion items { metadata { name } } } }
}
```

## Event Envelope

Subscriptions stream only changes as events. Each event is an object with:

- `type`: An enum (`WatchEventType`) with values `ADDED`, `MODIFIED`, or `DELETED` (aligned with Kubernetes `watch.EventType`).
- `object`: The resource object for `ADDED`, `MODIFIED`, and `DELETED` events. On `DELETED`, it contains the last known object (at least `metadata` such as `name`, `namespace`, `uid`, and `resourceVersion`) so clients can identify and remove it from their cache.

Behavior alignment with Kubernetes watch:

- If you start a subscription WITHOUT providing `resourceVersion`, the gateway will emit an `ADDED` event for every existing object first (initial sync), then continue streaming subsequent changes.
- If you provide a `resourceVersion`, the gateway will stream only events that happened after that version.

Recommendation: To avoid the initial burst of `ADDED` events on large collections, first issue a list query to obtain the list `resourceVersion` (returned alongside the items), then open the subscription with that `resourceVersion`.

Example variables:

```
{"resourceVersion": "12345"}
```

## Subscribe to the ConfigMap Resource

ConfigMap is present in both KCP and standard Kubernetes clusters, so we can use it right away without any additional setup.

After subscription, you can run mutations from [ConfigMap Queries](./configmap_queries.md) to see the changes in the subscription.

### Subscribe to a Change of a Data Field in All ConfigMaps
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { v1_configmaps { type object { metadata { name resourceVersion } data } } }"}' \
  $GRAPHQL_URL
```
### Subscribe to a Change of a Data Field in a Specific ConfigMap

```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { v1_configmap(name: \"example-config\", namespace: \"default\") { type object { metadata { name resourceVersion } data } } }"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of All Fields in a Specific ConfigMap

```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { v1_configmap(name: \"example-config\", namespace: \"default\", subscribeToAll: true) { type object { metadata { name resourceVersion } } } }"}' \
  $GRAPHQL_URL
```

## Subscribe to the Account Resource

If you have the [Account](https://github.com/openmfp/account-operator/tree/main/config) CRD registered in your cluster, you can use the following queries:

After subscription, you can run mutations against accounts to see the changes in the subscription.

### Subscribe to a Change of a DisplayName Field in All Accounts
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { openmfp_org_v1alpha1_accounts { type object { spec { displayName } metadata { resourceVersion } } } }"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of a DisplayName Field in a Specific Account
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { openmfp_org_v1alpha1_account(name: \"root-account\") { type object { spec { displayName } metadata { resourceVersion } } } }"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of All Fields in a Specific Account
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { openmfp_org_v1alpha1_account(name: \"root-account\", subscribeToAll: true) { type object { metadata { name resourceVersion } } } }"}' \
  $GRAPHQL_URL
```

### Starting from a specific resourceVersion

To start from a known `resourceVersion` collected from a prior list query, pass the `resourceVersion` argument:

```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription ($rv: String) { v1_configmaps(resourceVersion: $rv) { type object { metadata { name resourceVersion } } } }", "variables": {"rv":"12345"}}' \
  $GRAPHQL_URL
```

## Plural list queries return pagination metadata

Plural queries (returning multiple items) now return an object with the list metadata and items instead of a bare array. The object contains:

- `resourceVersion: String` — list resourceVersion for starting subscriptions
- `continue: String` — pagination token to retrieve the next page
- `remainingItemCount: Int` — hint of how many items remain on the server
- `items: [Item!]!` — the current page of items

Example for ConfigMaps:

```shell
curl \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{
    "query": "query { v1 { ConfigMaps { resourceVersion continue remainingItemCount items { metadata { name resourceVersion } data } } } }"
  }' \
  $GRAPHQL_URL
```

Use the top-level `resourceVersion` from the list to start subscriptions without receiving the initial `ADDED` events burst.

### Pagination arguments

Plural list fields accept optional pagination arguments:

- `limit: Int` — maximum number of items to return (server may return fewer)
- `continue: String` — pass the token from a previous page to continue the listing

Example using pagination:

```shell
curl \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{
    "query": "query { v1 { ConfigMaps(limit: 10) { continue items { metadata { name } } } } }"
  }' \
  $GRAPHQL_URL
```

Then use the returned `continue` token to fetch the next page:

```shell
curl \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{
    "query": "query($tok: String) { v1 { ConfigMaps(limit: 10, continue: $tok) { continue remainingItemCount items { metadata { name } } } } }",
    "variables": {"tok": "<token-from-previous-response>"}
  }' \
  $GRAPHQL_URL
```

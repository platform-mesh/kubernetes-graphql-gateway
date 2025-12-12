# Subscriptions

To subscribe to events, you should use the SSE (Server-Sent Events) protocol.
Since GraphQL playground doesn't support SSE (see [Quick Start Guide](./quickstart.md)), we won't use the GraphQL playground to execute the queries.
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

### Event Envelope

Subscriptions stream only changes as events. Each event is an object with:

- `type`: An enum (`WatchEventType`) with values `ADDED`, `MODIFIED`, or `DELETED` (aligned with Kubernetes `watch.EventType`).
- `object`: The resource object for `ADDED`, `MODIFIED`, and `DELETED` events. On `DELETED`, it contains the last known object (at least `metadata` such as `name`, `namespace`, `uid`, and `resourceVersion`) so clients can identify and remove it from their cache.

Behavior alignment with Kubernetes watch:

- If you start a subscription WITHOUT providing `resourceVersion`, the gateway will emit an `ADDED` event for every existing object first (initial sync), then continue streaming subsequent changes.
- If you provide a `resourceVersion`, the gateway will stream only events that happened after that version.

Recommendation: To avoid the initial burst of `ADDED` events on large collections, first issue a list query to obtain `metadata.resourceVersion`, then open the subscription with that `resourceVersion`.

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
  -d '{"query": "subscription { core_configmaps { type object { metadata { name resourceVersion } data } } }"}' \
  $GRAPHQL_URL
```
### Subscribe to a Change of a Data Field in a Specific ConfigMap

```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_configmap(name: \"example-config\", namespace: \"default\") { type object { metadata { name resourceVersion } data } } }"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of All Fields in a Specific ConfigMap

```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_configmap(name: \"example-config\", namespace: \"default\", subscribeToAll: true) { type object { metadata { name resourceVersion } } } }"}' \
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
  -d '{"query": "subscription { core_openmfp_org_accounts { type object { spec { displayName } metadata { resourceVersion } } } }"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of a DisplayName Field in a Specific Account
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_openmfp_org_account(name: \"root-account\") { type object { spec { displayName } metadata { resourceVersion } } } }"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of All Fields in a Specific Account
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_openmfp_org_account(name: \"root-account\", subscribeToAll: true) { type object { metadata { name resourceVersion } } } }"}' \
  $GRAPHQL_URL
```

### Starting from a specific resourceVersion

To start from a known `resourceVersion` collected from a prior query, pass the `resourceVersion` argument:

```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription ($rv: String) { core_configmaps(resourceVersion: $rv) { type object { metadata { name resourceVersion } } } }", "variables": {"rv":"12345"}}' \
  $GRAPHQL_URL
```

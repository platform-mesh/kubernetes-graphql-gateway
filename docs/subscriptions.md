# Subscriptions

To subscribe to events, you should use the SSE (Server-Sent Events) protocol.
Since GraphQL playground doesn't support (see [Quick Start Guide](./quickstart.md)) we won't use the GraphQL playground to execute the queries.
Instead we use the `curl` command line tool to execute the queries.

## Prerequisites
```shell
export GRAPHQL_URL=http://localhost:8080/root/graphql # update with your actual GraphQL endpoint
export AUTHORIZATION_TOKEN=<your-token> # update this with your token, if LOCAL_DEVELOPMENT=false
```

## Parameters
- `subscribeToAll`: if true, any field change will be sent to the client.
Otherwise, only fields defined within the `{}` brackets will be listened to.

Please note that only fields specified in `{}` brackets will be returned, even if `subscribeToAll: true`

### Return parameters

- `data` field contains the data returned by the subscription.
- `errors` field contains the errors if any occurred during the subscription.

## Subscribe to the ConfigMap Resource

ConfigMap is present in both KCP and standard Kubernetes clusters, so we can use it right away without any additional setup.

After subscription, you can run mutations from [Configmap Queries](./configmap_queries.md) to see the changes in the subscription.

### Subscribe to a Change of a Data Field in All ConfigMaps:
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_configmaps { metadata { name } data }}"}' \
  $GRAPHQL_URL
```
### Subscribe to a Change of a Data Field in a Specific ConfigMap:

```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_configmap(name: \"example-config\", namespace: \"default\") { metadata { name } data }}"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of All Fields in a Specific ConfigMap:

Please note that only fields specified in `{}` brackets will be returned, even if `subscribeToAll: true`

```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_configmap(name: \"example-config\", namespace: \"default\", subscribeToAll: true) { metadata { name } }}"}' \
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
  -d '{"query": "subscription { core_openmfp_org_accounts { spec { displayName }}}"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of a DisplayName Field in a Specific Account
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_openmfp_org_account(name: \"root-account\") { spec { displayName }}}"}' \
  $GRAPHQL_URL
```

### Subscribe to a Change of a DisplayName Field in All Accounts
```shell
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTHORIZATION_TOKEN" \
  -d '{"query": "subscription { core_openmfp_org_account(name: \"root-account\", subscribeToAll: true) { metadata { name } }}"}' \
  $GRAPHQL_URL
```

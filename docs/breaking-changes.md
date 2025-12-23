# Breaking Changes and Migration Guide

This document summarizes recent breaking changes and how to migrate your clients.

## Queries and Mutations: grouped by API group and version

GraphQL queries and mutations are now organized under `group → version → resources`.

- Core group examples:
  - Before: `core { ConfigMaps { ... } }`
  - Now: `core { v1 { ConfigMaps { ... } } }`

- Non-core groups (dots replaced by underscores):
  - Before: `openmfp_org { Accounts { ... } }`
  - Now: `openmfp_org { v1alpha1 { Accounts { ... } } }`

Plural list fields now return a wrapper object with pagination metadata:

```graphql
query {
  core {
    v1 {
      ConfigMaps {
        resourceVersion
        continue
        remainingItemCount
        items { metadata { name namespace resourceVersion } }
      }
    }
  }
}
```

## Subscriptions: flat and versioned field names

Subscriptions remain flat but now include the version in the field name: `<group>_<version>_<resource>`.

- Core examples: `core_v1_configmaps`, `core_v1_configmap`
- Non-core examples: `openmfp_org_v1alpha1_accounts`, `openmfp_org_v1alpha1_account`

Subscriptions use Server‑Sent Events (SSE). GraphiQL/Playground typically do not support SSE subscriptions. Use curl/Postman/Insomnia or a custom EventSource client.

Example:

```sh
curl \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "subscription { core_v1_configmaps { type object { metadata { name namespace resourceVersion } } } }"
  }' \
  $GRAPHQL_URL
```

To start from a known list `resourceVersion`, fetch it via a list query and pass it to the subscription as an argument.

## Removed legacy names

- Legacy flat query/mutation names have been removed.
- Old unversioned subscription names have been removed.

Please update your client queries accordingly. See detailed examples:

- [ConfigMap queries](./configmap_queries.md)
- [Pod queries](./pod_queries.md)
- [Subscriptions](./subscriptions.md)
- [Quick Start](./quickstart.md)

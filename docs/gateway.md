# Gateway

The Gateway expects a directory input to watch for files containing OpenAPI specifications with resources.
Each file in that directory corresponds to a KCP workspace or a target cluster (via ClusterAccess).
It creates a separate URL for each file, like `/<workspace-name>/graphql` which will be used to query the resources of that workspace or cluster.
It watches for changes in the directory and updates the schema accordingly.

So, if there are two files in the directory - `root` and `root:alpha`, then we will have two URLs:
- `http://localhost:3000/root/graphql`
- `http://localhost:3000/root:alpha/graphql`

Open the URL in the browser and you will see the GraphQL playground.

See example queries in the [Queries Examples](./quickstart.md#first-steps-and-basic-examples) section.

## Packages Overview

### Manager (`gateway/manager/`)

Manages the gateway lifecycle and cluster connections:
- **Watcher**: Watches the definitions directory for schema file changes
- **Target Cluster**: Manages cluster registry and GraphQL endpoint routing
- **Round Tripper**: Handles HTTP request routing and authentication

### Schema (`gateway/schema/`)

Converts OpenAPI specifications into GraphQL schemas:
- Generates GraphQL types from OpenAPI definitions
- Handles custom queries and relations between resources
- Manages scalar type mappings

### Resolver (`gateway/resolver/`)

Executes GraphQL queries against Kubernetes clusters:
- Resolves GraphQL queries to Kubernetes API calls
- Handles subscriptions for real-time updates
- Processes query arguments and filters

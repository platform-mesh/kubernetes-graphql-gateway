# Gateway

The Gateway expects a directory input to watch for files containing OpenAPI specifications with resources.
Each file in that directory will correspond to a KCP workspace (or API server).
It creates a separate URL for each file, like `/<workspace-name>/graphql` which will be used to query the resources of that workspace.
It watches for changes in the directory and update the schema accordingly.

So, if there are two files in the directory - `root` and `root:alpha`, then we will have two URLs:
- `http://localhost:3000/root/graphql`
- `http://localhost:3000/root:alpha/graphql`

Open the URL in the browser and you will see the GraphQL playground.

See example queries in the [Queries Examples](./quickstart.md#first-steps-and-basic-examples) section.

## Packages Overview

### Workspace Manager
Holds the logic for watching a directory, triggering schema generation, and binding it to an HTTP handler.

### Schema

Is responsible for the conversion from OpenAPI spec into the GraphQL schema.

### Resolver

Holds the logic of interaction with the cluster.

> [!WARNING]
> This repository is under construction and not yet ready for public consumption. Please check back later for updates.


# crd-gql-gateway

The goal of this library is to provide a reusable and generic way of exposing Custom Resource Definitions from within a cluster using GraphQL. This enables UIs that need to consume these objects to do so in a developer-friendly way, leveraging a rich ecosystem.

For each registered CRD, the gateway provides the following:

- A list query that allows the client to request a list of specific CRDs based on label selectors and/or namespace.
- A query for an individual resource.
- Create/Update/Delete mutations.
- A list subscription type that opens a watch and serves the client live updates from CRDs within the cluster.

Additionally, the gateway ensures that client requests are authorized to perform the desired actions using `SubjectAccessReview`, which ensures proper authorization.

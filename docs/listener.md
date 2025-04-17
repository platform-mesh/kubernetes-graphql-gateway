# Listener

The Listener component is responsible for watching a Kubernetes cluster and generating OpenAPI specifications for discovered resources.
It stores these specifications in a directory, which can then be used by the [Gateway](./gateway.md) component to expose them as GraphQL endpoints.
The Listener creates a separate file for each KCP workspace in the specified directory. 
The Gateway will then watch this directory for changes and update the GraphQL schema accordingly.

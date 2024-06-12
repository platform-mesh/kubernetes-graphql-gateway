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

## Usage

The goal is to provide a reusable library that can serve Custom Resources from any cluster without being specifically tied to a cluster/setup. The library is also able to dynamically infer which custom resource to expose based on the registered types in the [`runtime.Scheme`](https://pkg.go.dev/k8s.io/apimachinery/pkg/runtime#Scheme), which need to be registered anyway in order to get a functioning `controller-runtime` client.

To get started, you can consume the library in the following way:

#### 1. Create a `controller-runtime.Client` however you like

Please make sure to also include the `k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1` and the `k8s.io/api/authorization/v1` types, so the library can create `SubjectAccessReviews` and load `CustomResourceDefinitions`.

```go
schema := runtime.NewScheme()
apiextensionsv1.AddToScheme(schema)
authzv1.AddToScheme(schema)
```

After you set up the generally needed schema, feel free to add the types of any CRD that is available in your target cluster to the scheme. For every type that you register, there will be a set of queries, mutations, and subscriptions generated to expose your type via the gateway.

```go
package main

import (
    // ...
    accountv1alpha1 "github.com/openmfp/account-operator/api/v1alpha1"
    // ...
)

func main() {
    // ...
    accountv1alpha1.AddToScheme(schema)

    cfg := controllerruntime.GetConfigOrDie()

    cl, err := client.NewWithWatch(cfg, client.Options{
        Scheme: schema,
    })
    if err != nil {
        panic(err)
    }
}
```

#### 2. Pass the client to the gateway library and see your resource being exposed :rocket:

```go
gqlSchema, err := gateway.New(cmd.Context(), gateway.Config{
    Client: cl,
})
if err != nil {
    return err
}

http.Handle("/graphql", gateway.Handler(gateway.HandlerConfig{
    Config: &handler.Config{
        Schema:     &gqlSchema,
        Pretty:     true,
        Playground: true,
    },
    UserClaim: "mail",
}))
```

You can expose the `gateway.Handler()` via the normal `net/http` package.

It takes care of serving the right protocol based on the `Content-Type` header, as it exposes the `subscriptions` via the [`SSE`](https://html.spec.whatwg.org/multipage/server-sent-events.html) standard.


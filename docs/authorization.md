# Authorization

All requests must contain an `Authorization` header with a valid Bearer token by default:
```shell
{
    "Authorization": "Bearer $YOUR_TOKEN"
}
```
You can disable authorization by setting the following environment variable:
```shell
export LOCAL_DEVELOPMENT=true
```
This is useful for local development and testing purposes.

## Introspection authentication

By default, introspection requests (i.e., the requests that are made to fetch the GraphQL schema) are **not** protected by authorization.

You can protect those requests by setting the following environment variable:
```shell
export GATEWAY_INTROSPECTION_AUTHENTICATION=true
```

### Error fetching schema in documentation explorer

When the GraphiQL page is loaded, it makes a request to fetch the GraphQL schema, and there is no way to add the `Authorization` header to that request.

We have this [issue](https://github.com/openmfp/kubernetes-graphql-gateway/issues/217) open to fix this.

But for now, you can use the following workaround:
1. Open the GraphiQL page in your browser.
2. Add the `Authorization` header in the `Headers` section of the GraphiQL user interface.
3. Press the `Re-fetch GraphQL schema` button in the left sidebar (third button from the top).
4. The GraphQL schema should now be fetched, and you can use the GraphiQL interface as usual.

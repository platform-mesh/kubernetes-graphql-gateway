# Test Locally

## Run and check cluster

1. Create and run a cluster if it is not running yet.

```shell
git clone https://github.com/platform-mesh/helm-charts.git
cd helm-charts
task local-setup
```
If this task fails, you can try to run `task local-setup:iterate` to complete it.

2. Verify that the cluster is running.

Run k9s and go to `:pods`. All pods should have a status of "Running".
It may take some time before they are all ready. Possible issues include insufficient RAM and/or CPU cores. In this case, increase the limits in Docker settings.

3. In k9s, go to `:pods`, then open pod `kubernetes-graphql-gateway-...`.

Open container `kubernetes-graphql-gateway-gateway` to see the logs.
The logs must contain more than a single line (with "Starting server...").
If you see only this single line, the problem might be in the container called "kubernetes-graphql-gateway-listener".

## Build and Load Image

Use the `docker:kind` task to automatically build and load the image into your kind cluster:

```shell
task docker:kind
```

This task automatically:
- Detects the current image tag from the running deployment
- Builds the image with the correct tag
- Loads it into the kind cluster (supports both Docker and Podman)
- Restarts the deployment

You can customize the behavior with variables:
```shell
# Use podman instead of docker
task docker:kind CONTAINER_RUNTIME=podman

# Target a different kind cluster
task docker:kind KIND_CLUSTER=my-cluster

# Target a different deployment
task docker:kind DEPLOYMENT_NAME=my-deployment DEPLOYMENT_NAMESPACE=my-namespace
```

9. Once the pod is recreated, go to [https://portal.dev.local:8443](https://portal.dev.local:8443)
and check if everything works fine.

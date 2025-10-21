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

Note the image name from one of the `kubernetes-...` containers. It contains the name and the currently used version of the build, e.g.:
```
ghcr.io/platform-mesh/kubernetes-graphql-gateway:v0.75.1
```

4. Build the Docker image:
```shell
task docker
```

5. Tag the newly built image with the version used in local-setup:
```shell
docker tag ghcr.io/platform-mesh/kubernetes-graphql-gateway:latest ghcr.io/platform-mesh/kubernetes-graphql-gateway:v0.75.1
```
Use the name and version you got from the `IMAGE` column in step 3.

6. Check your cluster name:
```shell
kind get clusters
```
In this example, the cluster name is `platform-mesh`.

7. Load the new image into your kind cluster:
***Docker-based kind:***
```shell
kind load docker-image ghcr.io/platform-mesh/kubernetes-graphql-gateway:v0.75.1 -n platform-mesh
```
The argument `-n platform-mesh` targets the platform-mesh kind cluster.

***Podman-based kind:***
- Pull (or build) the image locally with Podman:
```shell
podman pull ghcr.io/platform-mesh/kubernetes-graphql-gateway:v0.75.1
```

- Save it to a tarball in OCI-archive format:
```shell
podman save --format oci-archive ghcr.io/platform-mesh/kubernetes-graphql-gateway:v0.75.1 -o kubernetes-graphql-gateway_v0.75.1.tar
```

- Load the tarball into your Podman-backed kind cluster:
```shell
kind load image-archive kubernetes-graphql-gateway_v0.75.1.tar -n platform-mesh
```

8. In k9s, go to `:pods` and delete the pod (not the container) called `kubernetes-graphql-gateway-...`.

Kubernetes will immediately recreate the pod - but this time it will use the new version of the build.

9. Once the pod is recreated, go to [https://portal.dev.local:8443](https://portal.dev.local:8443)
and check if everything works fine.

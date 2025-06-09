# Test Locally

**Warning!** This test is for those who have access to `helm-charts-priv`.

## Run and check cluster

1. Create and run a cluster if it is not running yet.

```shell
git clone https://github.com/openmfp/helm-charts-priv.git
cd helm-charts-priv
task local-setup
```

2. Verify that the cluster is running.

Run k9s, go to `:pods`. All pods must have a status of "Running".
It may take some time before they are all ready. The possible issues may be insufficient RAM and/or CPU cores. In this case, increase the limits in Docker settings.

3. In k9s, go to `:pods`, then open pod `kubernetes-graphql-gateway-...`.

Open container `kubernetes-graphql-gateway-gateway` to see the logs.
The logs must contain more than a single line (with "Starting server...").
If you see only this single line, the problem might be in the container called "kubernetes-graphql-gateway-listener".

Note the `IMAGE` column, corresponding to the two `kubernetes-...` container. It contains the name and the currently used version of the build, i.e.
```
ghcr.io/openmfp/kubernetes-graphql-gateway:v0.75.1
```

4. Build the Docker image:
```shell
task docker
```

5. Tag the newly built image with the version used in local-setup -- that image is going to be replaced with the one built on step 4.
```shell
docker tag ghcr.io/openmfp/kubernetes-graphql-gateway:latest ghcr.io/openmfp/kubernetes-graphql-gateway:v0.75.1
```
Use the name you and version got from the `IMAGE` column on step 3. Leave the version number unchanged.

6. Check your cluster name:
```shell
kind get clusters
```
In this example, the cluster name is `openmfp`.

7. Load the new image into your kind cluster:
***Docker-based kind:***
```shell
kind load docker-image ghcr.io/openmfp/kubernetes-graphql-gateway:v0.75.1 -n openmfp
```
The argument `-n openmfp` is to change the default value of the cluster name, which is `kind`.

***Podman-based kind:***
- Pull (or build) the image locally with Podman:
```shell
podman pull ghcr.io/openmfp/kubernetes-graphql-gateway:v0.75.1
```

- Save it to a tarball in OCI-archive format:
```shell
podman save --format oci-archive ghcr.io/openmfp/kubernetes-graphql-gateway:v0.75.1 -o kubernetes-graphql-gateway_v0.75.1.tar
```

- Load the tarball into your Podman-backed kind cluster:
```shell
kind load image-archive kubernetes-graphql-gateway_v0.75.1.tar -n openmfp
```

8. In k9s, go to `:pods` and delete the pod (not the container) called `kubernetes-graphql-gateway-...`.

Kubernetes will immediately recreate the pod -- but this time it will use the new version of the build.

9. Once the pod is recreated, go to [https://portal.dev.local:8443](https://portal.dev.local:8443)
and check if everything works fine.

# Air-gapped K3s Cluster

K3s is supporting air-gapped installations. This sample demonstrates how to create a K3s cluster in an air-gapped environment with cluster API k3s and Docker.

It will first build a kind node docker image with the K3s binary, the required images and scripts, following [k3s Air-Gap Install](https://docs.k3s.io/installation/airgap). Then it will create a K3s cluster with this kind node image.

K3s CAPI depends on the `alpine/socat` image to communicate with the etcd server on the node. A daemonset is created to deploy the `alpine/socat` image to all the etcd nodes. You should also download the `alpine/socat` image in the air-gapped node.

```shell
export AIRGAPPED_KIND_IMAGE=kindnode:airgapped
export AIRGAPPED_INSTALL_SCRIPT_PATH=/k3s-airgapped-install.sh
export CLUSTER_NAME=k3s-airgapped
export NAMESPACE=default
export CONTROL_PLANE_MACHINE_COUNT=1
export KUBERNETES_VERSION=v1.28.6+k3s2
export WORKER_MACHINE_COUNT=3
export AIRGAPPED_ALPINE_SOCAT_IMAGE=alpine/socat:1.8.0.0

# Prepare the alpine/socat image
docker pull $AIRGAPPED_ALPINE_SOCAT_IMAGE
docker save $AIRGAPPED_ALPINE_SOCAT_IMAGE -o alpine-socat.tar

# Build the kind node image
docker build -t $AIRGAPPED_KIND_IMAGE . \
--build-arg="K3S_VERSION=$KUBERNETES_VERSION" \
--build-arg="INSTALL_SCRIPT_PATH=$AIRGAPPED_INSTALL_SCRIPT_PATH" \
--build-arg="ALPINE_SOCAT_TARBALL_PATH=./alpine-socat.tar"

# Generate the cluster yaml
# Note that `airGapped` is set to true in `agentConfig`
clusterctl generate yaml --from ./k3s-template.yaml > k3s-airgapped.yaml

# Create the cluster to the management cluster
kubectl apply -f k3s-airgapped.yaml
```

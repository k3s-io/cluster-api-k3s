# Cluster API Provider k3s

Cluster API Provider k3s provides the following [Cluster API](https://github.com/kubernetes-sigs/cluster-api) (CAPI) providers:

- **Cluster API Bootstrap Provider k3s (CABP3)** is responsible for generating the instructions (and encoding them as cloud-init) to turn a Machine into a Kubernetes Node; this implementation brings up [k3s](https://k3s.io/) clusters instead of full kubernetes clusters.
- **Cluster API ControlPlane Provider k3s (CACP3)** is responsible for managing the lifecycle of control plane machines for k3s; this implementation brings up [k3s](https://k3s.io/) clusters instead of full kubernetes clusters.

## Getting Started

**Warning**: Project and documentation are in an early stage, there is an assumption that a user of this provider is already familiar with Cluster API. Please consider contributing.

### Prerequisites

Check out the general [Cluster API Quickstart](https://cluster-api.sigs.k8s.io/user/quick-start.html) page to see the prerequisites for Cluster API.

Three main pieces are:

1. Management cluster. In the `samples/azure/azure-setup.sh` script, [k3d](https://k3d.io/) is used, but feel free to use [kind](https://kind.sigs.k8s.io/) as well .
2. clusterctl. Please check out [Cluster API Quickstart](https://cluster-api.sigs.k8s.io/user/quick-start.html) for instructions.
3. Infrastructure specific prerequisites:
   - For more Azure information go to [CAPZ Getting Started](https://capz.sigs.k8s.io/topics/getting-started.html)
   - For more AWS information go to [CAPA Getting Started](https://cluster-api-aws.sigs.k8s.io/)
   - For more Nutanix information go to [CAPX Getting Started](https://opendocs.nutanix.com/capx/latest/getting_started/)
   - For more OpenStack information go to [CAPO Getting Started](https://cluster-api.sigs.k8s.io/user/quick-start.html)
   - For more Vsphere information go to [CAPV Getting Started](https://cluster-api.sigs.k8s.io/user/quick-start.html)

In this getting started guide we'll be using Docker as the infrastructure provider (CAPD).

### Create a management cluster

1. Ensure kind is installed ([instructions](https://kind.sigs.k8s.io/docs/user/quick-start/#installation))
2. Create a kind configuration to expose the local docker socket:

```bash
cat > kind-cluster-with-extramounts.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: capi-quickstart
nodes:
- role: control-plane
  extraMounts:
    - hostPath: /var/run/docker.sock
      containerPath: /var/run/docker.sock
EOF
```

3. Run the following command to create the cluster:

```bash
kind create cluster --config kind-cluster-with-extramounts.yaml
```

4. Check that you can connect to the cluster

```bash
kubectl get pods -A
```

### Install the providers

1. Create a file called `clusterctl.yaml` in the `$HOME/.config/cluster-api` directory with the following content:

```yaml
# NOTE: the following will be changed to use 'latest' in the future
providers:
  - name: "k3s"
    url: "https://github.com/k3s-io/cluster-api-k3s/releases/v0.2.1/bootstrap-components.yaml"
    type: "BootstrapProvider"
  - name: "k3s"
    url: "https://github.com/k3s-io/cluster-api-k3s/releases/v0.2.1/control-plane-components.yaml"
    type: "ControlPlaneProvider"
```

> This configuration tells clusterctl where to look for the provider manifests. You could run `clusterctl config -h` to check default clusterctl configuration file path.

2. Install the providers:

```bash
clusterctl init --bootstrap k3s --control-plane k3s --infrastructure docker
```

3. Wait for the pods to start

### Create a workload cluster

There are a number of different cluster templates in the [samples](./samples/) directory.

> Note: there is an issue with CAPD, it would be better you could do this setup beforehand. [Cluster API with Docker - &#34;too many open files&#34;.](https://cluster-api.sigs.k8s.io/user/troubleshooting.html?highlight=too%20many#cluster-api-with-docker----too-many-open-files)

1. Run the following command to generate your cluster definition:

```bash
export KIND_IMAGE_VERSION=v1.30.0
clusterctl generate cluster --from samples/docker/cluster-template-quickstart.yaml test1 --kubernetes-version v1.30.2+k3s2 --worker-machine-count 2 --control-plane-machine-count 1 > cluster.yaml
```

> NOTE: the kubernetes version specified with the k3s suffix `+k3s2`.

2. Check the contents of the generated cluster definition in **cluster.yaml**
3. Ensure the definition is valid by doing a dry run:

```bash
kubectl apply -f cluster.yaml --dry-run=server
```

4. When you are happy apply the definition:

```bash
kubectl apply -f cluster.yaml
```

### Check the workload cluster

- Check the state of the CAPI machines:

```bash
kubectl get machine
```

- Get the kubeconfig for the cluster:

```bash
clusterctl get kubeconfig test1 > workload-kubeconfig.yaml
```

> Note: if you are using Docker Desktop, you need to fix the kubeconfig by running:

```bash
# Point the kubeconfig to the exposed port of the load balancer, rather than the inaccessible container IP.
sed -i -e "s/server:.*/server: https:\/\/$(docker port test1-lb 6443/tcp | sed "s/0.0.0.0/127.0.0.1/")/g" ./workload-kubeconfig.yaml
```

- Connect to the child cluster

```bash
kubectl --kubeconfig workload-kubeconfig.yaml get pods -A
```

### Deleting the workload cluster

When deleting a cluster created via CAPI you must delete the top level **Cluster** resource. DO NOT delete using the original file.

For the quick start:

```bash
kubectl delete cluster test1
```

### Additional Samples

Cluster API k3s has been tested on AWS, Azure, AzureStackHCI, Nutanix, OpenStack, Docker and Vsphere environments.

- To try out the Azure flow, fork the repo and look at `samples/azure/azure-setup.sh`.
- To try out the AWS flow, fork the repo and look at `samples/aws/aws-setup.sh`.
- To try out the Nutanix flow, fork the repo and look at `samples/nutanix/nutanix-setup.sh`.
- To try out the OpenStack flow, fork the repo and look at `samples/openstack/setup.sh`.
- To try out the Vsphere flow, fork the repo and look at `samples/vsphere-capv/setup.sh`.

## Developer Setup

You could also build and install CABP3 and CACP3 from src:

```sh
# Build image with `dev` tag
make BOOTSTRAP_IMG_TAG=dev docker-build-bootstrap
make CONTROLPLANE_IMG_TAG=dev docker-build-controlplane

# Push image to your registry
export REGISTRY="localhost:5001"  # Set this to your local/remote registry
docker tag ghcr.io/k3s-io/cluster-api-k3s/controlplane-controller:dev ${REGISTRY}/controlplane-controller:dev
docker tag ghcr.io/k3s-io/cluster-api-k3s/bootstrap-controller:dev ${REGISTRY}/bootstrap-controller:dev
docker push ${REGISTRY}/controlplane-controller:dev
docker push ${REGISTRY}/bootstrap-controller:dev

# Install CAPI k3s to management cluster
make install-controlplane # install CRDs
make install-bootstrap
make CONTROLPLANE_IMG=${REGISTRY}/controlplane-controller CONTROLPLANE_IMG_TAG=dev deploy-controlplane  # deploy the component
make BOOTSTRAP_IMG=${REGISTRY}/bootstrap-controller BOOTSTRAP_IMG_TAG=dev deploy-bootstrap
```

For easy development, please refer to [tilt-setup.md](docs/tilt-setup.md).

## Roadmap

- Support for External Databases
- Fix Token Logic
- Clean up Control Plane Provider Code
- Post an issue!

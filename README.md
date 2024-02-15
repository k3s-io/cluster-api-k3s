# Cluster API Provider k3s

Cluster API Provider k3s provides the following [Cluster API](<https://github.com/kubernetes-sigs/cluster-api> (CAPI) providers:

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
    url: "https://github.com/k3s-io/cluster-api-k3s/releases/v0.1.9/bootstrap-components.yaml"
    type: "BootstrapProvider"
  - name: "k3s"
    url: "https://github.com/k3s-io/cluster-api-k3s/releases/v0.1.9/control-plane-components.yaml"
    type: "ControlPlaneProvider"
```

> This configuration tells clusterctl where to look for the provider manifests.

2. Install the providers:

```bash
clusterctl init --bootstrap k3s --control-plane k3s --infrastructure docker
```

3. Wait for the pods to start

### Create a workload cluster

There are a number of different cluster templates in the [samples](./samples/) directory.

1. Run the following command to generate your cluster definition:

```bash
clusterctl generate cluster --from samples/docker/quickstart.yaml test1 --kubernetes-version v1.28.6 --worker-machine-count 2 --control-plane-machine-count 1 > cluster.yaml
```

> NOTE: the kubernetes version specified with without the k3s suffix, this gets added in the template.

2. Check the contents of the generated cluster definition in **cluster.yaml**
3. Ensure the definition is valid by doing a dry run:

```bash
kubectl apply -f cluster.yaml --dry-run=server
```

4. When you are happy apply the definition:

```bash
kubectl apply -f cluster.yaml
```

## Check the workload cluster

- Check the state of the CAPI machines:

```bash
kubectl get machine
```

- Get the kubeconfig for the cluster:

```bash
clusterctl get kubeconfig test1 > workload-kubeconfig.yaml
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

## Roadmap

- Support for External Databases
- Fix Token Logic
- Clean up Control Plane Provider Code
- Post an issue!

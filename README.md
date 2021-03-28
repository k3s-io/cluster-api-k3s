# Cluster API k3s

Cluster API bootstrap provider k3s (CABP3) is a component of [Cluster API](https://github.com/kubernetes-sigs/cluster-api/blob/master/README.md) that is responsible for generating a cloud-init script to turn a Machine into a Kubernetes Node; this implementation brings up [k3s](https://k3s.io/) clusters instead of full kubernetes clusters.

CABP3 is the bootstrap component of Cluster API for k3s and brings in the following CRDS and controllers:
- k3s bootstrap provider (KThrees, KThreesTemplate)

Cluster API ControlPlane provider k3s (CACP3) is a component of [Cluster API](https://github.com/kubernetes-sigs/cluster-api/blob/master/README.md) that is responsible for managing the lifecycle of control plane machines for k3s; this implementation brings up [k3s](https://k3s.io/) clusters instead of full kubernetes clusters.

CACP3 is the controlplane component of Cluster API for k3s and brings in the following CRDS and controllers:
- k3s controlplane provider (KThreesControlPlane)

## Testing it out.

**Warning**: Project and documentation are in an early stage, there is an assumption that an user of this provider is already familiar with ClusterAPI.  


### Prerequisites

Check out the [ClusterAPI Quickstart](https://cluster-api.sigs.k8s.io/user/quick-start.html) page to see the prerequisites for ClusterAPI.

Three main pieces are 

1. Bootstrap cluster. In the `samples/azure/azure-setup.sh` script, I use [k3d](https://k3d.io/), but feel free to use [kind](https://kind.sigs.k8s.io/) as well.
2. clusterctl. Please check out [ClusterAPI Quickstart](https://cluster-api.sigs.k8s.io/user/quick-start.html) for instructions.
3. Azure Service Principals. For more information go to [CAPZ Getting Started](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/master/docs/getting-started.md)

CABP3 has been tested only on with an Azure and AzureStackHCI environment. To try out the Azure flow, fork the repo and look at `samples/azure/azure-setup.sh`.

CACP3 is alive! Sample now includes the K3s Control Plane Provider. If you run the sample script you will get a cluster with a control plane and two workers.

Then run the following to scale the control plane...
```sh
kubectl scale kthreescontrolplane ${CLUSTER_NAME}-control-plane --replicas 3
```

### Known Issues

## Roadmap

* Support for External Databases
* Fix Token Logic
* Setup CAPA and CAPV samples
* Post an issue!


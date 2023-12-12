# Cluster API k3s

Cluster API bootstrap provider k3s (CABP3) is a component of [Cluster API](https://github.com/kubernetes-sigs/cluster-api/blob/master/README.md) that is responsible for generating a cloud-init script to turn a Machine into a Kubernetes Node; this implementation brings up [k3s](https://k3s.io/) clusters instead of full kubernetes clusters.

CABP3 is the bootstrap component of Cluster API for k3s and brings in the following CRDS and controllers:
- k3s bootstrap provider (KThrees, KThreesTemplate)

Cluster API ControlPlane provider k3s (CACP3) is a component of [Cluster API](https://github.com/kubernetes-sigs/cluster-api/blob/master/README.md) that is responsible for managing the lifecycle of control plane machines for k3s; this implementation brings up [k3s](https://k3s.io/) clusters instead of full kubernetes clusters.

CACP3 is the controlplane component of Cluster API for k3s and brings in the following CRDS and controllers:
- k3s controlplane provider (KThreesControlPlane)

Together these two components make up Cluster API k3s...

## Testing it out.

**Warning**: Project and documentation are in an early stage, there is an assumption that an user of this provider is already familiar with ClusterAPI.  

### Prerequisites

Check out the [ClusterAPI Quickstart](https://cluster-api.sigs.k8s.io/user/quick-start.html) page to see the prerequisites for ClusterAPI.

Three main pieces are 

1. Bootstrap cluster. In the `samples/azure/azure-setup.sh` script, I use [k3d](https://k3d.io/), but feel free to use [kind](https://kind.sigs.k8s.io/) as well.
2. clusterctl. Please check out [ClusterAPI Quickstart](https://cluster-api.sigs.k8s.io/user/quick-start.html) for instructions.
3. Infrastructure Specific Prerequisites:

    * For more Azure information go to [CAPZ Getting Started](https://capz.sigs.k8s.io/topics/getting-started.html)
    * For more AWS information go to [CAPA Getting Started](https://cluster-api-aws.sigs.k8s.io/)
    * For more Nutanix information go to [CAPX Getting Started](https://opendocs.nutanix.com/capx/latest/getting_started/)
    * For more OpenStack information go to [CAPO Getting Started](https://cluster-api.sigs.k8s.io/user/quick-start.html)
    * For more Vsphere information go to [CAPV Getting Started](https://cluster-api.sigs.k8s.io/user/quick-start.html)

Cluster API k3s has been tested on AWS, Azure, AzureStackHCI, Nutanix, OpenStack, and Vsphere environments. 

To try out the Azure flow, fork the repo and look at `samples/azure/azure-setup.sh`.

To try out the AWS flow, fork the repo and look at `samples/aws/aws-setup.sh`.

To try out the Nutanix flow, fork the repo and look at `samples/nutanix/nutanix-setup.sh`.

To try out the OpenStack flow, fork the repo and look at `samples/openstack/setup.sh`.

To try out the Vsphere flow, fork the repo and look at `samples/vsphere-capv/setup.sh`.

### Known Issues

## Roadmap

* Support for External Databases
* Fix Token Logic
* Clean up Control Plane Provider Code
* Post an issue!


## Configure your Vsphere parameters


export VSPHERE_USERNAME="******"
export VSPHERE_PASSWORD="${VSPHERE_PASSWORD:-}"
export VSPHERE_SSH_AUTHORIZED_KEY="ssh-rsa ******"

# Use an Vsphere image-builder builded image (https://github.com/kubernetes-sigs/image-builder)
export VSPHERE_TEMPLATE="ubuntu-2004-kube-v1.24.11"
export CPI_IMAGE_K8S_VERSION="v1.24.11+k3s1"
export CCM_IMAGE="gcr.io/cloud-provider-vsphere/cpi/release/manager:v1.24.0"

export VSPHERE_SERVER="10.0.0.1"
export VSPHERE_TLS_THUMBPRINT="$(echo | openssl s_client -showcerts -connect ${VSPHERE_SERVER}:443 2> /dev/null | openssl x509 -sha1 -fingerprint 2> /dev/null | grep Fingerprint | cut -d= -f2)"
export VSPHERE_INSECURE_TLS="false"
export VSPHERE_DATACENTER="vc01"
export VSPHERE_CLUSTER_NAME="vc01cl01"
export VSPHERE_RESOURCE_POOL="k3s-test"
export VSPHERE_FOLDER="k3s-test"
export VSPHERE_DATASTORE="vc01cl01-vsan"
export VSPHERE_STORAGE_POLICY=""
export VSPHERE_NETWORK="k8-vdp"
export VSPHERE_MACHINE_MEMORY_SIZE=4Gi
export VSPHERE_SYSTEMDISK_SIZE=40Gi
export VSPHERE_MACHINE_VCPU_SOCKET=2
export VSPHERE_MACHINE_VCPU_PER_SOCKET=1
export VSPHERE_SYSTEMDISK_SIZE_GB=50
export VSPHERE_MACHINE_MEMORY_SIZE_MB=32000
export VIP_NETWORK_INTERFACE=""
export KUBE_VIP_IMAGE="ghcr.io/kube-vip/kube-vip"
export KUBE_VIP_VERSION="v0.5.7"
export CONTROL_PLANE_ENDPOINT_IP=10.0.0.250
export CONTROL_PLANE_ENDPOINT_PORT=6443
export EXP_CLUSTER_RESOURCE_SET="true"
export WORKER_MACHINE_COUNT=0
export CONTROL_PLANE_MACHINE_COUNT=1

if [ -z "${CLUSTER_NAME}" ]; then
  echo "please set CLUSTER_NAME"
  exit 0
fi

if [ -z "${CONTROL_PLANE_ENDPOINT_IP}" ]; then
  echo "please set CONTROL_PLANE_ENDPOINT_IP"
  exit 0
fi

k3d cluster create bootstrap-cluster

## Install correctly your cluser-api-k3s provider

export PWD="$(pwd)"
mkdir -p ~/.cluster-api
cat samples/clusterctl.yaml | envsubst > ~/.cluster-api/clusterctl.yaml

clusterctl init --infrastructure vsphere --bootstrap k3s --control-plane k3s

kubectl wait --for=condition=Available --timeout=5m -n capi-system deployment/capi-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-control-plane-system deployment/capi-k3s-control-plane-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capv-system deployment/capv-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-bootstrap-system deployment/capi-k3s-bootstrap-controller-manager

# clusterctl generate cluster "${CLUSTER_NAME}" -f k3s | kubectl apply -f -
clusterctl generate cluster "${CLUSTER_NAME}" --from samples/vsphere-capv/cluster-template-k3s.yaml | kubectl apply -f -

echo "Once the cluster is up run clusterctl get kubeconfig $CLUSTER_NAME > k3s.yaml or kubectl scale kthreescontrolplane $CLUSTER_NAME-control-plane --replicas 3 for HA"
echo "To use the single node cluster, you might need to also run the following commands:"
echo "   kubectl taint nodes --all node-role.kubernetes.io/control-plane-"
echo "   kubectl taint nodes --all node-role.kubernetes.io/master-"

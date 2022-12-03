if [ -z "${CLUSTER_NAME}" ]; then
  echo "please set CLUSTER_NAME"
  exit 0
fi

if [ -z "${CONTROL_PLANE_ENDPOINT_IP}" ]; then
  echo "please set CONTROL_PLANE_ENDPOINT_IP"
  exit 0
fi

k3d cluster create bootstrap-cluster

export EXP_CLUSTER_RESOURCE_SET=true
export WORKER_MACHINE_COUNT=2


## Configure your Nutanix parameters

# Use an Nutanix image-builder builded image (https://github.com/kubernetes-sigs/image-builder)
# export NUTANIX_MACHINE_TEMPLATE_IMAGE_NAME="nutanix-ubuntu-kube-20.04"

# export NUTANIX_ENDPOINT=""
# export NUTANIX_USER=""
# export NUTANIX_PASSWORD=""
# export NUTANIX_PRISM_ELEMENT_CLUSTER_NAME=""
# export NUTANIX_SUBNET_NAME=""


## Install correctly your cluser-api-k3s provider

# export PWD="$(pwd)"
# mkdir -p ~/.cluster-api
# cat samples/clusterctl.yaml | envsubst > ~/.cluster-api/clusterctl.yaml

clusterctl init --infrastructure nutanix --bootstrap k3s --control-plane k3s

kubectl wait --for=condition=Available --timeout=5m -n capi-system deployment/capi-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-control-plane-system deployment/capi-k3s-control-plane-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capx-system deployment/capx-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-bootstrap-system deployment/capi-k3s-bootstrap-controller-manager

clusterctl generate cluster ${CLUSTER_NAME} -f k3s | kubectl apply -f -

echo "Once the cluster is up run clusterctl get kubeconfig $CLUSTER_NAME > k3s.yaml or kubectl scale kthreescontrolplane $CLUSTER_NAME-control-plane --replicas 3 for HA"
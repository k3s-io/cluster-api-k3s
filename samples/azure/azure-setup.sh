read -p "Please follow Getting Started to setup the clusterctl configuration first before running this script!"

if [ -z "${AZURE_SUBSCRIPTION_ID}" ]; then
  echo "please set AZURE_SUBSCRIPTION_ID"
  exit 0
fi

if [ -z "${AZURE_TENANT_ID}" ]; then
  echo "please set AZURE_TENANT_ID"
  exit 0
fi

if [ -z "${AZURE_CLIENT_ID}" ]; then
  echo "please set AZURE_CLIENT_ID"
  exit 0
fi

if [ -z "${AZURE_CLIENT_SECRET}" ]; then
  echo "please set AZURE_CLIENT_SECRET"
  exit 0
fi

if [ -z "${CLUSTER_NAME}" ]; then
  echo "please set CLUSTER_NAME"
  exit 0
fi

k3d cluster create mycluster

export AZURE_LOCATION="eastus"
export AZURE_ENVIRONMENT="AzurePublicCloud"
export AZURE_SUBSCRIPTION_ID_B64="$(echo -n "$AZURE_SUBSCRIPTION_ID" | base64 | tr -d '\n')"
export AZURE_TENANT_ID_B64="$(echo -n "$AZURE_TENANT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_ID_B64="$(echo -n "$AZURE_CLIENT_ID" | base64 | tr -d '\n')"
export AZURE_CLIENT_SECRET_B64="$(echo -n "$AZURE_CLIENT_SECRET" | base64 | tr -d '\n')"

clusterctl init --infrastructure azure --bootstrap k3s --control-plane k3s

kubectl wait --for=condition=Available --timeout=5m -n capi-system deployment/capi-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-control-plane-system deployment/capi-k3s-control-plane-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capz-system deployment/capz-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-bootstrap-system deployment/capi-k3s-bootstrap-controller-manager

# Settings needed for AzureClusterIdentity used by the AzureCluster
export AZURE_CLUSTER_IDENTITY_SECRET_NAME="cluster-identity-secret"
export CLUSTER_IDENTITY_NAME="cluster-identity"
export AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE="default"

# Create a secret to include the password of the Service Principal identity created in Azure
# This secret will be referenced by the AzureClusterIdentity used by the AzureCluster
kubectl create secret generic "${AZURE_CLUSTER_IDENTITY_SECRET_NAME}" --from-literal=clientSecret="${AZURE_CLIENT_SECRET}" --namespace "${AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE}"

clusterctl generate cluster --from samples/azure/k3s-template.yaml $CLUSTER_NAME --kubernetes-version v1.30.2+k3s2 --worker-machine-count 2 --control-plane-machine-count 1 > samples/azure/k3s-cluster.yaml

kubectl apply -f samples/azure/k3s-cluster.yaml

read -p "Please wait for the cluster to be up and running, press any key to continue"

clusterctl get kubeconfig $CLUSTER_NAME > k3s.yaml

echo "Manually apply cloud provider to workload cluster, you could visit https://cluster-api.sigs.k8s.io/user/quick-start for the latest info"

# Note that the nodeSelector should be modified to fit k3s. Run this command for installing cloud provider:
helm install --kubeconfig=./k3s.yaml --repo https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo cloud-provider-azure --generate-name --set infra.clusterName=$CLUSTER_NAME --set cloudControllerManager.clusterCIDR="10.42.0.0/16" --set-string cloudControllerManager.nodeSelector."node-role\.kubernetes\.io/control-plane"=true

echo "You could run kubectl scale kthreescontrolplane $CLUSTER_NAME-control-plane --replicas 3 for HA"
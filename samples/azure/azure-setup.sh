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

export EXP_CLUSTER_RESOURCE_SET=true
clusterctl init --infrastructure azure
kubectl apply -f samples/bootstrap-provider.yaml

kubectl wait --for=condition=Available --timeout=5m -n capi-system deployment/capi-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-kubeadm-control-plane-system deployment/capi-kubeadm-control-plane-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capz-system deployment/capz-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-bootstrap-system deployment/capi-k3s-bootstrap-controller-manager


cat samples/azure/k3s-template.yaml | envsubst > samples/azure/k3s-cluster.yaml
kubectl create configmap azure-ccm-addon --from-file=samples/azure/azure-ccm.yaml
kubectl create configmap azure-cn-addon --from-file=samples/azure/azure-cn.yaml
kubectl apply -f samples/azure/k3s-cluster.yaml

echo "wait for the the first control plane to be in provisioning state run kubectl apply -f samples/azure/resource-set.yaml"
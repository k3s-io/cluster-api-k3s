if [ -z "${CLUSTER_NAME}" ]; then
  echo "please set CLUSTER_NAME"
  exit 0
fi

if [ -z "${KUBERNETES_VERSION}" ]; then
  echo "please set KUBERNETES_VERSION"
  exit 0
fi

## Configure your OpenStack parameters
# https://github.com/kubernetes-sigs/cluster-api-provider-openstack/blob/main/docs/book/src/clusteropenstack/configuration.md

if [ -z "${OPENSTACK_CLOUD}" ] || [ -z "${OPENSTACK_CONTROL_PLANE_MACHINE_FLAVOR}" ] || [ -z "${OPENSTACK_DNS_NAMESERVERS}" ] || [ -z "${OPENSTACK_EXTERNAL_NETWORK_ID}" ] || [ -z "${OPENSTACK_FAILURE_DOMAIN}" ] || [ -z "${OPENSTACK_IMAGE_NAME}" ] || [ -z "${OPENSTACK_NODE_MACHINE_FLAVOR}" ] || [ -z "${OPENSTACK_SSH_KEY_NAME}" ]; then
  echo 'Please set KUBERNETES_VERSION, OPENSTACK_CLOUD, OPENSTACK_CONTROL_PLANE_MACHINE_FLAVOR, OPENSTACK_DNS_NAMESERVERS, OPENSTACK_EXTERNAL_NETWORK_ID, OPENSTACK_FAILURE_DOMAIN, OPENSTACK_IMAGE_NAME, OPENSTACK_NODE_MACHINE_FLAVOR, OPENSTACK_SSH_KEY_NAME'
  exit 0
fi

## Create a secret for your OpenStack API

if [ -z "${OS_AUTH_URL}" ] || [ -z "${OS_USERNAME}" ] || [ -z "${OS_PASSWORD}" ] || [ -z "${OS_PROJECT_NAME}" ] || [ -z "${OS_USER_DOMAIN_NAME}" ] || [ -z "${OS_PROJECT_DOMAIN_NAME}" ] || [ -z "${OS_REGION_NAME}" ]; then
  echo 'Please set OS_AUTH_URL, OS_USERNAME, OS_PASSWORD, OS_PROJECT_NAME, OS_USER_DOMAIN_NAME, OS_PROJECT_DOMAIN_NAME, OS_REGION_NAME'
  exit 0
fi

kubectl create -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: k3s-cloud-config
  labels:
    clusterctl.cluster.x-k8s.io/move: "true"
stringData:
  cacert: ''
  clouds.yaml: |
    clouds:
      openstack:
        auth:
          auth_url: "${OS_AUTH_URL}"
          username: "${OS_USERNAME}"
          password: "${OS_PASSWORD}"
          project_name: "${OS_PROJECT_NAME}"
          user_domain_name: "${OS_USER_DOMAIN_NAME}"
          project_domain_name: "${OS_PROJECT_DOMAIN_NAME}"
        region_name: "${OS_REGION_NAME}"
        interface: "public"
        verify: false
        identity_api_version: 3
EOF


## Install correctly your cluser-api-k3s provider

export PWD="$(pwd)"
mkdir -p ~/.cluster-api
cat samples/clusterctl.yaml | envsubst > ~/.cluster-api/clusterctl.yaml

clusterctl init --infrastructure openstack --bootstrap k3s --control-plane k3s

kubectl wait --for=condition=Available --timeout=5m -n capi-system deployment/capi-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-control-plane-system deployment/capi-k3s-control-plane-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capo-system deployment/capo-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-bootstrap-system deployment/capi-k3s-bootstrap-controller-manager

clusterctl generate cluster ${CLUSTER_NAME} --from samples/openstack/k3s-template.yaml | kubectl apply -f -


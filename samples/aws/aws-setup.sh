if [ -z "${AWS_ACCESS_KEY_ID}" ]; then
  echo "please set AWS_ACCESS_KEY_ID"
  exit 0
fi

if [ -z "${AWS_SECRET_ACCESS_KEY}" ]; then
  echo "please set AWS_SECRET_ACCESS_KEY"
  exit 0
fi

if [ -z "${CLUSTER_NAME}" ]; then
  echo "please set CLUSTER_NAME"
  exit 0
fi


k3d cluster create mycluster

export AWS_REGION=us-east-1 # This is used to help encode your environment variables
export AWS_SSH_KEY_NAME=default
# Select instance types
export AWS_CONTROL_PLANE_MACHINE_TYPE=t3.large
export AWS_NODE_MACHINE_TYPE=t3.large
export EXP_CLUSTER_RESOURCE_SET=true

export PWD="$(pwd)"
mkdir -p ~/.cluster-api
cat samples/clusterctl.yaml | envsubst > ~/.cluster-api/clusterctl.yaml

# The clusterawsadm utility takes the credentials that you set as environment
# variables and uses them to create a CloudFormation stack in your AWS account
# with the correct IAM resources.
clusterawsadm bootstrap iam create-cloudformation-stack

# Create the base64 encoded credentials using clusterawsadm.
# This command uses your environment variables and encodes
# them in a value to be stored in a Kubernetes Secret.
export AWS_B64ENCODED_CREDENTIALS=$(clusterawsadm bootstrap credentials encode-as-profile)

clusterctl init --infrastructure aws --bootstrap k3s --control-plane k3s

kubectl wait --for=condition=Available --timeout=5m -n capi-system deployment/capi-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-control-plane-system deployment/capi-k3s-control-plane-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capa-system deployment/capa-controller-manager
kubectl wait --for=condition=Available --timeout=5m -n capi-k3s-bootstrap-system deployment/capi-k3s-bootstrap-controller-manager

cat samples/aws/k3s-template.yaml | envsubst > samples/aws/k3s-cluster.yaml
kubectl create configmap cloud-controller-manager-addon --from-file=samples/aws/aws-ccm.yaml
kubectl create configmap aws-ebs-csi-driver-addon --from-file=samples/aws/aws-csi.yaml
kubectl apply -f samples/aws/k3s-cluster.yaml
kubectl apply -f samples/aws/resource-set.yaml


echo ""
echo "Waiting on Cluster $CLUSTER_NAME to be provisioned, then applying elb healthcheck workaround..."
until kubectl get awsclusters/$CLUSTER_NAME --output=jsonpath='{.status.ready}' | grep "true"; do : ; done
aws elb configure-health-check --load-balancer-name $CLUSTER_NAME-apiserver --health-check Target=TCP:6443,Interval=30,UnhealthyThreshold=2,HealthyThreshold=2,Timeout=3 > /dev/null
echo ""
echo "Once the cluster is up run clusterctl get kubeconfig $CLUSTER_NAME > k3s.yaml or kubectl scale kthreescontrolplane $CLUSTER_NAME-control-plane --replicas 3 for HA"
package desiredstate

import (
	"encoding/json"

	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/controlplane/internal/names"
)

// computeDesiredMachine computes the desired Machine.
// This Machine will be used during reconciliation to:
// * create a new Machine
// * update an existing Machine
// Because we are using Server-Side-Apply we always have to calculate the full object.
// There are small differences in how we calculate the Machine depending on if it
// is a create or update. Example: for a new Machine we have to calculate a new name,
// while for an existing Machine we have to use the name of the existing Machine.
// Also, for an existing Machine, we will not copy its labels, as they are not managed by the KThreesControlPlane controller.
func ComputeDesiredMachine(kcp *controlplanev1.KThreesControlPlane, cluster *clusterv1.Cluster, failureDomain string, existingMachine *clusterv1.Machine) (*clusterv1.Machine, error) {
	var machineName string
	var machineUID types.UID
	var version string
	annotations := map[string]string{}
	if existingMachine == nil {
		// Creating a new machine
		nameTemplate := "{{.kthreesControlPlane.name}}-{{.random}}"
		generatedMachineName, err := names.KCPMachineNameGenerator(nameTemplate, cluster.Name, kcp.Name).GenerateName()
		if err != nil {
			return nil, errors.Wrap(err, "failed to compute desired Machine: failed to generate Machine name")
		}
		machineName = generatedMachineName
		version = kcp.Spec.Version

		// Machine's bootstrap config may be missing ClusterConfiguration if it is not the first machine in the control plane.
		// We store ClusterConfiguration as annotation here to detect any changes in KCP ClusterConfiguration and rollout the machine if any.
		serverConfig, err := json.Marshal(kcp.Spec.KThreesConfigSpec.ServerConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal cluster configuration")
		}
		annotations[controlplanev1.KThreesServerConfigurationAnnotation] = string(serverConfig)

		// In case this machine is being created as a consequence of a remediation, then add an annotation
		// tracking remediating data.
		// NOTE: This is required in order to track remediation retries.
		if remediationData, ok := kcp.Annotations[controlplanev1.RemediationInProgressAnnotation]; ok {
			annotations[controlplanev1.RemediationForAnnotation] = remediationData
		}
	} else {
		// Updating an existing machine
		machineName = existingMachine.Name
		machineUID = existingMachine.UID
		version = existingMachine.Spec.Version

		// For existing machine only set the ClusterConfiguration annotation if the machine already has it.
		// We should not add the annotation if it was missing in the first place because we do not have enough
		// information.
		if serverConfig, ok := existingMachine.Annotations[controlplanev1.KThreesServerConfigurationAnnotation]; ok {
			annotations[controlplanev1.KThreesServerConfigurationAnnotation] = serverConfig
		}

		// If the machine already has remediation data then preserve it.
		// NOTE: This is required in order to track remediation retries.
		if remediationData, ok := existingMachine.Annotations[controlplanev1.RemediationForAnnotation]; ok {
			annotations[controlplanev1.RemediationForAnnotation] = remediationData
		}
	}

	// Construct the basic Machine.
	desiredMachine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: kcp.Namespace,
			UID:       machineUID,
			Labels:    k3s.ControlPlaneLabelsForCluster(cluster.Name, kcp.Spec.MachineTemplate),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KThreesControlPlane")),
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName:   cluster.Name,
			Version:       version,
			FailureDomain: failureDomain,
		},
	}

	// Set annotations
	for k, v := range kcp.Spec.MachineTemplate.ObjectMeta.Annotations {
		annotations[k] = v
	}

	desiredMachine.SetAnnotations(annotations)

	if existingMachine != nil {
		desiredMachine.Spec.InfrastructureRef = existingMachine.Spec.InfrastructureRef
		desiredMachine.Spec.Bootstrap.ConfigRef = existingMachine.Spec.Bootstrap.ConfigRef
	}

	return desiredMachine, nil
}

/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package desiredstate computes the objects managed by KThreesControlPlane.
package desiredstate

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/contract"
)

// ControlPlaneMachineLabels returns the labels managed by KThreesControlPlane.
func ControlPlaneMachineLabels(kcp *controlplanev1.KThreesControlPlane, clusterName string) map[string]string {
	labels := map[string]string{}
	maps.Copy(labels, kcp.Spec.MachineTemplate.ObjectMeta.Labels)
	labels[clusterv1beta1.ClusterNameLabel] = clusterName
	labels[clusterv1beta1.MachineControlPlaneNameLabel] = ""
	labels[clusterv1beta1.MachineControlPlaneLabel] = ""
	return labels
}

// ControlPlaneMachineAnnotations returns the annotations managed by KThreesControlPlane.
func ControlPlaneMachineAnnotations(kcp *controlplanev1.KThreesControlPlane) map[string]string {
	annotations := map[string]string{}
	for key, value := range kcp.Spec.MachineTemplate.ObjectMeta.Annotations {
		if !controlplanev1.IsReservedMachineTemplateAnnotation(key) {
			annotations[key] = value
		}
	}
	return annotations
}

// ComputeDesiredMachine computes the complete Machine object managed by KThreesControlPlane.
func ComputeDesiredMachine(
	kcp *controlplanev1.KThreesControlPlane,
	cluster *clusterv1.Cluster,
	infraRef clusterv1.ContractVersionedObjectReference,
	bootstrapRef clusterv1.ContractVersionedObjectReference,
	failureDomain string,
	existingMachine *clusterv1.Machine,
) (*clusterv1.Machine, error) {
	var (
		machineName string
		machineUID  types.UID
		version     string
	)
	annotations := map[string]string{}

	if existingMachine == nil {
		machineName = names.SimpleNameGenerator.GenerateName(kcp.Name + "-")
		version = kcp.Spec.Version

		serverConfig, err := json.Marshal(kcp.Spec.KThreesConfigSpec.ServerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal server configuration: %w", err)
		}
		annotations[controlplanev1.KThreesServerConfigurationAnnotation] = string(serverConfig)
		if remediationData, ok := kcp.Annotations[controlplanev1.RemediationInProgressAnnotation]; ok {
			annotations[controlplanev1.RemediationForAnnotation] = remediationData
		}
	} else {
		machineName = existingMachine.Name
		machineUID = existingMachine.UID
		version = existingMachine.Spec.Version
		infraRef = existingMachine.Spec.InfrastructureRef
		bootstrapRef = existingMachine.Spec.Bootstrap.ConfigRef
		failureDomain = existingMachine.Spec.FailureDomain

		if serverConfig, ok := existingMachine.Annotations[controlplanev1.KThreesServerConfigurationAnnotation]; ok {
			annotations[controlplanev1.KThreesServerConfigurationAnnotation] = serverConfig
		}
		if remediationData, ok := existingMachine.Annotations[controlplanev1.RemediationForAnnotation]; ok {
			annotations[controlplanev1.RemediationForAnnotation] = remediationData
		}
	}

	maps.Copy(annotations, ControlPlaneMachineAnnotations(kcp))
	desiredMachine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        machineName,
			Namespace:   kcp.Namespace,
			UID:         machineUID,
			Labels:      ControlPlaneMachineLabels(kcp, cluster.Name),
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KThreesControlPlane")),
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName:       cluster.Name,
			Version:           version,
			FailureDomain:     failureDomain,
			InfrastructureRef: infraRef,
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: bootstrapRef,
			},
			Deletion: clusterv1.MachineDeletionSpec{},
		},
	}

	if kcp.Spec.MachineTemplate.NodeDrainTimeout != nil {
		desiredMachine.Spec.Deletion.NodeDrainTimeoutSeconds = new(int32)
		*desiredMachine.Spec.Deletion.NodeDrainTimeoutSeconds = int32(kcp.Spec.MachineTemplate.NodeDrainTimeout.Seconds())
	}
	if kcp.Spec.MachineTemplate.NodeVolumeDetachTimeout != nil {
		desiredMachine.Spec.Deletion.NodeVolumeDetachTimeoutSeconds = new(int32)
		*desiredMachine.Spec.Deletion.NodeVolumeDetachTimeoutSeconds = int32(kcp.Spec.MachineTemplate.NodeVolumeDetachTimeout.Seconds())
	}
	if kcp.Spec.MachineTemplate.NodeDeletionTimeout != nil {
		desiredMachine.Spec.Deletion.NodeDeletionTimeoutSeconds = new(int32)
		*desiredMachine.Spec.Deletion.NodeDeletionTimeoutSeconds = int32(kcp.Spec.MachineTemplate.NodeDeletionTimeout.Seconds())
	}

	return desiredMachine, nil
}

// ComputeDesiredKThreesConfig computes the complete KThreesConfig managed by KThreesControlPlane.
func ComputeDesiredKThreesConfig(
	kcp *controlplanev1.KThreesControlPlane,
	cluster *clusterv1.Cluster,
	name string,
	existingConfig *bootstrapv1.KThreesConfig,
) (*bootstrapv1.KThreesConfig, error) {
	var ownerReferences []metav1.OwnerReference
	if existingConfig == nil || !util.HasOwner(existingConfig.OwnerReferences, clusterv1.GroupVersion.String(), []string{"Machine"}) {
		ownerReferences = []metav1.OwnerReference{{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KThreesControlPlane",
			Name:       kcp.Name,
			UID:        kcp.UID,
		}}
	}

	desired := &bootstrapv1.KThreesConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: bootstrapv1.GroupVersion.String(),
			Kind:       "KThreesConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       kcp.Namespace,
			Labels:          ControlPlaneMachineLabels(kcp, cluster.Name),
			Annotations:     ControlPlaneMachineAnnotations(kcp),
			OwnerReferences: ownerReferences,
		},
		Spec: *kcp.Spec.KThreesConfigSpec.DeepCopy(),
	}
	if existingConfig != nil {
		desired.Name = existingConfig.Name
		desired.UID = existingConfig.UID
		desired.Spec.Version = existingConfig.Spec.Version
	}
	return desired, nil
}

// ComputeDesiredInfraMachine computes the InfraMachine from the current infrastructure template.
func ComputeDesiredInfraMachine(
	ctx context.Context,
	c client.Client,
	kcp *controlplanev1.KThreesControlPlane,
	cluster *clusterv1.Cluster,
	name string,
	existingInfraMachine *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	var ownerReference *metav1.OwnerReference
	if existingInfraMachine == nil || !util.HasOwner(existingInfraMachine.GetOwnerReferences(), clusterv1.GroupVersion.String(), []string{"Machine"}) {
		ownerReference = &metav1.OwnerReference{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KThreesControlPlane",
			Name:       kcp.Name,
			UID:        kcp.UID,
		}
	}

	infraRef := kcp.Spec.MachineTemplate.InfrastructureRef
	apiVersion, err := contract.GetAPIVersion(ctx, c, infraRef.GroupVersionKind().GroupKind())
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute desired InfraMachine")
	}
	templateRef := &corev1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       infraRef.Kind,
		Namespace:  kcp.Namespace,
		Name:       infraRef.Name,
	}
	template, err := external.Get(ctx, c, templateRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute desired InfraMachine")
	}
	infraMachine, err := external.GenerateTemplate(&external.GenerateTemplateInput{
		Template:    template,
		TemplateRef: templateRef,
		Namespace:   kcp.Namespace,
		Name:        name,
		ClusterName: cluster.Name,
		OwnerRef:    ownerReference,
		Labels:      ControlPlaneMachineLabels(kcp, cluster.Name),
		Annotations: ControlPlaneMachineAnnotations(kcp),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute desired InfraMachine")
	}
	if existingInfraMachine != nil {
		infraMachine.SetName(existingInfraMachine.GetName())
		infraMachine.SetUID(existingInfraMachine.GetUID())
	}
	return infraMachine, nil
}

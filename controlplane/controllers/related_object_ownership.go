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

package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	pkgcontract "github.com/k3s-io/cluster-api-k3s/pkg/contract"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/util/ssa"
)

const relatedObjectOwnershipReplacementMessage = "Related-object ownership requires Machine replacement"

func (r *KThreesControlPlaneReconciler) createRelatedObject(
	ctx context.Context,
	obj client.Object,
	gvk schema.GroupVersionKind,
	kcp *controlplanev1.KThreesControlPlane,
	cluster *clusterv1.Cluster,
) error {
	if err := ssa.Patch(ctx, r.Client, kcpManagerName, obj); err != nil {
		return errors.Wrapf(err, "failed to create %s", gvk.Kind)
	}
	if err := ssa.RemoveManagedFieldsForLabelsAndAnnotations(
		ctx,
		r.Client,
		r.apiReader,
		obj,
		kcpManagerName,
	); err != nil {
		return errors.Wrapf(err, "failed to split managedFields ownership for %s", gvk.Kind)
	}
	if err := r.updateExternalObject(ctx, obj, gvk, kcp, cluster); err != nil {
		return errors.Wrapf(err, "failed to establish metadata ownership for %s", gvk.Kind)
	}
	return nil
}

func (r *KThreesControlPlaneReconciler) reconcileRelatedObjectManagedFields(
	ctx context.Context,
	controlPlane *k3s.ControlPlane,
) (migrationCompleted bool, err error) {
	for machineName, machine := range controlPlane.Machines {
		if !machine.DeletionTimestamp.IsZero() {
			continue
		}

		if infraMachine, ok := controlPlane.InfraResources[machineName]; ok {
			result, err := ssa.MigrateManagedFields(
				ctx,
				r.Client,
				r.apiReader,
				infraMachine,
				kcpManagerName,
				kcpMetadataManagerName,
			)
			if err != nil {
				return false, errors.Wrapf(err, "failed to migrate managedFields of InfrastructureMachine %s", klog.KObj(infraMachine))
			}
			switch result.Outcome {
			case ssa.ManagedFieldsMigrationCompleted:
				migrationCompleted = true
			case ssa.ManagedFieldsMigrationInPlaceUpdateUnsupported:
				controlPlane.MarkInPlaceUpdateUnsupported(
					machineName,
					result.Reason,
					relatedObjectOwnershipReplacementMessage,
				)
			}
		}

		kthreesConfig, ok := controlPlane.KthreesConfigs[machineName]
		if !ok {
			continue
		}

		version, err := pkgcontract.GetAPIVersion(ctx, r.Client, schema.GroupKind{
			Group: machine.Spec.Bootstrap.ConfigRef.APIGroup,
			Kind:  machine.Spec.Bootstrap.ConfigRef.Kind,
		})
		if err != nil {
			return false, fmt.Errorf("failed to get api version for bootstrap config: %w", err)
		}
		groupVersion, err := schema.ParseGroupVersion(version)
		if err != nil {
			return false, fmt.Errorf("failed to parse api version for bootstrap config: %w", err)
		}
		kthreesConfig.SetGroupVersionKind(groupVersion.WithKind(machine.Spec.Bootstrap.ConfigRef.Kind))

		result, err := ssa.MigrateManagedFields(
			ctx,
			r.Client,
			r.apiReader,
			kthreesConfig,
			kcpManagerName,
			kcpMetadataManagerName,
		)
		if err != nil {
			return false, errors.Wrapf(err, "failed to migrate managedFields of KThreesConfigs %s", klog.KObj(kthreesConfig))
		}
		switch result.Outcome {
		case ssa.ManagedFieldsMigrationCompleted:
			migrationCompleted = true
		case ssa.ManagedFieldsMigrationInPlaceUpdateUnsupported:
			controlPlane.MarkInPlaceUpdateUnsupported(
				machineName,
				result.Reason,
				relatedObjectOwnershipReplacementMessage,
			)
		}
	}

	return migrationCompleted, nil
}

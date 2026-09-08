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

	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/util/collections"
	ctrl "sigs.k8s.io/controller-runtime"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

func (r *KThreesControlPlaneReconciler) updateControlPlane(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	kcp *controlplanev1.KThreesControlPlane,
	controlPlane *k3s.ControlPlane,
	machinesNeedingRollout collections.Machines,
	results map[string]k3s.UpToDateResult,
) (ctrl.Result, error) {
	if kcp.Spec.RolloutStrategy != nil &&
		kcp.Spec.RolloutStrategy.Type != "" &&
		kcp.Spec.RolloutStrategy.Type != controlplanev1.RollingUpdateStrategyType {
		return ctrl.Result{}, errors.Errorf("unknown rollout strategy type %q", kcp.Spec.RolloutStrategy.Type)
	}
	return r.rollingUpdate(ctx, cluster, kcp, controlPlane, machinesNeedingRollout, results)
}

func (r *KThreesControlPlaneReconciler) rollingUpdate(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	kcp *controlplanev1.KThreesControlPlane,
	controlPlane *k3s.ControlPlane,
	machinesNeedingRollout collections.Machines,
	results map[string]k3s.UpToDateResult,
) (ctrl.Result, error) {
	currentReplicas := int32(controlPlane.Machines.Len())
	currentUpToDateReplicas := int32(controlPlane.UpToDateMachines().Len())
	desiredReplicas := *kcp.Spec.Replicas
	maxSurge := rolloutMaxSurge(kcp)
	maxReplicas := desiredReplicas + maxSurge

	if currentReplicas < maxReplicas {
		if r.overrides != nil && r.overrides.scaleUpControlPlane != nil {
			return r.overrides.scaleUpControlPlane(ctx, cluster, kcp, controlPlane)
		}
		return r.scaleUpControlPlane(ctx, cluster, kcp, controlPlane)
	}

	if maxSurge == 0 &&
		desiredReplicas < 3 &&
		currentReplicas <= desiredReplicas &&
		!feature.Gates.Enabled(feature.InPlaceUpdates) {
		return ctrl.Result{}, errors.New("maxSurge=0 with fewer than three replicas requires InPlaceUpdates")
	}

	machine, err := selectMachineForInPlaceUpdateOrScaleDown(ctx, controlPlane, machinesNeedingRollout)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to select next Machine for rollout")
	}
	result, ok := results[machine.Name]
	if !ok {
		return ctrl.Result{}, errors.Errorf("failed to check if Machine %s is UpToDate", machine.Name)
	}

	if feature.Gates.Enabled(feature.InPlaceUpdates) &&
		result.EligibleForInPlaceUpdate &&
		currentUpToDateReplicas < desiredReplicas {
		var (
			fallback bool
			res      ctrl.Result
		)
		if r.overrides != nil && r.overrides.tryInPlaceUpdate != nil {
			fallback, res, err = r.overrides.tryInPlaceUpdate(ctx, controlPlane, machine, result)
		} else {
			fallback, res, err = r.tryInPlaceUpdate(ctx, controlPlane, machine, result)
		}
		if err != nil {
			return ctrl.Result{}, err
		}
		if !res.IsZero() {
			return res, nil
		}
		if !fallback {
			return ctrl.Result{}, nil
		}
		return r.scaleDownForUpdate(ctx, cluster, kcp, controlPlane, collections.FromMachines(machine))
	}

	return r.scaleDownForUpdate(ctx, cluster, kcp, controlPlane, machinesNeedingRollout)
}

// scaleDownForUpdate prevents update fallback from deleting without safe surplus capacity on low-replica control planes.
func (r *KThreesControlPlaneReconciler) scaleDownForUpdate(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	kcp *controlplanev1.KThreesControlPlane,
	controlPlane *k3s.ControlPlane,
	machines collections.Machines,
) (ctrl.Result, error) {
	desiredReplicas := *kcp.Spec.Replicas
	currentReplicas := int32(controlPlane.Machines.Len())
	if rolloutMaxSurge(kcp) == 0 && desiredReplicas < 3 && currentReplicas <= desiredReplicas {
		return ctrl.Result{}, errors.Errorf(
			"maxSurge=0 with fewer than three replicas cannot fall back to Machine replacement; "+
				"enable or fix a working in-place update extension or set maxSurge to 1 "+
				"(desired replicas: %d, current Machines: %d)",
			desiredReplicas,
			currentReplicas,
		)
	}
	if r.overrides != nil && r.overrides.scaleDownControlPlane != nil {
		return r.overrides.scaleDownControlPlane(ctx, cluster, kcp, controlPlane, machines)
	}
	return r.scaleDownControlPlane(ctx, cluster, kcp, controlPlane, machines)
}

func rolloutMaxSurge(kcp *controlplanev1.KThreesControlPlane) int32 {
	if strategy := kcp.Spec.RolloutStrategy; strategy != nil &&
		strategy.RollingUpdate != nil &&
		strategy.RollingUpdate.MaxSurge != nil {
		return int32(strategy.RollingUpdate.MaxSurge.IntValue())
	}
	return 1
}

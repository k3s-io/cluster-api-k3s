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

	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

func (r *KThreesControlPlaneReconciler) tryInPlaceUpdate(
	ctx context.Context,
	controlPlane *k3s.ControlPlane,
	machine *clusterv1.Machine,
	result k3s.UpToDateResult,
) (fallbackToScaleDown bool, reconcileResult ctrl.Result, err error) {
	resultForAllMachines, err := r.preflightChecks(ctx, controlPlane)
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if !resultForAllMachines.IsZero() {
		resultWithoutSelectedMachine, err := r.preflightChecks(ctx, controlPlane, machine)
		if err != nil {
			return false, ctrl.Result{}, err
		}
		if resultWithoutSelectedMachine.IsZero() {
			return true, ctrl.Result{}, nil
		}
		return false, resultForAllMachines, nil
	}

	var canUpdate bool
	if r.overrideCanUpdateMachine != nil {
		canUpdate, err = r.overrideCanUpdateMachine(ctx, machine, result)
	} else {
		canUpdate, err = r.canUpdateMachine(ctx, machine, result)
	}
	if err != nil {
		return false, ctrl.Result{}, fmt.Errorf("failed to determine if Machine %s can be updated in-place: %w", klog.KObj(machine), err)
	}
	if !canUpdate {
		return true, ctrl.Result{}, nil
	}

	if r.overrideTriggerInPlaceUpdate != nil {
		return false, ctrl.Result{}, r.overrideTriggerInPlaceUpdate(ctx, machine, result)
	}
	return false, ctrl.Result{}, r.triggerInPlaceUpdate(ctx, machine, result)
}

func (r *KThreesControlPlaneReconciler) reconcileInPlaceUpdateState(
	controlPlane *k3s.ControlPlane,
) bool {
	return controlPlane.MachinesToCompleteInPlaceUpdate().Len() > 0
}

func (r *KThreesControlPlaneReconciler) reconcilePendingInPlaceUpdateTrigger(
	ctx context.Context,
	controlPlane *k3s.ControlPlane,
) (bool, error) {
	machines := controlPlane.MachinesToCompleteTriggerInPlaceUpdate()
	if machines.Len() == 0 {
		return false, nil
	}

	_, results := controlPlane.NotUpToDateMachines()
	for _, machine := range machines {
		result, ok := results[machine.Name]
		if !ok {
			return false, fmt.Errorf("missing UpToDateResult for Machine %s", machine.Name)
		}
		if r.overrideTriggerInPlaceUpdate != nil {
			if err := r.overrideTriggerInPlaceUpdate(ctx, machine, result); err != nil {
				return false, err
			}
			continue
		}
		if err := r.triggerInPlaceUpdate(ctx, machine, result); err != nil {
			return false, err
		}
	}
	return true, nil
}

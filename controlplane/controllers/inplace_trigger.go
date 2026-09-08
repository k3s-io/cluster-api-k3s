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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientutil "github.com/k3s-io/cluster-api-k3s/pkg/capi/client"
	"github.com/k3s-io/cluster-api-k3s/pkg/capi/hooks"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/util/ssa"
)

func (r *KThreesControlPlaneReconciler) triggerInPlaceUpdate(
	ctx context.Context,
	machine *clusterv1.Machine,
	result k3s.UpToDateResult,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues("Machine", klog.KObj(machine))
	log.Info(fmt.Sprintf("Triggering in-place update for Machine %s", klog.KObj(machine)))

	if _, ok := machine.Annotations[clusterv1.UpdateInProgressAnnotation]; !ok {
		original := machine.DeepCopy()
		if machine.Annotations == nil {
			machine.Annotations = map[string]string{}
		}
		machine.Annotations[clusterv1.UpdateInProgressAnnotation] = ""
		if err := r.Patch(ctx, machine, client.MergeFrom(original)); err != nil {
			return errors.Wrapf(err, "failed to trigger in-place update for Machine %s by setting the %s annotation",
				klog.KObj(machine), clusterv1.UpdateInProgressAnnotation)
		}
		if err := clientutil.WaitForCacheToBeUpToDate(
			ctx, r.Client, fmt.Sprintf("setting the %s annotation", clusterv1.UpdateInProgressAnnotation), machine,
		); err != nil {
			return err
		}
	}

	if result.DesiredMachine == nil {
		return errors.Errorf("failed to complete triggering in-place update for Machine %s, could not compute desired Machine", klog.KObj(machine))
	}
	if result.DesiredInfraMachine == nil {
		return errors.Errorf("failed to complete triggering in-place update for Machine %s, could not compute desired InfraMachine", klog.KObj(machine))
	}
	if result.DesiredKThreesConfig == nil {
		return errors.Errorf("failed to complete triggering in-place update for Machine %s, could not compute desired KThreesConfig", klog.KObj(machine))
	}

	desiredInfraMachine := result.DesiredInfraMachine.DeepCopy()
	desiredInfraMachine.SetLabels(nil)
	desiredInfraMachine.SetAnnotations(map[string]string{
		clusterv1.TemplateClonedFromNameAnnotation:      result.DesiredInfraMachine.GetAnnotations()[clusterv1.TemplateClonedFromNameAnnotation],
		clusterv1.TemplateClonedFromGroupKindAnnotation: result.DesiredInfraMachine.GetAnnotations()[clusterv1.TemplateClonedFromGroupKindAnnotation],
		clusterv1.UpdateInProgressAnnotation:            "",
	})
	if err := ssa.Patch(ctx, r.Client, kcpManagerName, desiredInfraMachine); err != nil {
		return errors.Wrapf(err, "failed to complete triggering in-place update for Machine %s", klog.KObj(machine))
	}

	desiredConfig := result.DesiredKThreesConfig.DeepCopy()
	desiredConfig.Labels = nil
	desiredConfig.Annotations = map[string]string{clusterv1.UpdateInProgressAnnotation: ""}
	if err := ssa.Patch(ctx, r.Client, kcpManagerName, desiredConfig); err != nil {
		return errors.Wrapf(err, "failed to complete triggering in-place update for Machine %s", klog.KObj(machine))
	}

	desiredMachine := result.DesiredMachine.DeepCopy()
	if err := ssa.Patch(ctx, r.Client, kcpManagerName, desiredMachine); err != nil {
		return errors.Wrapf(err, "failed to complete triggering in-place update for Machine %s", klog.KObj(machine))
	}

	if err := hooks.MarkAsPending(ctx, r.Client, desiredMachine, true, runtimehooksv1.UpdateMachine); err != nil {
		return errors.Wrapf(err, "failed to complete triggering in-place update for Machine %s", klog.KObj(machine))
	}

	log.Info(fmt.Sprintf("Completed triggering in-place update for Machine %s", klog.KObj(machine)))
	if r.recorder != nil {
		r.recorder.Event(machine, corev1.EventTypeNormal, "SuccessfulStartInPlaceUpdate", "Machine starting in-place update")
	}
	return clientutil.WaitForCacheToBeUpToDate(ctx, r.Client, "marking the UpdateMachine hook as pending", desiredMachine)
}

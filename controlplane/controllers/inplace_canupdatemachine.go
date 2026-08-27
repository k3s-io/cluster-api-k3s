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
	"strings"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/feature"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/capi/compare"
	"github.com/k3s-io/cluster-api-k3s/pkg/capi/inplace"
	"github.com/k3s-io/cluster-api-k3s/pkg/capi/patchutil"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/util/ssa"
)

func (r *KThreesControlPlaneReconciler) canUpdateMachine(
	ctx context.Context,
	machine *clusterv1.Machine,
	result k3s.UpToDateResult,
) (bool, error) {
	if !feature.Gates.Enabled(feature.InPlaceUpdates) {
		return false, nil
	}
	if result.DesiredMachine == nil ||
		result.CurrentInfraMachine == nil ||
		result.DesiredInfraMachine == nil ||
		result.CurrentKThreesConfig == nil ||
		result.DesiredKThreesConfig == nil {
		return false, nil
	}

	extensionHandlers, err := r.RuntimeClient.GetAllExtensions(ctx, runtimehooksv1.CanUpdateMachine, machine)
	if err != nil {
		return false, err
	}
	if len(extensionHandlers) == 0 {
		return false, nil
	}
	if len(extensionHandlers) > 1 {
		return false, errors.Errorf("found multiple CanUpdateMachine hooks (%s): only one hook is supported", strings.Join(extensionHandlers, ","))
	}

	canUpdate, reasons, err := r.canExtensionsUpdateMachine(ctx, machine, result, extensionHandlers)
	if err != nil {
		return false, err
	}
	if !canUpdate {
		ctrl.LoggerFrom(ctx).Info(
			fmt.Sprintf("Machine %s cannot be updated in-place by extensions", klog.KObj(machine)),
			"reason", strings.Join(reasons, ","),
		)
	}
	return canUpdate, nil
}

func (r *KThreesControlPlaneReconciler) canExtensionsUpdateMachine(
	ctx context.Context,
	machine *clusterv1.Machine,
	result k3s.UpToDateResult,
	extensionHandlers []string,
) (bool, []string, error) {
	request, err := createCanUpdateRequest(ctx, r.Client, machine, result)
	if err != nil {
		var nonCoverable *nonCoverableDiffError
		if errors.As(err, &nonCoverable) {
			return false, []string{nonCoverable.Error()}, nil
		}
		return false, nil, errors.Wrap(err, "failed to generate CanUpdateMachine request")
	}

	var reasons []string
	for _, extensionHandler := range extensionHandlers {
		response := &runtimehooksv1.CanUpdateMachineResponse{}
		if err := r.RuntimeClient.CallExtension(
			ctx, runtimehooksv1.CanUpdateMachine, machine, extensionHandler, request, response,
		); err != nil {
			return false, nil, err
		}
		if err := applyPatchesToRequest(ctx, request, response); err != nil {
			return false, nil, errors.Wrapf(err, "failed to apply patches from extension %s to the CanUpdateMachine request", extensionHandler)
		}
		matches, currentReasons, err := matchesMachine(request)
		if err != nil {
			return false, nil, errors.Wrapf(err, "failed to compare current and desired objects after calling extension %s", extensionHandler)
		}
		reasons = currentReasons
		if matches {
			return true, nil, nil
		}
	}
	return false, reasons, nil
}

type nonCoverableDiffError struct {
	err error
}

func (e *nonCoverableDiffError) Error() string {
	return e.err.Error()
}

func (e *nonCoverableDiffError) Unwrap() error {
	return e.err
}

func createCanUpdateRequest(
	ctx context.Context,
	c client.Client,
	currentMachine *clusterv1.Machine,
	result k3s.UpToDateResult,
) (*runtimehooksv1.CanUpdateMachineRequest, error) {
	currentMachineForDiff := currentMachine.DeepCopy()
	desiredMachineForDiff := result.DesiredMachine.DeepCopy()
	currentConfigForDiff, desiredConfigForDiff := k3s.PrepareKThreesConfigsForDiff(
		result.CurrentKThreesConfig, result.DesiredKThreesConfig,
	)
	currentInfraForDiff := result.CurrentInfraMachine.DeepCopy()
	desiredInfraForDiff := result.DesiredInfraMachine.DeepCopy()

	// Related-object metadata is synchronized before rollout and is not the extension's responsibility.
	currentConfigForDiff.SetLabels(desiredConfigForDiff.GetLabels())
	currentConfigForDiff.SetAnnotations(desiredConfigForDiff.GetAnnotations())
	currentInfraForDiff.SetLabels(desiredInfraForDiff.GetLabels())
	currentInfraForDiff.SetAnnotations(desiredInfraForDiff.GetAnnotations())

	if err := ssa.Patch(ctx, c, kcpManagerName, desiredMachineForDiff, ssa.WithDryRun{}); err != nil {
		return nil, errors.Wrap(err, "server side apply dry-run failed for desired Machine")
	}
	if err := ssa.Patch(ctx, c, kcpManagerName, currentInfraForDiff, ssa.WithDryRun{}); err != nil {
		if apierrors.IsInvalid(err) || apierrors.IsForbidden(err) {
			return nil, &nonCoverableDiffError{err: errors.Wrap(err, "current InfraMachine does not support server side apply dry-run")}
		}
		return nil, errors.Wrap(err, "server side apply dry-run failed for current InfraMachine")
	}
	if err := ssa.Patch(ctx, c, kcpManagerName, desiredInfraForDiff, ssa.WithDryRun{}); err != nil {
		if apierrors.IsInvalid(err) || apierrors.IsForbidden(err) {
			return nil, &nonCoverableDiffError{err: errors.Wrap(err, "desired InfraMachine does not support server side apply dry-run")}
		}
		return nil, errors.Wrap(err, "server side apply dry-run failed for desired InfraMachine")
	}

	request := &runtimehooksv1.CanUpdateMachineRequest{
		Current: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine: *cleanupMachine(currentMachineForDiff),
		},
		Desired: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine: *cleanupMachine(desiredMachineForDiff),
		},
	}
	var err error
	request.Current.BootstrapConfig, err = patchutil.ConvertToRawExtension(cleanupKThreesConfig(currentConfigForDiff))
	if err != nil {
		return nil, err
	}
	request.Desired.BootstrapConfig, err = patchutil.ConvertToRawExtension(cleanupKThreesConfig(desiredConfigForDiff))
	if err != nil {
		return nil, err
	}
	request.Current.InfrastructureMachine, err = patchutil.ConvertToRawExtension(cleanupUnstructured(currentInfraForDiff))
	if err != nil {
		return nil, err
	}
	request.Desired.InfrastructureMachine, err = patchutil.ConvertToRawExtension(cleanupUnstructured(desiredInfraForDiff))
	if err != nil {
		return nil, err
	}
	return request, nil
}

func cleanupMachine(machine *clusterv1.Machine) *clusterv1.Machine {
	return &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        machine.Name,
			Namespace:   machine.Namespace,
			Labels:      machine.Labels,
			Annotations: machine.Annotations,
		},
		Spec: *machine.Spec.DeepCopy(),
	}
}

func cleanupKThreesConfig(config *bootstrapv1.KThreesConfig) *bootstrapv1.KThreesConfig {
	return &bootstrapv1.KThreesConfig{
		TypeMeta: metav1.TypeMeta{APIVersion: bootstrapv1.GroupVersion.String(), Kind: "KThreesConfig"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.Name,
			Namespace:   config.Namespace,
			Labels:      config.Labels,
			Annotations: config.Annotations,
		},
		Spec: *config.Spec.DeepCopy(),
	}
}

func cleanupUnstructured(object *unstructured.Unstructured) *unstructured.Unstructured {
	cleaned := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": object.GetAPIVersion(),
		"kind":       object.GetKind(),
		"spec":       object.Object["spec"],
	}}
	cleaned.SetName(object.GetName())
	cleaned.SetNamespace(object.GetNamespace())
	cleaned.SetLabels(object.GetLabels())
	cleaned.SetAnnotations(object.GetAnnotations())
	return cleaned
}

func applyPatchesToRequest(
	ctx context.Context,
	request *runtimehooksv1.CanUpdateMachineRequest,
	response *runtimehooksv1.CanUpdateMachineResponse,
) error {
	if response.MachinePatch.IsDefined() {
		if err := patchutil.ApplyPatchToTypedObject(ctx, &request.Current.Machine, response.MachinePatch, "spec"); err != nil {
			return err
		}
	}
	if response.BootstrapConfigPatch.IsDefined() {
		if _, err := patchutil.ApplyPatchToObject(ctx, &request.Current.BootstrapConfig, response.BootstrapConfigPatch, "spec"); err != nil {
			return err
		}
	}
	if response.InfrastructureMachinePatch.IsDefined() {
		if _, err := patchutil.ApplyPatchToObject(ctx, &request.Current.InfrastructureMachine, response.InfrastructureMachinePatch, "spec"); err != nil {
			return err
		}
	}
	return nil
}

func matchesMachine(request *runtimehooksv1.CanUpdateMachineRequest) (bool, []string, error) {
	reasons := []string{}
	match, diff, err := compare.Diff(
		&clusterv1.Machine{Spec: *inplace.CleanupMachineSpecForDiff(&request.Current.Machine.Spec)},
		&clusterv1.Machine{Spec: *inplace.CleanupMachineSpecForDiff(&request.Desired.Machine.Spec)},
	)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to match Machine")
	}
	if !match {
		reasons = append(reasons, "Machine cannot be updated in-place: "+diff)
	}

	match, diff, err = matchesUnstructuredSpec(request.Current.BootstrapConfig, request.Desired.BootstrapConfig)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to match KThreesConfig")
	}
	if !match {
		reasons = append(reasons, "KThreesConfig cannot be updated in-place: "+diff)
	}

	match, diff, err = matchesUnstructuredSpec(request.Current.InfrastructureMachine, request.Desired.InfrastructureMachine)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to match InfrastructureMachine")
	}
	if !match {
		reasons = append(reasons, "InfrastructureMachine cannot be updated in-place: "+diff)
	}
	return len(reasons) == 0, reasons, nil
}

func matchesUnstructuredSpec(current, desired runtime.RawExtension) (bool, string, error) {
	currentObject, ok := current.Object.(*unstructured.Unstructured)
	if !ok {
		return false, "", errors.New("current object is not Unstructured")
	}
	desiredObject, ok := desired.Object.(*unstructured.Unstructured)
	if !ok {
		return false, "", errors.New("desired object is not Unstructured")
	}
	return compare.Diff(
		&unstructured.Unstructured{Object: map[string]interface{}{"spec": currentObject.Object["spec"]}},
		&unstructured.Unstructured{Object: map[string]interface{}{"spec": desiredObject.Object["spec"]}},
	)
}

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

package inplaceupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	callRecordNamespaceSetting = "callRecordNamespace"
	callRecordConfigMapSetting = "callRecordConfigMap"
)

type jsonPatchOperation struct {
	Operation string `json:"op"`
	Path      string `json:"path"`
	Value     any    `json:"value,omitempty"`
}

// ExtensionHandlers implements the in-place update hooks used by E2E tests.
type ExtensionHandlers struct {
	client client.Client
}

// NewExtensionHandlers creates in-place update hook handlers.
func NewExtensionHandlers(client client.Client) *ExtensionHandlers {
	return &ExtensionHandlers{client: client}
}

// DoCanUpdateMachine reports that the extension can handle Machine version changes only.
func (h *ExtensionHandlers) DoCanUpdateMachine(
	ctx context.Context,
	req *runtimehooksv1.CanUpdateMachineRequest,
	resp *runtimehooksv1.CanUpdateMachineResponse,
) {
	machineUID, err := h.resolveMachineUID(ctx, &req.Current.Machine)
	if err != nil {
		setFailure(&resp.CommonResponse, fmt.Errorf("failed to record CanUpdateMachine call: %w", err))
		return
	}
	if _, err := h.recordCall(ctx, req.Settings, machineUID, "canUpdateMachine"); err != nil {
		setFailure(&resp.CommonResponse, fmt.Errorf("failed to record CanUpdateMachine call: %w", err))
		return
	}

	if err := validateCanUpdateMachineRequest(req); err != nil {
		setFailure(&resp.CommonResponse, err)
		return
	}

	machinePatch := []jsonPatchOperation{}
	if req.Current.Machine.Spec.Version != req.Desired.Machine.Spec.Version {
		machinePatch = append(machinePatch, jsonPatchOperation{
			Operation: "replace",
			Path:      "/spec/version",
			Value:     req.Desired.Machine.Spec.Version,
		})
	}

	resp.MachinePatch, err = newJSONPatch(machinePatch)
	if err != nil {
		setFailure(&resp.CommonResponse, err)
		return
	}
	resp.BootstrapConfigPatch, err = newJSONPatch(nil)
	if err != nil {
		setFailure(&resp.CommonResponse, err)
		return
	}
	resp.InfrastructureMachinePatch, err = newJSONPatch(nil)
	if err != nil {
		setFailure(&resp.CommonResponse, err)
		return
	}
	resp.Status = runtimehooksv1.ResponseStatusSuccess
}

// DoUpdateMachine deterministically fakes asynchronous update progress.
func (h *ExtensionHandlers) DoUpdateMachine(
	ctx context.Context,
	req *runtimehooksv1.UpdateMachineRequest,
	resp *runtimehooksv1.UpdateMachineResponse,
) {
	machineUID, err := h.resolveMachineUID(ctx, &req.Desired.Machine)
	if err != nil {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("failed to record UpdateMachine call: %v", err)
		return
	}
	roundCall, err := h.recordUpdateMachineCall(ctx, req.Settings, machineUID)
	if err != nil {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("failed to record UpdateMachine call: %v", err)
		return
	}

	resp.Status = runtimehooksv1.ResponseStatusSuccess
	if roundCall == 1 {
		resp.Message = "Test extension is updating Machine"
		resp.RetryAfterSeconds = 5
		return
	}
	resp.Message = "Test extension completed updating Machine"
	resp.RetryAfterSeconds = 0
}

func (h *ExtensionHandlers) recordCall(
	ctx context.Context,
	settings map[string]string,
	machineUID types.UID,
	hook string,
) (int, error) {
	key := fmt.Sprintf("%s.%s", machineUID, hook)
	return h.mutateCallRecords(ctx, settings, machineUID, func(data map[string]string) (int, error) {
		return incrementCallCount(data, key)
	})
}

func (h *ExtensionHandlers) recordUpdateMachineCall(
	ctx context.Context,
	settings map[string]string,
	machineUID types.UID,
) (int, error) {
	cumulativeKey := fmt.Sprintf("%s.updateMachine", machineUID)
	roundKey := fmt.Sprintf("%s.updateMachineRound", machineUID)
	return h.mutateCallRecords(ctx, settings, machineUID, func(data map[string]string) (int, error) {
		if _, err := incrementCallCount(data, cumulativeKey); err != nil {
			return 0, err
		}
		roundCall, err := incrementCallCount(data, roundKey)
		if err != nil {
			return 0, err
		}
		if roundCall == 2 {
			delete(data, roundKey)
		}
		return roundCall, nil
	})
}

func (h *ExtensionHandlers) mutateCallRecords(
	ctx context.Context,
	settings map[string]string,
	machineUID types.UID,
	mutate func(map[string]string) (int, error),
) (int, error) {
	namespace := settings[callRecordNamespaceSetting]
	name := settings[callRecordConfigMapSetting]
	if namespace == "" || name == "" {
		return 0, fmt.Errorf("%s and %s settings must be configured", callRecordNamespaceSetting, callRecordConfigMapSetting)
	}
	if machineUID == "" {
		return 0, fmt.Errorf("machine UID must be set")
	}

	result := 0
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configMap := &corev1.ConfigMap{}
		if err := h.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, configMap); err != nil {
			return err
		}

		if configMap.Data == nil {
			configMap.Data = map[string]string{}
		}
		mutationResult, err := mutate(configMap.Data)
		if err != nil {
			return err
		}
		if err := h.client.Update(ctx, configMap); err != nil {
			return err
		}
		result = mutationResult
		return nil
	})
	if err != nil {
		return 0, err
	}
	return result, nil
}

func incrementCallCount(data map[string]string, key string) (int, error) {
	current := 0
	if value := data[key]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("invalid call count %q for %s: %w", value, key, err)
		}
		current = parsed
	}
	count := current + 1
	data[key] = strconv.Itoa(count)
	return count, nil
}

func (h *ExtensionHandlers) resolveMachineUID(ctx context.Context, machine *clusterv1.Machine) (types.UID, error) {
	if machine.UID != "" {
		return machine.UID, nil
	}
	storedMachine := &clusterv1.Machine{}
	if err := h.client.Get(ctx, client.ObjectKeyFromObject(machine), storedMachine); err != nil {
		return "", fmt.Errorf("failed to get Machine %s: %w", client.ObjectKeyFromObject(machine), err)
	}
	if storedMachine.UID == "" {
		return "", fmt.Errorf("stored Machine %s has no UID", client.ObjectKeyFromObject(machine))
	}
	return storedMachine.UID, nil
}

func validateCanUpdateMachineRequest(req *runtimehooksv1.CanUpdateMachineRequest) error {
	if err := validateRawExtension("current infrastructureMachine", req.Current.InfrastructureMachine, true); err != nil {
		return err
	}
	if err := validateRawExtension("desired infrastructureMachine", req.Desired.InfrastructureMachine, true); err != nil {
		return err
	}
	if err := validateRawExtension("current bootstrapConfig", req.Current.BootstrapConfig, false); err != nil {
		return err
	}
	return validateRawExtension("desired bootstrapConfig", req.Desired.BootstrapConfig, false)
}

func validateRawExtension(name string, rawExtension runtime.RawExtension, required bool) error {
	raw := rawExtension.Raw
	if len(raw) == 0 && rawExtension.Object != nil {
		var err error
		raw, err = json.Marshal(rawExtension.Object)
		if err != nil {
			return fmt.Errorf("failed to marshal %s: %w", name, err)
		}
	}
	if len(raw) == 0 {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	if !json.Valid(raw) {
		return fmt.Errorf("%s contains malformed JSON", name)
	}
	return nil
}

func newJSONPatch(operations []jsonPatchOperation) (runtimehooksv1.Patch, error) {
	if operations == nil {
		operations = []jsonPatchOperation{}
	}
	patch, err := json.Marshal(operations)
	if err != nil {
		return runtimehooksv1.Patch{}, fmt.Errorf("failed to marshal JSON patch: %w", err)
	}
	return runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     patch,
	}, nil
}

func setFailure(response *runtimehooksv1.CommonResponse, err error) {
	response.Status = runtimehooksv1.ResponseStatusFailure
	response.Message = err.Error()
}

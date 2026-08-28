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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilfeature "k8s.io/component-base/featuregate/testing"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/util/collections"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/util/ssa"
)

func TestRollingUpdate(t *testing.T) {
	tests := []struct {
		name            string
		current         int
		desired         int32
		maxSurge        int
		omitMaxSurge    bool
		outdated        []int
		enabled         bool
		mutateResult    func(*k3s.UpToDateResult)
		tryFallback     bool
		tryErr          error
		wantAction      string
		wantScaleDowns  int
		wantErrContains string
	}{
		{
			name:       "current replicas below desired plus maxSurge scales up",
			current:    2,
			desired:    3,
			maxSurge:   1,
			outdated:   []int{0},
			enabled:    true,
			wantAction: "scale-up",
		},
		{
			name:         "omitted maxSurge defaults to one",
			current:      3,
			desired:      3,
			omitMaxSurge: true,
			outdated:     []int{0},
			wantAction:   "scale-up",
		},
		{
			name:       "single-node maxSurge zero may scale up while feature is disabled",
			current:    0,
			desired:    1,
			maxSurge:   0,
			wantAction: "scale-up",
		},
		{
			name:       "maxSurge zero eligible covered diff uses in-place for one replica",
			current:    1,
			desired:    1,
			maxSurge:   0,
			outdated:   []int{0},
			enabled:    true,
			wantAction: "in-place",
		},
		{
			name:       "maxSurge one remains create first for one replica",
			current:    1,
			desired:    1,
			maxSurge:   1,
			outdated:   []int{0},
			enabled:    true,
			wantAction: "scale-up",
		},
		{
			name:           "maxSurge one replaces after creating surge Machine for one replica",
			current:        2,
			desired:        1,
			maxSurge:       1,
			outdated:       []int{0},
			enabled:        true,
			wantAction:     "scale-down",
			wantScaleDowns: 1,
		},
		{
			name:           "feature disabled with maxSurge one scales down",
			current:        4,
			desired:        3,
			maxSurge:       1,
			outdated:       []int{0},
			wantAction:     "scale-down",
			wantScaleDowns: 1,
		},
		{
			name:            "feature disabled single-node maxSurge zero fails safely",
			current:         1,
			desired:         1,
			maxSurge:        0,
			outdated:        []int{0},
			wantErrContains: "maxSurge=0 with fewer than three replicas requires InPlaceUpdates",
		},
		{
			name:           "feature disabled permits outdated surplus deletion",
			current:        2,
			desired:        1,
			maxSurge:       0,
			outdated:       []int{0},
			wantAction:     "scale-down",
			wantScaleDowns: 1,
		},
		{
			name:     "one replica ownership replacement with zero surge fails closed",
			current:  1,
			desired:  1,
			maxSurge: 0,
			outdated: []int{0},
			enabled:  true,
			mutateResult: func(result *k3s.UpToDateResult) {
				result.EligibleForInPlaceUpdate = false
				result.LogMessages = append(result.LogMessages, "related-object spec ownership spans multiple API versions")
				result.ConditionMessages = append(result.ConditionMessages, "Related-object ownership requires Machine replacement")
			},
			wantErrContains: "maxSurge=0 with fewer than three replicas cannot fall back to Machine replacement; enable or fix a working in-place update extension or set maxSurge to 1",
		},
		{
			name:     "three replica ownership replacement with zero surge scales down",
			current:  3,
			desired:  3,
			maxSurge: 0,
			outdated: []int{0},
			enabled:  true,
			mutateResult: func(result *k3s.UpToDateResult) {
				result.EligibleForInPlaceUpdate = false
				result.LogMessages = append(result.LogMessages, "related-object spec ownership spans multiple API versions")
				result.ConditionMessages = append(result.ConditionMessages, "Related-object ownership requires Machine replacement")
			},
			wantAction:     "scale-down",
			wantScaleDowns: 1,
		},
		{
			name:     "one replica ownership replacement with surge scales up",
			current:  1,
			desired:  1,
			maxSurge: 1,
			outdated: []int{0},
			enabled:  true,
			mutateResult: func(result *k3s.UpToDateResult) {
				result.EligibleForInPlaceUpdate = false
				result.LogMessages = append(result.LogMessages, "related-object spec ownership spans multiple API versions")
				result.ConditionMessages = append(result.ConditionMessages, "Related-object ownership requires Machine replacement")
			},
			wantAction: "scale-up",
		},
		{
			name:            "one replica coverage false fallback fails closed",
			current:         1,
			desired:         1,
			maxSurge:        0,
			outdated:        []int{0},
			enabled:         true,
			tryFallback:     true,
			wantAction:      "in-place",
			wantErrContains: "maxSurge=0 with fewer than three replicas cannot fall back to Machine replacement; enable or fix a working in-place update extension or set maxSurge to 1",
		},
		{
			name:            "two replica fallback fails closed",
			current:         2,
			desired:         2,
			maxSurge:        0,
			outdated:        []int{0},
			enabled:         true,
			tryFallback:     true,
			wantAction:      "in-place",
			wantErrContains: "maxSurge=0 with fewer than three replicas cannot fall back to Machine replacement; enable or fix a working in-place update extension or set maxSurge to 1",
		},
		{
			name:           "three replica unsupported fallback scales down",
			current:        3,
			desired:        3,
			maxSurge:       0,
			outdated:       []int{0},
			enabled:        true,
			tryFallback:    true,
			wantAction:     "scale-down",
			wantScaleDowns: 1,
		},
		{
			name:           "one desired replica with two current Machines permits surplus deletion",
			current:        2,
			desired:        1,
			maxSurge:       0,
			outdated:       []int{0},
			enabled:        true,
			wantAction:     "scale-down",
			wantScaleDowns: 1,
		},
		{
			name:            "coverage error is returned",
			current:         3,
			desired:         3,
			maxSurge:        0,
			outdated:        []int{0},
			enabled:         true,
			tryErr:          errors.New("coverage failed"),
			wantAction:      "in-place",
			wantErrContains: "coverage failed",
		},
		{
			name:           "surplus outdated Machine is deleted after desired replicas are up to date",
			current:        4,
			desired:        3,
			maxSurge:       1,
			outdated:       []int{0},
			enabled:        true,
			wantAction:     "scale-down",
			wantScaleDowns: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, tt.enabled)
			controlPlane, cluster, kcp, machinesNeedingRollout, results, c := newRolloutControlPlane(t, tt.current, tt.desired, tt.maxSurge, tt.outdated, false)
			if tt.omitMaxSurge {
				kcp.Spec.RolloutStrategy.RollingUpdate.MaxSurge = nil
			}
			for _, index := range tt.outdated {
				result := results[fmt.Sprintf("machine-%d", index)]
				if tt.mutateResult != nil {
					tt.mutateResult(&result)
					results[fmt.Sprintf("machine-%d", index)] = result
				}
			}
			action := ""
			scaleDowns := 0
			r := &KThreesControlPlaneReconciler{
				Client: c,
				overrides: &reconcilerOverrides{
					scaleUpControlPlane: func(context.Context, *clusterv1.Cluster, *controlplanev1.KThreesControlPlane, *k3s.ControlPlane) (ctrl.Result, error) {
						action = "scale-up"
						return ctrl.Result{}, nil
					},
					scaleDownControlPlane: func(_ context.Context, _ *clusterv1.Cluster, _ *controlplanev1.KThreesControlPlane, _ *k3s.ControlPlane, machines collections.Machines) (ctrl.Result, error) {
						scaleDowns++
						action = "scale-down"
						g.Expect(machines).NotTo(BeEmpty())
						return ctrl.Result{}, nil
					},
					tryInPlaceUpdate: func(context.Context, *k3s.ControlPlane, *clusterv1.Machine, k3s.UpToDateResult) (bool, ctrl.Result, error) {
						action = "in-place"
						return tt.tryFallback, ctrl.Result{}, tt.tryErr
					},
				},
			}

			_, err := r.updateControlPlane(context.Background(), cluster, kcp, controlPlane, machinesNeedingRollout, results)
			if tt.wantErrContains != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErrContains)))
				g.Expect(action).To(Equal(tt.wantAction))
				g.Expect(scaleDowns).To(Equal(tt.wantScaleDowns))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(action).To(Equal(tt.wantAction))
			g.Expect(scaleDowns).To(Equal(tt.wantScaleDowns))
		})
	}
}

func TestRolloutLogDetailsSortsMachinesAndIncludesReasons(t *testing.T) {
	machines := collections.FromMachines(
		&clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "machine-b"}},
		&clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "machine-a"}},
	)
	results := map[string]k3s.UpToDateResult{
		"machine-a": {LogMessages: []string{"version changed", "config changed"}},
		"machine-b": {LogMessages: []string{"rolloutAfter expired"}},
	}

	names, reasons := rolloutLogDetails(machines, results)

	g := NewWithT(t)
	g.Expect(names).To(Equal([]string{"machine-a", "machine-b"}))
	g.Expect(reasons).To(Equal(
		"Machine machine-a needs rollout: version changed, config changed, " +
			"Machine machine-b needs rollout: rolloutAfter expired",
	))
}

func TestRolloutLogDetailsRedactsKThreesConfigValues(t *testing.T) {
	const sentinel = "sentinel-rollout-secret"
	oldContent := "old-content-" + sentinel
	newContent := "new-content-" + sentinel
	oldCommand := "old-command-" + sentinel
	newCommand := "new-command-" + sentinel

	controlPlane, cluster, kcp, _, _, c := newRolloutControlPlane(t, 1, 1, 0, nil, false)
	currentConfig := controlPlane.KthreesConfigs["machine-0"]
	currentConfig.Spec.Files = []bootstrapv1.File{{Path: "/etc/sensitive", Content: oldContent}}
	currentConfig.Spec.PreK3sCommands = []string{oldCommand}
	kcp.Spec.KThreesConfigSpec.Files = []bootstrapv1.File{{Path: "/etc/sensitive", Content: newContent}}
	kcp.Spec.KThreesConfigSpec.PreK3sCommands = []string{newCommand}

	machine := controlPlane.Machines["machine-0"]
	upToDate, result, err := k3s.UpToDate(
		context.Background(),
		c,
		cluster,
		machine,
		kcp,
		&metav1.Time{Time: time.Now()},
		controlPlane.InfraResources,
		controlPlane.KthreesConfigs,
	)
	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(upToDate).To(BeFalse())
	results := map[string]k3s.UpToDateResult{machine.Name: *result}
	machines := collections.FromMachines(machine)
	names, reasons := rolloutLogDetails(machines, results)

	g.Expect(result.LogMessages).To(Equal([]string{"KThreesConfig spec is not up-to-date"}))
	g.Expect(names).To(Equal([]string{"machine-0"}))
	g.Expect(reasons).To(Equal("Machine machine-0 needs rollout: KThreesConfig spec is not up-to-date"))
	for _, sensitiveValue := range []string{sentinel, oldContent, newContent, oldCommand, newCommand} {
		g.Expect(strings.Join(result.LogMessages, ", ")).NotTo(ContainSubstring(sensitiveValue))
		g.Expect(reasons).NotTo(ContainSubstring(sensitiveValue))
	}
}

func TestSyncMachinesRefreshesRolloutMachineBeforeInPlaceSelection(t *testing.T) {
	g := NewWithT(t)
	utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
	controlPlane, cluster, kcp, _, _, c := newRolloutControlPlane(t, 1, 1, 0, []int{0}, false)
	controlPlane.InfraResources = map[string]*unstructured.Unstructured{}
	controlPlane.KthreesConfigs = map[string]*bootstrapv1.KThreesConfig{}

	originalMachine := controlPlane.Machines["machine-0"]
	kcp.Spec.MachineTemplate.ObjectMeta = clusterv1beta1.ObjectMeta{
		Labels:      map[string]string{"updated": "label"},
		Annotations: map[string]string{"updated": "annotation"},
	}
	kcp.Spec.MachineTemplate.NodeDrainTimeout = &metav1.Duration{Duration: time.Minute}

	var selectedMachine *clusterv1.Machine
	syncClient := &machineApplyResourceVersionClient{Client: c, resourceVersion: "post-sync-resource-version"}
	r := &KThreesControlPlaneReconciler{
		Client:   syncClient,
		ssaCache: ssa.NewCache(),
		overrides: &reconcilerOverrides{
			tryInPlaceUpdate: func(_ context.Context, _ *k3s.ControlPlane, machine *clusterv1.Machine, _ k3s.UpToDateResult) (bool, ctrl.Result, error) {
				selectedMachine = machine
				return false, ctrl.Result{}, nil
			},
		},
	}

	migrationCompleted, err := r.syncMachines(context.Background(), controlPlane)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(migrationCompleted).To(BeFalse())
	machinesNeedingRollout, results := controlPlane.MachinesNeedingRolloutWithResults()
	_, err = r.updateControlPlane(context.Background(), cluster, kcp, controlPlane, machinesNeedingRollout, results)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(selectedMachine).NotTo(BeNil())
	g.Expect(selectedMachine).To(BeIdenticalTo(controlPlane.Machines[originalMachine.Name]))
	g.Expect(selectedMachine).NotTo(BeIdenticalTo(originalMachine))
	g.Expect(selectedMachine.ResourceVersion).To(Equal("post-sync-resource-version"))
	g.Expect(selectedMachine.Labels).To(HaveKeyWithValue("updated", "label"))
	g.Expect(selectedMachine.Annotations).To(HaveKeyWithValue("updated", "annotation"))
	g.Expect(selectedMachine.Spec.Deletion.NodeDrainTimeoutSeconds).NotTo(BeNil())
	g.Expect(*selectedMachine.Spec.Deletion.NodeDrainTimeoutSeconds).To(Equal(int32(60)))
}

func TestSyncMachinesMarksUnsupportedOwnershipAndContinuesNormalSync(t *testing.T) {
	g := NewWithT(t)
	fixture := newRolloutControlPlaneFixture(t, 1, 1, 0, []int{0}, false)
	fixture.controlPlane.KthreesConfigs = map[string]*bootstrapv1.KThreesConfig{}
	infraMachine := fixture.controlPlane.InfraResources["machine-0"]
	infraMachine.SetManagedFields([]metav1.ManagedFieldsEntry{
		{
			Manager:    "manager",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:size":{}}}`)},
		},
		{
			Manager:    "manager",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
			FieldsType: "FieldsV1",
			FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:storage":{"f:size":{}}}}`)},
		},
	})

	trackingClient := &relatedObjectSyncTrackingClient{Client: fixture.client}
	r := &KThreesControlPlaneReconciler{
		Client:    trackingClient,
		apiReader: trackingClient,
		ssaCache:  ssa.NewCache(),
	}

	migrationCompleted, err := r.syncMachines(context.Background(), fixture.controlPlane)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(migrationCompleted).To(BeFalse())
	g.Expect(trackingClient.applyPatchCount).To(BeNumerically(">", 0))
	_, results := fixture.controlPlane.MachinesNeedingRolloutWithResults()
	result := results["machine-0"]
	g.Expect(result.EligibleForInPlaceUpdate).To(BeFalse())
	g.Expect(result.LogMessages).To(ContainElement("related-object spec ownership spans multiple API versions"))
	g.Expect(result.ConditionMessages).To(ContainElement("Related-object ownership requires Machine replacement"))
	g.Expect(fixture.controlPlane.MachinesNeedingRollout().Names()).To(ConsistOf("machine-0"))
}

type rolloutControlPlaneFixture struct {
	controlPlane           *k3s.ControlPlane
	cluster                *clusterv1.Cluster
	kcp                    *controlplanev1.KThreesControlPlane
	machinesNeedingRollout collections.Machines
	results                map[string]k3s.UpToDateResult
	client                 client.Client
}

func newRolloutControlPlaneFixture(
	t *testing.T,
	current int,
	desired int32,
	maxSurge int,
	outdated []int,
	withEtcdSecret bool,
) *rolloutControlPlaneFixture {
	t.Helper()
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(bootstrapv1.AddToScheme(scheme)).To(Succeed())

	cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster-1", Namespace: "default"}}
	maxSurgeValue := intstr.FromInt(maxSurge)
	kcp := &controlplanev1.KThreesControlPlane{
		TypeMeta:   metav1.TypeMeta{APIVersion: controlplanev1.GroupVersion.String(), Kind: "KThreesControlPlane"},
		ObjectMeta: metav1.ObjectMeta{Name: "kcp-1", Namespace: "default"},
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Replicas: &desired,
			Version:  "v1.31.2+k3s1",
			RolloutStrategy: &controlplanev1.RolloutStrategy{
				Type: controlplanev1.RollingUpdateStrategyType,
				RollingUpdate: &controlplanev1.RollingUpdate{
					MaxSurge: &maxSurgeValue,
				},
			},
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "TestMachineTemplate",
					Name:       "template-1",
				},
			},
		},
	}

	outdatedSet := map[int]struct{}{}
	for _, index := range outdated {
		outdatedSet[index] = struct{}{}
	}
	objects := []client.Object{
		&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{
			Name: "testmachinetemplates.infrastructure.cluster.x-k8s.io",
			Labels: map[string]string{
				clusterv1.GroupVersion.String(): "v1beta1",
			},
		}},
		&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{
			Name: "testmachines.infrastructure.cluster.x-k8s.io",
			Labels: map[string]string{
				clusterv1.GroupVersion.String(): "v1beta1",
			},
		}},
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "TestMachineTemplate",
			"metadata":   map[string]interface{}{"name": "template-1", "namespace": "default"},
			"spec":       map[string]interface{}{"template": map[string]interface{}{"spec": map[string]interface{}{"size": "same"}}},
		}},
	}
	if withEtcdSecret {
		objects = append(objects, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cluster.Name + "-etcd", Namespace: cluster.Namespace}})
	}

	machines := collections.Machines{}
	baseTime := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	for i := 0; i < current; i++ {
		name := fmt.Sprintf("machine-%d", i)
		version := kcp.Spec.Version
		if _, ok := outdatedSet[i]; ok {
			version = "v1.31.1+k3s1"
		}
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", CreationTimestamp: metav1.NewTime(baseTime.Add(time.Duration(i) * time.Minute)), Annotations: map[string]string{}},
			Spec: clusterv1.MachineSpec{
				ClusterName:       cluster.Name,
				Version:           version,
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{APIGroup: "infrastructure.cluster.x-k8s.io", Kind: "TestMachine", Name: name + "-infra"},
				Bootstrap:         clusterv1.Bootstrap{ConfigRef: clusterv1.ContractVersionedObjectReference{APIGroup: bootstrapv1.GroupVersion.Group, Kind: "KThreesConfig", Name: name + "-config"}},
			},
		}
		config := &bootstrapv1.KThreesConfig{ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: "default"}}
		infra := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "TestMachine",
			"metadata": map[string]interface{}{
				"name": name + "-infra", "namespace": "default",
				"annotations": map[string]interface{}{
					clusterv1.TemplateClonedFromNameAnnotation:      "template-1",
					clusterv1.TemplateClonedFromGroupKindAnnotation: "TestMachineTemplate.infrastructure.cluster.x-k8s.io",
				},
			},
			"spec": map[string]interface{}{"size": "same"},
		}}
		machines[name] = machine
		objects = append(objects, machine, config, infra)
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	controlPlane, err := k3s.NewControlPlane(context.Background(), c, cluster, kcp, machines)
	g.Expect(err).NotTo(HaveOccurred())
	machinesNeedingRollout, results := controlPlane.MachinesNeedingRolloutWithResults()
	return &rolloutControlPlaneFixture{
		controlPlane:           controlPlane,
		cluster:                cluster,
		kcp:                    kcp,
		machinesNeedingRollout: machinesNeedingRollout,
		results:                results,
		client:                 c,
	}
}

func newRolloutControlPlane(
	t *testing.T,
	current int,
	desired int32,
	maxSurge int,
	outdated []int,
	withEtcdSecret bool,
) (*k3s.ControlPlane, *clusterv1.Cluster, *controlplanev1.KThreesControlPlane, collections.Machines, map[string]k3s.UpToDateResult, client.Client) {
	t.Helper()
	fixture := newRolloutControlPlaneFixture(t, current, desired, maxSurge, outdated, withEtcdSecret)
	return fixture.controlPlane,
		fixture.cluster,
		fixture.kcp,
		fixture.machinesNeedingRollout,
		fixture.results,
		fixture.client
}

// machineApplyResourceVersionClient models the resourceVersion returned by an SSA Machine update.
type machineApplyResourceVersionClient struct {
	client.Client
	resourceVersion string
}

func (c *machineApplyResourceVersionClient) Patch(ctx context.Context, object client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if err := c.Client.Patch(ctx, object, patch, opts...); err != nil {
		return err
	}
	if patch == client.Apply {
		object.SetResourceVersion(c.resourceVersion)
	}
	return nil
}

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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	utilfeature "k8s.io/component-base/featuregate/testing"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/util/collections"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/capi/hooks"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

func TestTriggerInPlaceUpdateWritesObjectsInOrder(t *testing.T) {
	g := NewWithT(t)
	machine, result, baseClient := triggerFixtures(t)
	trackingClient := &patchTrackingClient{Client: baseClient}
	r := &KThreesControlPlaneReconciler{Client: trackingClient, recorder: record.NewFakeRecorder(10)}

	g.Expect(r.triggerInPlaceUpdate(context.Background(), machine, result)).To(Succeed())
	g.Expect(trackingClient.patchedKinds).To(Equal([]string{
		"Machine",
		"TestMachine",
		"KThreesConfig",
		"Machine",
		"Machine",
	}))
	g.Expect(trackingClient.operations).To(Equal([]string{
		"patch:Machine",
		"get:Machine",
		"patch:TestMachine",
		"patch:KThreesConfig",
		"patch:Machine",
		"patch:Machine",
		"get:Machine",
	}))

	actualMachine := &clusterv1.Machine{}
	g.Expect(baseClient.Get(context.Background(), client.ObjectKeyFromObject(machine), actualMachine)).To(Succeed())
	g.Expect(actualMachine.Annotations).To(HaveKey(clusterv1.UpdateInProgressAnnotation))
	g.Expect(actualMachine.Spec.Version).To(Equal("v1.31.2+k3s1"))
	g.Expect(hooks.IsPending(runtimehooksv1.UpdateMachine, actualMachine)).To(BeTrue())

	actualInfra := &unstructured.Unstructured{}
	actualInfra.SetAPIVersion(result.DesiredInfraMachine.GetAPIVersion())
	actualInfra.SetKind(result.DesiredInfraMachine.GetKind())
	g.Expect(baseClient.Get(context.Background(), client.ObjectKeyFromObject(result.DesiredInfraMachine), actualInfra)).To(Succeed())
	g.Expect(actualInfra.GetAnnotations()).To(HaveKey(clusterv1.UpdateInProgressAnnotation))
	g.Expect(actualInfra.GetAnnotations()).To(HaveKeyWithValue(clusterv1.TemplateClonedFromNameAnnotation, "template-2"))
	size, _, err := unstructured.NestedString(actualInfra.Object, "spec", "size")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(size).To(Equal("large"))

	actualConfig := &bootstrapv1.KThreesConfig{}
	g.Expect(baseClient.Get(context.Background(), client.ObjectKeyFromObject(result.DesiredKThreesConfig), actualConfig)).To(Succeed())
	g.Expect(actualConfig.Annotations).To(HaveKey(clusterv1.UpdateInProgressAnnotation))
	g.Expect(actualConfig.Spec.PostK3sCommands).To(Equal([]string{"new"}))
}

func TestTriggerInPlaceUpdateRetriesAfterEveryWriteBoundary(t *testing.T) {
	for failAt := 1; failAt <= 5; failAt++ {
		t.Run(fmt.Sprintf("failure at patch %d", failAt), func(t *testing.T) {
			g := NewWithT(t)
			machine, result, baseClient := triggerFixtures(t)
			trackingClient := &patchTrackingClient{Client: baseClient, failAt: failAt}
			r := &KThreesControlPlaneReconciler{Client: trackingClient, recorder: record.NewFakeRecorder(10)}

			g.Expect(r.triggerInPlaceUpdate(context.Background(), machine, result)).To(MatchError(ContainSubstring("injected patch failure")))
			assertTriggerState(t, baseClient, machine, result, triggerState{
				marked:         failAt > 1,
				infraUpdated:   failAt > 2,
				configUpdated:  failAt > 3,
				machineUpdated: failAt > 4,
			})

			trackingClient.failAt = 0
			trackingClient.patchCalls = 0
			latestMachine := &clusterv1.Machine{}
			g.Expect(baseClient.Get(context.Background(), client.ObjectKeyFromObject(machine), latestMachine)).To(Succeed())
			latestResult := result
			latestResult.DesiredMachine = result.DesiredMachine.DeepCopy()
			latestResult.DesiredInfraMachine = result.DesiredInfraMachine.DeepCopy()
			latestResult.DesiredKThreesConfig = result.DesiredKThreesConfig.DeepCopy()
			g.Expect(r.triggerInPlaceUpdate(context.Background(), latestMachine, latestResult)).To(Succeed())

			storedMachine := &clusterv1.Machine{}
			g.Expect(baseClient.Get(context.Background(), client.ObjectKeyFromObject(machine), storedMachine)).To(Succeed())
			g.Expect(storedMachine.Spec.Version).To(Equal("v1.31.2+k3s1"))
			g.Expect(hooks.IsPending(runtimehooksv1.UpdateMachine, storedMachine)).To(BeTrue())
		})
	}
}

func TestTriggerInPlaceUpdateRetriesAfterCacheBarrierFailures(t *testing.T) {
	tests := []struct {
		name      string
		failGetAt int
		wantState triggerState
	}{
		{
			name:      "failure after marking Machine update in progress",
			failGetAt: 1,
			wantState: triggerState{marked: true},
		},
		{
			name:      "failure after marking UpdateMachine hook pending",
			failGetAt: 2,
			wantState: triggerState{
				marked:         true,
				infraUpdated:   true,
				configUpdated:  true,
				machineUpdated: true,
				hookPending:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			machine, result, baseClient := triggerFixtures(t)
			trackingClient := &patchTrackingClient{Client: baseClient, failGetAt: tt.failGetAt}
			r := &KThreesControlPlaneReconciler{Client: trackingClient, recorder: record.NewFakeRecorder(10)}

			g.Expect(r.triggerInPlaceUpdate(context.Background(), machine, result)).To(MatchError(ContainSubstring("injected cache barrier failure")))
			assertTriggerState(t, baseClient, machine, result, tt.wantState)

			trackingClient.failGetAt = 0
			trackingClient.getCalls = 0
			latestMachine := &clusterv1.Machine{}
			g.Expect(baseClient.Get(context.Background(), client.ObjectKeyFromObject(machine), latestMachine)).To(Succeed())
			g.Expect(r.triggerInPlaceUpdate(context.Background(), latestMachine, result)).To(Succeed())
			assertTriggerState(t, baseClient, machine, result, triggerState{
				marked:         true,
				infraUpdated:   true,
				configUpdated:  true,
				machineUpdated: true,
				hookPending:    true,
			})
		})
	}
}

func TestTriggerInPlaceUpdateResumeUsesLatestDesiredState(t *testing.T) {
	g := NewWithT(t)
	machine, result, baseClient := triggerFixtures(t)
	trackingClient := &patchTrackingClient{Client: baseClient, failAt: 2}
	r := &KThreesControlPlaneReconciler{Client: trackingClient, recorder: record.NewFakeRecorder(10)}
	g.Expect(r.triggerInPlaceUpdate(context.Background(), machine, result)).To(HaveOccurred())

	current := &clusterv1.Machine{}
	g.Expect(baseClient.Get(context.Background(), client.ObjectKeyFromObject(machine), current)).To(Succeed())
	g.Expect(current.Annotations).To(HaveKey(clusterv1.UpdateInProgressAnnotation))
	g.Expect(hooks.IsPending(runtimehooksv1.UpdateMachine, current)).To(BeFalse())

	trackingClient.failAt = 0
	trackingClient.patchCalls = 0
	latest := result
	latest.DesiredMachine = result.DesiredMachine.DeepCopy()
	latest.DesiredMachine.Spec.Version = "v1.31.3+k3s1"
	g.Expect(r.triggerInPlaceUpdate(context.Background(), current, latest)).To(Succeed())

	stored := &clusterv1.Machine{}
	g.Expect(baseClient.Get(context.Background(), client.ObjectKeyFromObject(machine), stored)).To(Succeed())
	g.Expect(stored.Spec.Version).To(Equal("v1.31.3+k3s1"))
	g.Expect(hooks.IsPending(runtimehooksv1.UpdateMachine, stored)).To(BeTrue())
}

func TestReconcileControlPlaneOperationsBlocksAnotherRolloutDuringActiveInPlaceUpdate(t *testing.T) {
	g := NewWithT(t)
	utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
	controlPlane, cluster, kcp, machinesNeedingRollout, results, _ := newRolloutControlPlane(t, 2, 2, 0, []int{1}, false)
	g.Expect(machinesNeedingRollout.Names()).To(Equal([]string{"machine-1"}))
	g.Expect(results).To(HaveKey("machine-1"))
	active := controlPlane.Machines["machine-0"]
	active.Annotations[clusterv1.UpdateInProgressAnnotation] = ""
	hooks.MarkObjectAsPending(active, runtimehooksv1.UpdateMachine)

	operations := []string{}
	r := &KThreesControlPlaneReconciler{
		overrides: &reconcilerOverrides{
			scaleUpControlPlane: func(context.Context, *clusterv1.Cluster, *controlplanev1.KThreesControlPlane, *k3s.ControlPlane) (ctrl.Result, error) {
				operations = append(operations, "scale-up")
				return ctrl.Result{}, nil
			},
			scaleDownControlPlane: func(context.Context, *clusterv1.Cluster, *controlplanev1.KThreesControlPlane, *k3s.ControlPlane, collections.Machines) (ctrl.Result, error) {
				operations = append(operations, "scale-down")
				return ctrl.Result{}, nil
			},
			tryInPlaceUpdate: func(context.Context, *k3s.ControlPlane, *clusterv1.Machine, k3s.UpToDateResult) (bool, ctrl.Result, error) {
				operations = append(operations, "in-place")
				return false, ctrl.Result{}, nil
			},
		},
	}

	result, err := r.reconcileControlPlaneOperations(context.Background(), cluster, kcp, controlPlane)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(ctrl.Result{}))
	g.Expect(operations).To(BeEmpty())
}

func TestResumeTriggerUsesLatestDesiredStateWithoutRepeatingCoverage(t *testing.T) {
	g := NewWithT(t)
	initialControlPlane, cluster, kcp, machinesNeedingRollout, results, c := newRolloutControlPlane(t, 1, 1, 0, []int{0}, false)
	g.Expect(initialControlPlane.Machines).To(HaveLen(1))
	g.Expect(machinesNeedingRollout).To(HaveLen(1))
	g.Expect(results["machine-0"].DesiredMachine.Spec.Version).To(Equal("v1.31.2+k3s1"))
	machine := &clusterv1.Machine{}
	g.Expect(c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "machine-0"}, machine)).To(Succeed())
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}
	machine.Annotations[clusterv1.UpdateInProgressAnnotation] = ""
	g.Expect(c.Update(context.Background(), machine)).To(Succeed())

	kcp.Spec.Version = "v1.31.3+k3s1"
	latestControlPlane, err := k3s.NewControlPlane(
		context.Background(), c, cluster, kcp, collections.FromMachines(machine),
	)
	g.Expect(err).NotTo(HaveOccurred())

	canUpdateCalled := false
	triggeredVersion := ""
	r := &KThreesControlPlaneReconciler{
		overrides: &reconcilerOverrides{
			canUpdateMachine: func(context.Context, *clusterv1.Machine, k3s.UpToDateResult) (bool, error) {
				canUpdateCalled = true
				return false, nil
			},
			triggerInPlaceUpdate: func(_ context.Context, _ *clusterv1.Machine, result k3s.UpToDateResult) error {
				triggeredVersion = result.DesiredMachine.Spec.Version
				return nil
			},
		},
	}

	handled, err := r.reconcilePendingInPlaceUpdateTrigger(context.Background(), latestControlPlane)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(handled).To(BeTrue())
	g.Expect(canUpdateCalled).To(BeFalse())
	g.Expect(triggeredVersion).To(Equal("v1.31.3+k3s1"))
}

func triggerFixtures(t *testing.T) (*clusterv1.Machine, k3s.UpToDateResult, client.Client) {
	t.Helper()
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(bootstrapv1.AddToScheme(scheme)).To(Succeed())

	machine := &clusterv1.Machine{
		TypeMeta:   metav1.TypeMeta{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine"},
		ObjectMeta: metav1.ObjectMeta{Name: "machine-1", Namespace: "default", Annotations: map[string]string{}},
		Spec: clusterv1.MachineSpec{
			Version:           "v1.31.1+k3s1",
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{APIGroup: "infrastructure.cluster.x-k8s.io", Kind: "TestMachine", Name: "infra-1"},
			Bootstrap:         clusterv1.Bootstrap{ConfigRef: clusterv1.ContractVersionedObjectReference{APIGroup: bootstrapv1.GroupVersion.Group, Kind: "KThreesConfig", Name: "config-1"}},
		},
	}
	desiredMachine := machine.DeepCopy()
	desiredMachine.Spec.Version = "v1.31.2+k3s1"

	config := &bootstrapv1.KThreesConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: bootstrapv1.GroupVersion.String(), Kind: "KThreesConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: "config-1", Namespace: "default"},
	}
	desiredConfig := config.DeepCopy()
	desiredConfig.Spec.PostK3sCommands = []string{"new"}

	infra := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":       "TestMachine",
		"metadata":   map[string]interface{}{"name": "infra-1", "namespace": "default"},
		"spec":       map[string]interface{}{"size": "small"},
	}}
	desiredInfra := infra.DeepCopy()
	desiredInfra.Object["spec"] = map[string]interface{}{"size": "large"}
	desiredInfra.SetAnnotations(map[string]string{
		clusterv1.TemplateClonedFromNameAnnotation:      "template-2",
		clusterv1.TemplateClonedFromGroupKindAnnotation: "TestMachineTemplate.infrastructure.cluster.x-k8s.io",
	})

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(machine, config, infra).Build()
	return machine, k3s.UpToDateResult{
		DesiredMachine:           desiredMachine,
		CurrentInfraMachine:      infra,
		DesiredInfraMachine:      desiredInfra,
		CurrentKThreesConfig:     config,
		DesiredKThreesConfig:     desiredConfig,
		EligibleForInPlaceUpdate: true,
	}, c
}

type triggerState struct {
	marked         bool
	infraUpdated   bool
	configUpdated  bool
	machineUpdated bool
	hookPending    bool
}

func assertTriggerState(
	t *testing.T,
	c client.Client,
	machine *clusterv1.Machine,
	result k3s.UpToDateResult,
	want triggerState,
) {
	t.Helper()
	g := NewWithT(t)
	ctx := context.Background()

	actualMachine := &clusterv1.Machine{}
	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(machine), actualMachine)).To(Succeed())
	if want.marked {
		g.Expect(actualMachine.Annotations).To(HaveKey(clusterv1.UpdateInProgressAnnotation), "Machine update-in-progress annotation")
	} else {
		g.Expect(actualMachine.Annotations).NotTo(HaveKey(clusterv1.UpdateInProgressAnnotation), "Machine update-in-progress annotation")
	}
	if want.machineUpdated {
		g.Expect(actualMachine.Spec.Version).To(Equal(result.DesiredMachine.Spec.Version))
	} else {
		g.Expect(actualMachine.Spec.Version).To(Equal(machine.Spec.Version))
	}
	g.Expect(hooks.IsPending(runtimehooksv1.UpdateMachine, actualMachine)).To(Equal(want.hookPending))

	actualInfra := &unstructured.Unstructured{}
	actualInfra.SetAPIVersion(result.DesiredInfraMachine.GetAPIVersion())
	actualInfra.SetKind(result.DesiredInfraMachine.GetKind())
	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(result.DesiredInfraMachine), actualInfra)).To(Succeed())
	infraSize, _, err := unstructured.NestedString(actualInfra.Object, "spec", "size")
	g.Expect(err).NotTo(HaveOccurred())
	if want.infraUpdated {
		g.Expect(infraSize).To(Equal("large"))
		g.Expect(actualInfra.GetAnnotations()).To(HaveKey(clusterv1.UpdateInProgressAnnotation))
	} else {
		g.Expect(infraSize).To(Equal("small"))
		g.Expect(actualInfra.GetAnnotations()).NotTo(HaveKey(clusterv1.UpdateInProgressAnnotation))
	}

	actualConfig := &bootstrapv1.KThreesConfig{}
	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(result.DesiredKThreesConfig), actualConfig)).To(Succeed())
	if want.configUpdated {
		g.Expect(actualConfig.Spec.PostK3sCommands).To(Equal([]string{"new"}))
		g.Expect(actualConfig.Annotations).To(HaveKey(clusterv1.UpdateInProgressAnnotation))
	} else {
		g.Expect(actualConfig.Spec.PostK3sCommands).To(BeEmpty())
		g.Expect(actualConfig.Annotations).NotTo(HaveKey(clusterv1.UpdateInProgressAnnotation))
	}
}

type patchTrackingClient struct {
	client.Client
	failAt       int
	patchCalls   int
	failGetAt    int
	getCalls     int
	patchedKinds []string
	operations   []string
}

func (c *patchTrackingClient) Patch(ctx context.Context, object client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patchCalls++
	kind := object.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		kind = fmt.Sprintf("%T", object)
		switch object.(type) {
		case *clusterv1.Machine:
			kind = "Machine"
		case *bootstrapv1.KThreesConfig:
			kind = "KThreesConfig"
		}
	}
	c.patchedKinds = append(c.patchedKinds, kind)
	c.operations = append(c.operations, "patch:"+kind)
	if c.failAt == c.patchCalls {
		return errors.New("injected patch failure")
	}
	return c.Client.Patch(ctx, object, patch, opts...)
}

func (c *patchTrackingClient) Get(ctx context.Context, key client.ObjectKey, object client.Object, opts ...client.GetOption) error {
	c.getCalls++
	kind := object.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		switch object.(type) {
		case *clusterv1.Machine:
			kind = "Machine"
		case *bootstrapv1.KThreesConfig:
			kind = "KThreesConfig"
		}
	}
	c.operations = append(c.operations, "get:"+kind)
	if c.failGetAt == c.getCalls {
		return errors.New("injected cache barrier failure")
	}
	return c.Client.Get(ctx, key, object, opts...)
}

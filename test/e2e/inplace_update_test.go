//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimev1 "sigs.k8s.io/cluster-api/api/runtime/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	dockerinfrav1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
)

const inPlaceCallRecordConfigMap = "k3s-in-place-hook-calls"

type machineIdentity struct {
	Name string    `json:"name"`
	UID  types.UID `json:"uid"`
}

type machineUpdateSnapshot struct {
	Timestamp        metav1.Time        `json:"timestamp"`
	Identity         machineIdentity    `json:"identity"`
	Version          string             `json:"version"`
	UpdateInProgress bool               `json:"updateInProgress"`
	PendingHooks     string             `json:"pendingHooks,omitempty"`
	Conditions       []metav1.Condition `json:"conditions,omitempty"`
}

type inPlaceScenarioEvidence struct {
	Scenario        string                        `json:"scenario"`
	Original        []machineIdentity             `json:"originalMachines"`
	Final           []machineIdentity             `json:"finalMachines,omitempty"`
	Snapshots       []machineUpdateSnapshot       `json:"snapshots,omitempty"`
	Counters        map[string]string             `json:"counters,omitempty"`
	FinalConditions map[string][]metav1.Condition `json:"finalConditions,omitempty"`
}

var _ = Describe("In-place update via Runtime Extension [InPlaceUpdates] [PR-Blocking]", Serial, func() {
	const specName = "in-place-updates"

	var (
		testContext         = context.TODO()
		namespace           *corev1.Namespace
		cancelWatches       context.CancelFunc
		result              *ApplyClusterTemplateAndWaitResult
		clusterName         string
		clusterctlLogFolder string
		evidence            *inPlaceScenarioEvidence
		evidencePath        string
	)

	BeforeEach(func() {
		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))
		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersionUpgradeTo))

		clusterName = fmt.Sprintf("capik3s-in-place-%s", util.RandomString(6))
		namespace, cancelWatches = setupSpecNamespace(testContext, specName, bootstrapClusterProxy, artifactFolder)
		result = new(ApplyClusterTemplateAndWaitResult)
		clusterctlLogFolder = filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		if evidence != nil {
			collectInPlaceCallRecord(testContext, bootstrapClusterProxy.GetClient(), namespace.Name, evidencePath, evidence)
			writeInPlaceEvidence(evidencePath, evidence)
		}
		deleteInPlaceExtensionConfig(testContext, bootstrapClusterProxy.GetClient())

		dumpSpecResourcesAndCleanup(testContext, cleanupInput{
			SpecName:             specName,
			Cluster:              result.Cluster,
			ClusterProxy:         bootstrapClusterProxy,
			ClusterctlConfigPath: clusterctlConfigPath,
			Namespace:            namespace,
			CancelWatches:        cancelWatches,
			IntervalsGetter:      e2eConfig.GetIntervals,
			SkipCleanup:          skipCleanup,
			ArtifactFolder:       artifactFolder,
		})
	})

	It("preserves Machine identity for a supported version update", func() {
		evidence = &inPlaceScenarioEvidence{
			Scenario:        "supported-version-update",
			FinalConditions: map[string][]metav1.Condition{},
		}
		evidencePath = filepath.Join(artifactFolder, "in-place-updates", "scenario-1.json")

		createInPlaceCallRecorderAndExtension(testContext, bootstrapClusterProxy.GetClient(), namespace.Name)

		ApplyClusterTemplateAndWait(testContext, ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                clusterctlLogFolder,
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   "docker",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
				ControlPlaneMachineCount: ptr.To[int64](1),
				WorkerMachineCount:       ptr.To[int64](0),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)

		mgmtClient := bootstrapClusterProxy.GetClient()
		kcpKey := types.NamespacedName{Namespace: result.ControlPlane.Namespace, Name: result.ControlPlane.Name}
		patchKThreesControlPlane(testContext, mgmtClient, kcpKey, func(kcp *controlplanev1.KThreesControlPlane) {
			setZeroSurge(kcp)
		})

		original := getSingleControlPlaneMachine(testContext, mgmtClient, result.Cluster)
		evidence.Original = []machineIdentity{identityForMachine(original)}

		targetVersion := e2eConfig.MustGetVariable(KubernetesVersionUpgradeTo)
		patchKThreesControlPlane(testContext, mgmtClient, kcpKey, func(kcp *controlplanev1.KThreesControlPlane) {
			kcp.Spec.Version = targetVersion
		})

		finalMachine := waitForSupportedInPlaceUpdate(
			testContext,
			mgmtClient,
			result.Cluster,
			original,
			targetVersion,
			evidence,
			e2eConfig.GetIntervals(specName, "wait-control-plane"),
		)

		Expect(identityForMachine(finalMachine)).To(Equal(identityForMachine(original)))
		assertNoInPlaceAnnotations(testContext, mgmtClient, finalMachine)
		Expect(machineCondition(finalMachine, clusterv1.MachineUpToDateCondition).Status).To(Equal(metav1.ConditionTrue))

		collectInPlaceCallRecord(testContext, mgmtClient, namespace.Name, evidencePath, evidence)
		Expect(counterValue(evidence.Counters, string(original.UID)+".canUpdateMachine")).To(BeNumerically(">=", 1))
		Expect(counterValue(evidence.Counters, string(original.UID)+".updateMachine")).To(BeNumerically(">=", 2))

		evidence.Final = []machineIdentity{identityForMachine(finalMachine)}
		evidence.FinalConditions[finalMachine.Name] = finalMachine.Status.Conditions
		writeInPlaceEvidence(evidencePath, evidence)
	})

	It("replaces every Machine when an infrastructure diff is unsupported", func() {
		evidence = &inPlaceScenarioEvidence{
			Scenario:        "unsupported-infrastructure-diff",
			FinalConditions: map[string][]metav1.Condition{},
		}
		evidencePath = filepath.Join(artifactFolder, "in-place-updates", "scenario-2.json")

		createInPlaceCallRecorderAndExtension(testContext, bootstrapClusterProxy.GetClient(), namespace.Name)

		ApplyClusterTemplateAndWait(testContext, ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                clusterctlLogFolder,
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   "docker",
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
				ControlPlaneMachineCount: ptr.To[int64](3),
				WorkerMachineCount:       ptr.To[int64](0),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)

		mgmtClient := bootstrapClusterProxy.GetClient()
		kcpKey := types.NamespacedName{Namespace: result.ControlPlane.Namespace, Name: result.ControlPlane.Name}

		originalMachines := getControlPlaneMachines(testContext, mgmtClient, result.Cluster)
		Expect(originalMachines).To(HaveLen(3))
		evidence.Original = identitiesForMachines(originalMachines)

		templateB := createRotatedDockerMachineTemplate(testContext, mgmtClient, namespace.Name, clusterName)
		targetVersion := e2eConfig.MustGetVariable(KubernetesVersionUpgradeTo)
		patchKThreesControlPlane(testContext, mgmtClient, kcpKey, func(kcp *controlplanev1.KThreesControlPlane) {
			kcp.Spec.Version = targetVersion
			kcp.Spec.MachineTemplate.InfrastructureRef = corev1.ObjectReference{
				APIVersion: dockerinfrav1.GroupVersion.String(),
				Kind:       "DockerMachineTemplate",
				Namespace:  namespace.Name,
				Name:       templateB.Name,
			}
			setZeroSurge(kcp)
		})

		finalMachines := waitForUnsupportedDiffReplacement(
			testContext,
			mgmtClient,
			result.Cluster,
			originalMachines,
			templateB.Name,
			targetVersion,
		)

		collectInPlaceCallRecord(testContext, mgmtClient, namespace.Name, evidencePath, evidence)
		for i := range originalMachines {
			uid := string(originalMachines[i].UID)
			Expect(counterValue(evidence.Counters, uid+".canUpdateMachine")).To(BeNumerically(">=", 1))
			Expect(counterValueOrZero(evidence.Counters, uid+".updateMachine")).To(BeZero())
		}

		for i := range finalMachines {
			assertNoInPlaceAnnotations(testContext, mgmtClient, &finalMachines[i])
			evidence.FinalConditions[finalMachines[i].Name] = finalMachines[i].Status.Conditions
		}
		evidence.Final = identitiesForMachines(finalMachines)
		writeInPlaceEvidence(evidencePath, evidence)
	})
})

func inPlaceExtensionConfig(namespace string) *runtimev1.ExtensionConfig {
	return &runtimev1.ExtensionConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "k3s-test-extension",
			Annotations: map[string]string{
				runtimev1.InjectCAFromSecretAnnotation: "k3s-test-extension-system/k3s-test-extension-webhook-service-cert",
			},
		},
		Spec: runtimev1.ExtensionConfigSpec{
			ClientConfig: runtimev1.ClientConfig{
				Service: runtimev1.ServiceReference{
					Namespace: "k3s-test-extension-system",
					Name:      "k3s-test-extension-webhook-service",
				},
			},
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"kubernetes.io/metadata.name": namespace,
				},
			},
			Settings: map[string]string{
				"callRecordNamespace": namespace,
				"callRecordConfigMap": inPlaceCallRecordConfigMap,
			},
		},
	}
}

func createInPlaceCallRecorderAndExtension(ctx context.Context, c client.Client, namespace string) {
	Expect(c.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      inPlaceCallRecordConfigMap,
		},
	})).To(Succeed())
	Expect(c.Create(ctx, inPlaceExtensionConfig(namespace))).To(Succeed())

	Eventually(func() (bool, error) {
		extensionConfig := &runtimev1.ExtensionConfig{}
		if err := c.Get(ctx, client.ObjectKey{Name: "k3s-test-extension"}, extensionConfig); err != nil {
			return false, err
		}
		condition := meta.FindStatusCondition(extensionConfig.Status.Conditions, runtimev1.ExtensionConfigDiscoveredCondition)
		return condition != nil && condition.Status == metav1.ConditionTrue, nil
	}, 3*time.Minute, time.Second).Should(BeTrue(), "Runtime Extension was not discovered")
}

func deleteInPlaceExtensionConfig(ctx context.Context, c client.Client) {
	extensionConfig := &runtimev1.ExtensionConfig{ObjectMeta: metav1.ObjectMeta{Name: "k3s-test-extension"}}
	if err := c.Delete(ctx, extensionConfig); err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func patchKThreesControlPlane(
	ctx context.Context,
	c client.Client,
	key types.NamespacedName,
	mutate func(*controlplanev1.KThreesControlPlane),
) {
	Eventually(func() error {
		kcp := &controlplanev1.KThreesControlPlane{}
		if err := c.Get(ctx, key, kcp); err != nil {
			return err
		}
		original := kcp.DeepCopy()
		mutate(kcp)
		return c.Patch(ctx, kcp, client.MergeFrom(original))
	}, time.Minute, time.Second).Should(Succeed())
}

func setZeroSurge(kcp *controlplanev1.KThreesControlPlane) {
	kcp.Spec.RolloutStrategy = &controlplanev1.RolloutStrategy{
		Type: controlplanev1.RollingUpdateStrategyType,
		RollingUpdate: &controlplanev1.RollingUpdate{
			MaxSurge: ptr.To(intstr.FromInt32(0)),
		},
	}
}

func getControlPlaneMachines(ctx context.Context, c client.Client, cluster *clusterv1.Cluster) []clusterv1.Machine {
	machines := &clusterv1.MachineList{}
	Expect(c.List(ctx, machines,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel:         cluster.Name,
			clusterv1.MachineControlPlaneLabel: "",
		},
	)).To(Succeed())
	return machines.Items
}

func getSingleControlPlaneMachine(ctx context.Context, c client.Client, cluster *clusterv1.Cluster) *clusterv1.Machine {
	machines := getControlPlaneMachines(ctx, c, cluster)
	Expect(machines).To(HaveLen(1))
	return machines[0].DeepCopy()
}

func waitForSupportedInPlaceUpdate(
	ctx context.Context,
	c client.Client,
	cluster *clusterv1.Cluster,
	original *clusterv1.Machine,
	targetVersion string,
	evidence *inPlaceScenarioEvidence,
	_ []interface{},
) *clusterv1.Machine {
	var finalMachine *clusterv1.Machine
	sawUpdateInProgress := false
	sawPendingHook := false

	Eventually(func() (bool, error) {
		machine := &clusterv1.Machine{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: original.Name}, machine); err != nil {
			return false, err
		}

		_, updateInProgress := machine.Annotations[clusterv1.UpdateInProgressAnnotation]
		pendingHooks := machine.Annotations[runtimev1.PendingHooksAnnotation]
		if updateInProgress || pendingHooks != "" {
			evidence.Snapshots = append(evidence.Snapshots, machineUpdateSnapshot{
				Timestamp:        metav1.Now(),
				Identity:         identityForMachine(machine),
				Version:          machine.Spec.Version,
				UpdateInProgress: updateInProgress,
				PendingHooks:     pendingHooks,
				Conditions:       append([]metav1.Condition(nil), machine.Status.Conditions...),
			})
		}
		sawUpdateInProgress = sawUpdateInProgress || updateInProgress
		sawPendingHook = sawPendingHook || strings.Contains(pendingHooks, "UpdateMachine")

		condition := meta.FindStatusCondition(machine.Status.Conditions, clusterv1.MachineUpToDateCondition)
		if condition == nil ||
			condition.Status != metav1.ConditionTrue ||
			machine.Spec.Version != targetVersion ||
			updateInProgress ||
			pendingHooks != "" {
			return false, nil
		}
		finalMachine = machine.DeepCopy()
		return true, nil
	}, 10*time.Minute, 500*time.Millisecond).Should(BeTrue())

	Expect(sawUpdateInProgress).To(BeTrue(), "did not observe the update-in-progress annotation")
	Expect(sawPendingHook).To(BeTrue(), "did not observe the UpdateMachine pending hook")
	return finalMachine
}

func identityForMachine(machine *clusterv1.Machine) machineIdentity {
	return machineIdentity{Name: machine.Name, UID: machine.UID}
}

func identitiesForMachines(machines []clusterv1.Machine) []machineIdentity {
	identities := make([]machineIdentity, 0, len(machines))
	for i := range machines {
		identities = append(identities, identityForMachine(&machines[i]))
	}
	return identities
}

func createRotatedDockerMachineTemplate(
	ctx context.Context,
	c client.Client,
	namespace string,
	clusterName string,
) *dockerinfrav1.DockerMachineTemplate {
	template := &dockerinfrav1.DockerMachineTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: dockerinfrav1.GroupVersion.String(),
			Kind:       "DockerMachineTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName + "-control-plane-rotated",
		},
		Spec: dockerinfrav1.DockerMachineTemplateSpec{
			Template: dockerinfrav1.DockerMachineTemplateResource{
				Spec: dockerinfrav1.DockerMachineSpec{
					CustomImage: "kindest/node:v1.33.0",
				},
			},
		},
	}
	Expect(c.Create(ctx, template)).To(Succeed())
	return template
}

func waitForUnsupportedDiffReplacement(
	ctx context.Context,
	c client.Client,
	cluster *clusterv1.Cluster,
	originalMachines []clusterv1.Machine,
	templateName string,
	targetVersion string,
) []clusterv1.Machine {
	originalUIDs := map[types.UID]struct{}{}
	originalNames := map[string]struct{}{}
	for i := range originalMachines {
		originalUIDs[originalMachines[i].UID] = struct{}{}
		originalNames[originalMachines[i].Name] = struct{}{}
	}

	var finalMachines []clusterv1.Machine
	Eventually(func() (bool, error) {
		machines := &clusterv1.MachineList{}
		if err := c.List(ctx, machines,
			client.InNamespace(cluster.Namespace),
			client.MatchingLabels{
				clusterv1.ClusterNameLabel:         cluster.Name,
				clusterv1.MachineControlPlaneLabel: "",
			},
		); err != nil {
			return false, err
		}
		if len(machines.Items) != 3 {
			return false, nil
		}

		for i := range machines.Items {
			machine := &machines.Items[i]
			if _, found := originalUIDs[machine.UID]; found {
				return false, nil
			}
			if _, found := originalNames[machine.Name]; found {
				return false, nil
			}
			if machine.Spec.Version != targetVersion {
				return false, nil
			}
			condition := meta.FindStatusCondition(machine.Status.Conditions, clusterv1.MachineUpToDateCondition)
			if condition == nil || condition.Status != metav1.ConditionTrue {
				return false, nil
			}
			readyCondition := meta.FindStatusCondition(machine.Status.Conditions, clusterv1.MachineReadyCondition)
			if readyCondition == nil || readyCondition.Status != metav1.ConditionTrue {
				return false, nil
			}
			if _, found := machine.Annotations[clusterv1.UpdateInProgressAnnotation]; found {
				return false, nil
			}
			if machine.Annotations[runtimev1.PendingHooksAnnotation] != "" {
				return false, nil
			}

			infraMachine := &dockerinfrav1.DockerMachine{}
			if err := c.Get(ctx, client.ObjectKey{
				Namespace: machine.Namespace,
				Name:      machine.Spec.InfrastructureRef.Name,
			}, infraMachine); err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			if infraMachine.Annotations[clusterv1.TemplateClonedFromNameAnnotation] != templateName {
				return false, nil
			}
		}

		finalMachines = append([]clusterv1.Machine(nil), machines.Items...)
		return true, nil
	}, 15*time.Minute, time.Second).Should(BeTrue())

	return finalMachines
}

func machineCondition(machine *clusterv1.Machine, conditionType string) *metav1.Condition {
	condition := meta.FindStatusCondition(machine.Status.Conditions, conditionType)
	Expect(condition).NotTo(BeNil(), "Machine %s is missing condition %s", machine.Name, conditionType)
	return condition
}

func assertNoInPlaceAnnotations(ctx context.Context, c client.Client, machine *clusterv1.Machine) {
	assertAnnotationsCleared := func(name string, annotations map[string]string) {
		Expect(annotations).NotTo(HaveKey(clusterv1.UpdateInProgressAnnotation), "%s retained the update-in-progress annotation", name)
		Expect(annotations).NotTo(HaveKey(runtimev1.PendingHooksAnnotation), "%s retained pending hooks", name)
	}

	assertAnnotationsCleared("Machine "+machine.Name, machine.Annotations)

	infraMachine := &dockerinfrav1.DockerMachine{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: machine.Namespace, Name: machine.Spec.InfrastructureRef.Name}, infraMachine)).To(Succeed())
	assertAnnotationsCleared("DockerMachine "+infraMachine.Name, infraMachine.Annotations)

	if machine.Spec.Bootstrap.ConfigRef.IsDefined() {
		bootstrapConfig := &bootstrapv1.KThreesConfig{}
		Expect(c.Get(ctx, client.ObjectKey{Namespace: machine.Namespace, Name: machine.Spec.Bootstrap.ConfigRef.Name}, bootstrapConfig)).To(Succeed())
		assertAnnotationsCleared("KThreesConfig "+bootstrapConfig.Name, bootstrapConfig.Annotations)
	}
}

func collectInPlaceCallRecord(
	ctx context.Context,
	c client.Client,
	namespace string,
	evidencePath string,
	evidence *inPlaceScenarioEvidence,
) {
	configMap := &corev1.ConfigMap{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: inPlaceCallRecordConfigMap}, configMap)
	if apierrors.IsNotFound(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())

	evidence.Counters = map[string]string{}
	for key, value := range configMap.Data {
		evidence.Counters[key] = value
	}

	configMapPath := filepath.Join(
		filepath.Dir(evidencePath),
		strings.TrimSuffix(filepath.Base(evidencePath), filepath.Ext(evidencePath))+"-call-record-configmap.json",
	)
	writeJSONArtifact(configMapPath, configMap)
}

func writeInPlaceEvidence(path string, evidence *inPlaceScenarioEvidence) {
	writeJSONArtifact(path, evidence)
}

func writeJSONArtifact(path string, value any) {
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	data, err := json.MarshalIndent(value, "", "  ")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(path, append(data, '\n'), 0o600)).To(Succeed())
}

func counterValue(counters map[string]string, key string) int {
	value, ok := counters[key]
	Expect(ok).To(BeTrue(), "counter %s was not recorded", key)
	count, err := strconv.Atoi(value)
	Expect(err).NotTo(HaveOccurred())
	return count
}

func counterValueOrZero(counters map[string]string, key string) int {
	if _, ok := counters[key]; !ok {
		return 0
	}
	return counterValue(counters, key)
}

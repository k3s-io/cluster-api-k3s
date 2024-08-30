//go:build e2e
// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/exp/rand"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/labels"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test follows CAPI's clusterclass_rollout test to test the inplace mutable fields
// setting on ControlPlane object could be rollout to underlying machines.
// The original test does not apply to k3s cluster as it modified controlPlane fields specific to KubeadmControlPlane.
// Link to CAPI clusterclass_rollout test: https://github.com/kubernetes-sigs/cluster-api/blob/main/test/e2e/clusterclass_rollout.go
var _ = Describe("Inplace mutable fields rollout test [ClusterClass]", func() {
	var (
		ctx                    = context.TODO()
		specName               = "inplace-rollout"
		namespace              *corev1.Namespace
		cancelWatches          context.CancelFunc
		result                 *ApplyClusterTemplateAndWaitResult
		clusterName            string
		clusterctlLogFolder    string
		infrastructureProvider string
	)

	BeforeEach(func() {
		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		clusterName = fmt.Sprintf("capik3s-inplace-%s", util.RandomString(6))
		infrastructureProvider = "docker"

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, bootstrapClusterProxy, artifactFolder)

		result = new(ApplyClusterTemplateAndWaitResult)

		clusterctlLogFolder = filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		cleanInput := cleanupInput{
			SpecName:        specName,
			Cluster:         result.Cluster,
			ClusterProxy:    bootstrapClusterProxy,
			Namespace:       namespace,
			CancelWatches:   cancelWatches,
			IntervalsGetter: e2eConfig.GetIntervals,
			SkipCleanup:     skipCleanup,
			ArtifactFolder:  artifactFolder,
		}

		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	Context("Modifying inplace mutable fields", func() {
		It("Should apply new value without triggering rollout", func() {
			By("Creating a workload cluster with topology")
			ApplyClusterTemplateAndWait(ctx, ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   infrastructureProvider,
					Flavor:                   "topology",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(3),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)

			By("Rolling out changes to control plane (in-place)")
			machinesBeforeUpgrade := getMachinesByCluster(ctx, bootstrapClusterProxy.GetClient(), result.Cluster)
			By("Modifying the control plane configuration via Cluster topology and wait for changes to be applied to the control plane object (in-place)")
			modifyControlPlaneViaClusterAndWait(ctx, modifyControlPlaneViaClusterAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				Cluster:      result.Cluster,
				ModifyControlPlaneTopology: func(topology *clusterv1.ControlPlaneTopology) {
					// Drop existing labels and annotations and set new ones.
					topology.Metadata.Labels = map[string]string{
						"Cluster.topology.controlPlane.newLabel": "Cluster.topology.controlPlane.newLabelValue",
					}
					topology.Metadata.Annotations = map[string]string{
						"Cluster.topology.controlPlane.newAnnotation": "Cluster.topology.controlPlane.newAnnotationValue",
					}
					topology.NodeDrainTimeout = &metav1.Duration{Duration: time.Duration(rand.Intn(20)) * time.Second}        //nolint:gosec
					topology.NodeDeletionTimeout = &metav1.Duration{Duration: time.Duration(rand.Intn(20)) * time.Second}     //nolint:gosec
					topology.NodeVolumeDetachTimeout = &metav1.Duration{Duration: time.Duration(rand.Intn(20)) * time.Second} //nolint:gosec
				},
				WaitForControlPlane: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			})

			By("Verifying there are no unexpected rollouts through in-place rollout")
			Consistently(func(g Gomega) {
				machinesAfterUpgrade := getMachinesByCluster(ctx, bootstrapClusterProxy.GetClient(), result.Cluster)
				g.Expect(machinesAfterUpgrade.Equal(machinesBeforeUpgrade)).To(BeTrue(), "Machines must not be replaced through in-place rollout")
			}, 30*time.Second, 1*time.Second).Should(Succeed())
		})
	})
})

// modifyControlPlaneViaClusterAndWaitInput is the input type for modifyControlPlaneViaClusterAndWait.
type modifyControlPlaneViaClusterAndWaitInput struct {
	ClusterProxy               framework.ClusterProxy
	Cluster                    *clusterv1.Cluster
	ModifyControlPlaneTopology func(topology *clusterv1.ControlPlaneTopology)
	WaitForControlPlane        []interface{}
}

// modifyControlPlaneViaClusterAndWait modifies the ControlPlaneTopology of a Cluster topology via ModifyControlPlaneTopology.
// It then waits until the changes are rolled out to the ControlPlane and ControlPlane Machine of the Cluster.
func modifyControlPlaneViaClusterAndWait(ctx context.Context, input modifyControlPlaneViaClusterAndWaitInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for modifyControlPlaneViaClusterAndWait")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling modifyControlPlaneViaClusterAndWait")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling modifyControlPlaneViaClusterAndWait")

	mgmtClient := input.ClusterProxy.GetClient()

	Byf("Modifying the control plane topology of Cluster %s", klog.KObj(input.Cluster))

	// Patch the control plane topology in the Cluster.
	patchHelper, err := patch.NewHelper(input.Cluster, mgmtClient)
	Expect(err).ToNot(HaveOccurred())
	input.ModifyControlPlaneTopology(&input.Cluster.Spec.Topology.ControlPlane)
	Expect(patchHelper.Patch(ctx, input.Cluster)).To(Succeed())

	// NOTE: We wait until the change is rolled out to the control plane object and the control plane machines.
	Byf("Waiting for control plane rollout to complete.")
	Eventually(func(g Gomega) {
		// Get the ControlPlane.
		controlPlaneRef := input.Cluster.Spec.ControlPlaneRef
		controlPlaneTopology := input.Cluster.Spec.Topology.ControlPlane

		// Get KThreesControlPlane object.
		cpObj := &controlplanev1.KThreesControlPlane{}
		kcpObjKey := client.ObjectKey{
			Namespace: input.Cluster.Namespace,
			Name:      controlPlaneRef.Name,
		}
		err = mgmtClient.Get(ctx, kcpObjKey, cpObj)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify that the fields from Cluster topology are set on the control plane.
		assertControlPlaneTopologyFields(g, cpObj, controlPlaneTopology)

		// Verify that the control plane machines have the required fields.
		cluster := input.Cluster
		controlPlaneMachineList := &clusterv1.MachineList{}
		g.Expect(mgmtClient.List(ctx, controlPlaneMachineList, client.InNamespace(cluster.Namespace), client.MatchingLabels{
			clusterv1.MachineControlPlaneLabel: "",
			clusterv1.ClusterNameLabel:         cluster.Name,
		})).To(Succeed())
		for _, m := range controlPlaneMachineList.Items {
			metadata := m.ObjectMeta
			for k, v := range controlPlaneTopology.Metadata.Labels {
				g.Expect(metadata.Labels).To(HaveKeyWithValue(k, v))
			}
			for k, v := range controlPlaneTopology.Metadata.Annotations {
				g.Expect(metadata.Annotations).To(HaveKeyWithValue(k, v))
			}

			if controlPlaneTopology.NodeDrainTimeout != nil {
				nodeDrainTimeout := m.Spec.NodeDrainTimeout
				g.Expect(nodeDrainTimeout).To(Equal(controlPlaneTopology.NodeDrainTimeout))
			}

			if controlPlaneTopology.NodeDeletionTimeout != nil {
				nodeDeletionTimeout := m.Spec.NodeDeletionTimeout
				g.Expect(nodeDeletionTimeout).To(Equal(controlPlaneTopology.NodeDeletionTimeout))
			}

			if controlPlaneTopology.NodeVolumeDetachTimeout != nil {
				nodeVolumeDetachTimeout := m.Spec.NodeVolumeDetachTimeout
				g.Expect(nodeVolumeDetachTimeout).To(Equal(controlPlaneTopology.NodeVolumeDetachTimeout))
			}
		}
	}, input.WaitForControlPlane...).Should(Succeed())
}

// assertControlPlaneTopologyFields asserts that all fields set in the ControlPlaneTopology have been set on the ControlPlane.
// Note: We intentionally focus on the fields set in the ControlPlaneTopology and ignore the ones set through ClusterClass or
// ControlPlane template as we want to validate that the fields of the ControlPlaneTopology have been propagated correctly.
func assertControlPlaneTopologyFields(g Gomega, controlPlane *controlplanev1.KThreesControlPlane, controlPlaneTopology clusterv1.ControlPlaneTopology) {
	metadata := controlPlane.ObjectMeta
	for k, v := range controlPlaneTopology.Metadata.Labels {
		g.Expect(metadata.Labels).To(HaveKeyWithValue(k, v))
	}
	for k, v := range controlPlaneTopology.Metadata.Annotations {
		g.Expect(metadata.Annotations).To(HaveKeyWithValue(k, v))
	}

	if controlPlaneTopology.NodeDrainTimeout != nil {
		nodeDrainTimeout := controlPlane.Spec.MachineTemplate.NodeDrainTimeout
		g.Expect(nodeDrainTimeout).To(Equal(controlPlaneTopology.NodeDrainTimeout))
	}

	if controlPlaneTopology.NodeDeletionTimeout != nil {
		nodeDeletionTimeout := controlPlane.Spec.MachineTemplate.NodeDeletionTimeout
		g.Expect(nodeDeletionTimeout).To(Equal(controlPlaneTopology.NodeDeletionTimeout))
	}

	if controlPlaneTopology.NodeVolumeDetachTimeout != nil {
		nodeVolumeDetachTimeout := controlPlane.Spec.MachineTemplate.NodeVolumeDetachTimeout
		g.Expect(nodeVolumeDetachTimeout).To(Equal(controlPlaneTopology.NodeVolumeDetachTimeout))
	}
}

// getMachinesByCluster gets the Machines of a Cluster and returns them as a Set of Machine names.
// Note: This excludes MachinePool Machines as they are not replaced by rollout yet.
func getMachinesByCluster(ctx context.Context, client client.Client, cluster *clusterv1.Cluster) sets.Set[string] {
	machines := sets.Set[string]{}
	machinesByCluster := framework.GetMachinesByCluster(ctx, framework.GetMachinesByClusterInput{
		Lister:      client,
		ClusterName: cluster.Name,
		Namespace:   cluster.Namespace,
	})
	for i := range machinesByCluster {
		m := machinesByCluster[i]
		if !labels.IsMachinePoolOwned(&m) {
			machines.Insert(m.Name)
		}
	}
	return machines
}

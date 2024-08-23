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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	e2eOldLabelName       = "Cluster.topology.controlPlane.oldLabel"
	e2eOldAnnotationName  = "Cluster.topology.controlPlane.oldAnnotation"
	e2eNewAnnotationValue = "newAnnotationValue"
	kcpManagerName        = "capi-kthreescontrolplane"
)

var (
	clusterctlDownloadURL = "https://github.com/kubernetes-sigs/cluster-api/releases/download/v%s/clusterctl-{OS}-{ARCH}"
	providerCAPIPrefix    = "cluster-api:v%s"
	providerKThreesPrefix = "k3s:v%s"
	providerDockerPrefix  = "docker:v%s"
)

var _ = Describe("When testing clusterctl upgrades using ClusterClass (v0.2.0=>current) [ClusterClass]", func() {
	// Upgrade from v0.2.0 to current (current version is built from src).
	var (
		specName                = "clusterctl-upgrade"
		version                 = "0.2.0"
		k3sCapiUpgradedVersion  string
		capiCoreVersion         string
		capiCoreUpgradedVersion string
	)
	BeforeEach(func() {
		Expect(e2eConfig.Variables).To(HaveKey(K3sCapiCurrentVersion))
		Expect(e2eConfig.Variables).To(HaveKey(CapiCoreVersion))

		// Will upgrade k3s CAPI from v0.2.0 to k3sCapiUpgradedVersion.
		k3sCapiUpgradedVersion = e2eConfig.GetVariable(K3sCapiCurrentVersion)

		// Will init other CAPI core/CAPD componenets with CapiCoreVersion, and then upgrade to CapiCoreUpgradedVersion.
		// For now, this two versions are equal.
		capiCoreVersion = e2eConfig.GetVariable(CapiCoreVersion)
		capiCoreUpgradedVersion = capiCoreVersion
	})

	capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
		return capi_e2e.ClusterctlUpgradeSpecInput{
			E2EConfig:                       e2eConfig,
			ClusterctlConfigPath:            clusterctlConfigPath,
			BootstrapClusterProxy:           bootstrapClusterProxy,
			ArtifactFolder:                  artifactFolder,
			SkipCleanup:                     skipCleanup,
			InfrastructureProvider:          ptr.To("docker"),
			InitWithBinary:                  fmt.Sprintf(clusterctlDownloadURL, capiCoreVersion),
			InitWithCoreProvider:            fmt.Sprintf(providerCAPIPrefix, capiCoreVersion),
			InitWithBootstrapProviders:      []string{fmt.Sprintf(providerKThreesPrefix, version)},
			InitWithControlPlaneProviders:   []string{fmt.Sprintf(providerKThreesPrefix, version)},
			InitWithInfrastructureProviders: []string{fmt.Sprintf(providerDockerPrefix, capiCoreVersion)},
			InitWithProvidersContract:       "v1beta1",
			// InitWithKubernetesVersion is for the management cluster, WorkloadKubernetesVersion is for the workload cluster.
			// Hardcoding the versions as later versions of k3s might not be compatible with the older versions of CAPI k3s.
			InitWithKubernetesVersion:   "v1.30.0",
			WorkloadKubernetesVersion:   "v1.30.2+k3s2",
			MgmtFlavor:                  "topology",
			WorkloadFlavor:              "topology",
			UseKindForManagementCluster: true,
			// Configuration for the provider upgrades.
			Upgrades: []capi_e2e.ClusterctlUpgradeSpecInputUpgrade{
				{
					// CAPI core or CAPD with compatible version.
					CoreProvider:            fmt.Sprintf(providerCAPIPrefix, capiCoreUpgradedVersion),
					InfrastructureProviders: []string{fmt.Sprintf(providerDockerPrefix, capiCoreUpgradedVersion)},
					// Upgrade to current k3s.
					BootstrapProviders:    []string{fmt.Sprintf(providerKThreesPrefix, k3sCapiUpgradedVersion)},
					ControlPlaneProviders: []string{fmt.Sprintf(providerKThreesPrefix, k3sCapiUpgradedVersion)},
				},
			},
			// After the k3s CAPI upgrade, will test the inplace mutable fields
			// could be updated correctly. This is in complement to the
			// inplace_rollout_test to include the k3s CAPI upgrade scenario.
			// We are testing upgrading from v0.2.0 as we do not support SSA
			// before v0.2.0.
			PostUpgrade: func(managementClusterProxy framework.ClusterProxy, clusterNamespace, clusterName string) {
				clusterList := &clusterv1.ClusterList{}
				mgmtClient := managementClusterProxy.GetClient()

				if err := mgmtClient.List(ctx, clusterList, client.InNamespace(clusterNamespace)); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(clusterList.Items).To(HaveLen(1), fmt.Sprintf("Expected to have only one cluster in the namespace %s", clusterNamespace))

				cluster := &clusterList.Items[0]

				Byf("Waiting the new controller to reconcile at least once, to set the managed fields with k3s kcpManagerName for all control plane machines.")
				Eventually(func(g Gomega) {
					controlPlaneMachineList := &clusterv1.MachineList{}
					g.Expect(mgmtClient.List(ctx, controlPlaneMachineList, client.InNamespace(clusterNamespace), client.MatchingLabels{
						clusterv1.MachineControlPlaneLabel: "",
						clusterv1.ClusterNameLabel:         cluster.Name,
					})).To(Succeed())
					for _, m := range controlPlaneMachineList.Items {
						g.Expect(m.ObjectMeta.ManagedFields).To(ContainElement(MatchFields(IgnoreExtras, Fields{
							"Manager": Equal(kcpManagerName),
						})))
					}
				}, e2eConfig.GetIntervals(specName, "wait-control-plane")...).Should(Succeed())

				Byf("Modifying the control plane label and annotations of Cluster %s", cluster.Name)
				topologyControlPlane := cluster.Spec.Topology.ControlPlane
				Expect(topologyControlPlane.Metadata.Labels).To(HaveKey(e2eOldLabelName))
				Expect(topologyControlPlane.Metadata.Annotations).To(HaveKey(e2eOldAnnotationName))

				patchHelper, err := patch.NewHelper(cluster, mgmtClient)
				Expect(err).ToNot(HaveOccurred())

				// Remove old label, and set an old annotation with new value.
				delete(topologyControlPlane.Metadata.Labels, e2eOldLabelName)
				topologyControlPlane.Metadata.Annotations[e2eOldAnnotationName] = e2eNewAnnotationValue

				Expect(patchHelper.Patch(ctx, cluster)).To(Succeed())

				Byf("Waiting for labels and annotations of all controlplane machines to be updated.")
				Eventually(func(g Gomega) {
					controlPlaneMachineList := &clusterv1.MachineList{}
					g.Expect(mgmtClient.List(ctx, controlPlaneMachineList, client.InNamespace(clusterNamespace), client.MatchingLabels{
						clusterv1.MachineControlPlaneLabel: "",
						clusterv1.ClusterNameLabel:         cluster.Name,
					})).To(Succeed())
					for _, m := range controlPlaneMachineList.Items {
						g.Expect(m.ObjectMeta.Labels).NotTo(HaveKey(e2eOldLabelName))
						g.Expect(m.ObjectMeta.Annotations).To(HaveKeyWithValue(e2eOldAnnotationName, e2eNewAnnotationValue))
					}
				}, e2eConfig.GetIntervals(specName, "wait-control-plane")...).Should(Succeed())
			},
		}
	})
})

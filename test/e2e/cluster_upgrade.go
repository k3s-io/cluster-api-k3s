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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
)

// ClusterUpgradeSpecInput is the input for ClusterUpgradeConformanceSpec.
type ClusterUpgradeSpecInput struct {
	E2EConfig             *clusterctl.E2EConfig
	ClusterctlConfigPath  string
	BootstrapClusterProxy framework.ClusterProxy
	ArtifactFolder        string
	SkipCleanup           bool
	ControlPlaneWaiters   ControlPlaneWaiters

	// InfrastructureProviders specifies the infrastructure to use for clusterctl
	// operations (Example: get cluster templates).
	// Note: In most cases this need not be specified. It only needs to be specified when
	// multiple infrastructure providers (ex: CAPD + in-memory) are installed on the cluster as clusterctl will not be
	// able to identify the default.
	InfrastructureProvider *string

	// ControlPlaneMachineCount is used in `config cluster` to configure the count of the control plane machines used in the test.
	// Default is 1.
	ControlPlaneMachineCount *int64

	// WorkerMachineCount is used in `config cluster` to configure the count of the worker machines used in the test.
	// NOTE: If the WORKER_MACHINE_COUNT var is used multiple times in the cluster template, the absolute count of
	// worker machines is a multiple of WorkerMachineCount.
	// Default is 2.
	WorkerMachineCount *int64

	// Flavor to use when creating the cluster for testing, "upgrades" is used if not specified.
	Flavor *string
}

func ClusterUpgradeSpec(ctx context.Context, inputGetter func() ClusterUpgradeSpecInput) {
	const (
		specName = "workload-cluster-upgrade"
	)

	var (
		input         ClusterUpgradeSpecInput
		namespace     *corev1.Namespace
		cancelWatches context.CancelFunc

		controlPlaneMachineCount   int64
		workerMachineCount         int64
		kubernetesVersionUpgradeTo string

		result              *ApplyClusterTemplateAndWaitResult
		clusterName         string
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(input.E2EConfig).ToNot(BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		Expect(input.ClusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. input.ClusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(input.ArtifactFolder, 0750)).To(Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)

		Expect(input.E2EConfig.Variables).To(HaveKey(KubernetesVersion))
		Expect(input.E2EConfig.Variables).To(HaveKey(KubernetesVersionUpgradeTo))

		clusterName = fmt.Sprintf("capik3s-cluster-upgrade-%s", util.RandomString(6))

		if input.ControlPlaneMachineCount == nil {
			controlPlaneMachineCount = 1
		} else {
			controlPlaneMachineCount = *input.ControlPlaneMachineCount
		}

		if input.WorkerMachineCount == nil {
			workerMachineCount = 2
		} else {
			workerMachineCount = *input.WorkerMachineCount
		}

		kubernetesVersionUpgradeTo = input.E2EConfig.GetVariable(KubernetesVersionUpgradeTo)

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)

		result = new(ApplyClusterTemplateAndWaitResult)

		clusterctlLogFolder = filepath.Join(input.ArtifactFolder, "clusters", input.BootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		cleanInput := cleanupInput{
			SpecName:        specName,
			Cluster:         result.Cluster,
			ClusterProxy:    input.BootstrapClusterProxy,
			Namespace:       namespace,
			CancelWatches:   cancelWatches,
			IntervalsGetter: input.E2EConfig.GetIntervals,
			SkipCleanup:     input.SkipCleanup,
			ArtifactFolder:  input.ArtifactFolder,
		}

		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	It("Should create and upgrade a workload cluster", func() {
		By("Creating a workload cluster")
		ApplyClusterTemplateAndWait(ctx, ApplyClusterTemplateAndWaitInput{
			ClusterProxy: input.BootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                clusterctlLogFolder,
				ClusterctlConfigPath:     input.ClusterctlConfigPath,
				KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   *input.InfrastructureProvider,
				Flavor:                   ptr.Deref(input.Flavor, ""),
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        input.E2EConfig.GetVariable(KubernetesVersion),
				ControlPlaneMachineCount: &controlPlaneMachineCount,
				WorkerMachineCount:       &workerMachineCount,
			},
			ControlPlaneWaiters:          input.ControlPlaneWaiters,
			WaitForClusterIntervals:      input.E2EConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
		}, result)

		if result.Cluster.Spec.Topology != nil {
			// Cluster is using ClusterClass, upgrade via topology.
			By("Upgrading the Cluster topology")
			mgmtClient := input.BootstrapClusterProxy.GetClient()

			By("Patching the new Kubernetes version to Cluster topology")
			patchHelper, err := patch.NewHelper(result.Cluster, mgmtClient)
			Expect(err).ToNot(HaveOccurred())

			result.Cluster.Spec.Topology.Version = kubernetesVersionUpgradeTo

			Eventually(func() error {
				return patchHelper.Patch(ctx, result.Cluster)
			}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to patch Cluster topology %s with version %s", klog.KObj(result.Cluster), kubernetesVersionUpgradeTo)

			By("Waiting for control-plane machines to have the upgraded kubernetes version")
			framework.WaitForControlPlaneMachinesToBeUpgraded(ctx, framework.WaitForControlPlaneMachinesToBeUpgradedInput{
				Lister:                   mgmtClient,
				Cluster:                  result.Cluster,
				MachineCount:             int(*result.ControlPlane.Spec.Replicas),
				KubernetesUpgradeVersion: kubernetesVersionUpgradeTo,
			}, input.E2EConfig.GetIntervals(specName, "wait-machine-upgrade")...)

			for _, deployment := range result.MachineDeployments {
				if *deployment.Spec.Replicas > 0 {
					Byf("Waiting for Kubernetes versions of machines in MachineDeployment %s to be upgraded to %s",
						klog.KObj(deployment), kubernetesVersionUpgradeTo)
					framework.WaitForMachineDeploymentMachinesToBeUpgraded(ctx, framework.WaitForMachineDeploymentMachinesToBeUpgradedInput{
						Lister:                   mgmtClient,
						Cluster:                  result.Cluster,
						MachineCount:             int(*deployment.Spec.Replicas),
						KubernetesUpgradeVersion: kubernetesVersionUpgradeTo,
						MachineDeployment:        *deployment,
					}, input.E2EConfig.GetIntervals(specName, "wait-machine-upgrade")...)
				}
			}

			for _, pool := range result.MachinePools {
				if *pool.Spec.Replicas > 0 {
					Byf("Waiting for Kubernetes versions of machines in MachinePool %s to be upgraded to %s",
						klog.KObj(pool), kubernetesVersionUpgradeTo)
					framework.WaitForMachinePoolInstancesToBeUpgraded(ctx, framework.WaitForMachinePoolInstancesToBeUpgradedInput{
						Getter:                   mgmtClient,
						WorkloadClusterGetter:    input.BootstrapClusterProxy.GetWorkloadCluster(ctx, result.Cluster.Namespace, result.Cluster.Name).GetClient(),
						Cluster:                  result.Cluster,
						MachineCount:             int(*pool.Spec.Replicas),
						KubernetesUpgradeVersion: kubernetesVersionUpgradeTo,
						MachinePool:              pool,
					}, input.E2EConfig.GetIntervals(specName, "wait-machine-pool-upgrade")...)
				}
			}
		} else {
			By("Upgrading the Kubernetes control-plane")
			UpgradeControlPlaneAndWaitForUpgrade(ctx, UpgradeControlPlaneAndWaitForUpgradeInput{
				ClusterProxy:                input.BootstrapClusterProxy,
				Cluster:                     result.Cluster,
				ControlPlane:                result.ControlPlane,
				KubernetesUpgradeVersion:    kubernetesVersionUpgradeTo,
				WaitForMachinesToBeUpgraded: input.E2EConfig.GetIntervals(specName, "wait-machine-upgrade"),
			})

			By("Upgrading the machine deployment")
			framework.UpgradeMachineDeploymentsAndWait(ctx, framework.UpgradeMachineDeploymentsAndWaitInput{
				ClusterProxy:                input.BootstrapClusterProxy,
				Cluster:                     result.Cluster,
				UpgradeVersion:              kubernetesVersionUpgradeTo,
				MachineDeployments:          result.MachineDeployments,
				WaitForMachinesToBeUpgraded: input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
			})
		}

		By("Waiting until nodes are ready")
		workloadProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, result.Cluster.Name)
		workloadClient := workloadProxy.GetClient()
		framework.WaitForNodesReady(ctx, framework.WaitForNodesReadyInput{
			Lister:            workloadClient,
			KubernetesVersion: kubernetesVersionUpgradeTo,
			Count:             int(result.ExpectedTotalNodes()),
			WaitForNodesReady: input.E2EConfig.GetIntervals(specName, "wait-nodes-ready"),
		})
	})
}

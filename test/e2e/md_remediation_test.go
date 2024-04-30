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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("When testing MachineDeployment remediation", func() {
	var (
		ctx                    = context.TODO()
		specName               = "machine-deployment-remediation"
		namespace              *corev1.Namespace
		cancelWatches          context.CancelFunc
		result                 *ApplyClusterTemplateAndWaitResult
		clusterName            string
		clusterctlLogFolder    string
		infrastructureProvider string
	)

	BeforeEach(func() {
		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		clusterName = fmt.Sprintf("capik3s-md-remediation-%s", util.RandomString(6))
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

	Context("Machine deployment remediation", func() {
		It("Should replace unhealthy machines", func() {
			By("Creating a workload cluster")
			ApplyClusterTemplateAndWait(ctx, ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   infrastructureProvider,
					Flavor:                   "md-remediation",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)

			// TODO: this should be re-written like the KCP remediation test, because the current implementation
			//   only tests that MHC applies the unhealthy condition but it doesn't test that the unhealthy machine is delete and a replacement machine comes up.
			By("Setting a machine unhealthy and wait for MachineDeployment remediation")
			framework.DiscoverMachineHealthChecksAndWaitForRemediation(ctx, framework.DiscoverMachineHealthCheckAndWaitForRemediationInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   result.Cluster,
				WaitForMachineRemediation: e2eConfig.GetIntervals(specName, "wait-machine-remediation"),
			})

			By("Waiting until nodes are ready")
			workloadProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, result.Cluster.Name)
			workloadClient := workloadProxy.GetClient()
			framework.WaitForNodesReady(ctx, framework.WaitForNodesReadyInput{
				Lister:            workloadClient,
				KubernetesVersion: e2eConfig.GetVariable(KubernetesVersion),
				Count:             int(result.ExpectedTotalNodes()),
				WaitForNodesReady: e2eConfig.GetIntervals(specName, "wait-nodes-ready"),
			})

			By("PASSED!")
		})
	})
})

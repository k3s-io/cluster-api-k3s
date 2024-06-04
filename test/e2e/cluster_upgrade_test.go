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
	. "github.com/onsi/ginkgo/v2"
	"k8s.io/utils/ptr"
)

var _ = Describe("Workload cluster upgrade [K3s-Upgrade]", func() {
	Context("Upgrading a cluster with 1 control plane", func() {
		ClusterUpgradeSpec(ctx, func() ClusterUpgradeSpecInput {
			return ClusterUpgradeSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				InfrastructureProvider:   ptr.To("docker"),
				ControlPlaneMachineCount: ptr.To[int64](1),
				WorkerMachineCount:       ptr.To[int64](2),
			}
		})
	})

	Context("Upgrading a cluster with HA control plane", func() {
		ClusterUpgradeSpec(ctx, func() ClusterUpgradeSpecInput {
			return ClusterUpgradeSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				InfrastructureProvider:   ptr.To("docker"),
				ControlPlaneMachineCount: ptr.To[int64](3),
				WorkerMachineCount:       ptr.To[int64](1),
			}
		})
	})

	Context("Upgrading a cluster with ClusterClass [ClusterClass]", func() {
		ClusterUpgradeSpec(ctx, func() ClusterUpgradeSpecInput {
			return ClusterUpgradeSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				Flavor:                   ptr.To("topology"),
				InfrastructureProvider:   ptr.To("docker"),
				ControlPlaneMachineCount: ptr.To[int64](1),
				WorkerMachineCount:       ptr.To[int64](2),
			}
		})
	})
})

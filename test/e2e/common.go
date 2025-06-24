//go:build e2e
// +build e2e

/*
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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/yaml"
)

// Test suite constants for e2e config variables.
const (
	KubernetesVersionManagement     = "KUBERNETES_VERSION_MANAGEMENT"
	KubernetesVersion               = "KUBERNETES_VERSION"
	KubernetesVersionUpgradeTo      = "KUBERNETES_VERSION_UPGRADE_TO"
	CPMachineTemplateUpgradeTo      = "CONTROL_PLANE_MACHINE_TEMPLATE_UPGRADE_TO"
	WorkersMachineTemplateUpgradeTo = "WORKERS_MACHINE_TEMPLATE_UPGRADE_TO"
	IPFamily                        = "IP_FAMILY"
	KindImageVersion                = "KIND_IMAGE_VERSION"
	CapiCoreVersion                 = "CAPI_CORE_VERSION"
	K3sCapiCurrentVersion           = "K3S_CAPI_CURRENT_VERSION"
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

func setupSpecNamespace(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string) (*corev1.Namespace, context.CancelFunc) {
	Byf("Creating a namespace for hosting the %q test spec", specName)
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   clusterProxy.GetClient(),
		ClientSet: clusterProxy.GetClientSet(),
		Name:      fmt.Sprintf("%s-%s", specName, util.RandomString(6)),
		LogFolder: filepath.Join(artifactFolder, "clusters", clusterProxy.GetName()),
	})

	return namespace, cancelWatches
}

type cleanupInput struct {
	SpecName             string
	ClusterProxy         framework.ClusterProxy
	ClusterctlConfigPath string
	ArtifactFolder       string
	Namespace            *corev1.Namespace
	CancelWatches        context.CancelFunc
	Cluster              *clusterv1.Cluster
	IntervalsGetter      func(spec, key string) []interface{}
	SkipCleanup          bool
	AdditionalCleanup    func()
}

func dumpSpecResourcesAndCleanup(ctx context.Context, input cleanupInput) {
	defer func() {
		input.CancelWatches()
	}()

	if input.Cluster == nil {
		By("Unable to dump workload cluster logs as the cluster is nil")
	} else {
		Byf("Dumping logs from the %q workload cluster", input.Cluster.Name)
		input.ClusterProxy.CollectWorkloadClusterLogs(ctx, input.Cluster.Namespace, input.Cluster.Name, filepath.Join(input.ArtifactFolder, "clusters", input.Cluster.Name))
	}

	Byf("Dumping all the Cluster API resources in the %q namespace", input.Namespace.Name)
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:               input.ClusterProxy.GetClient(),
		KubeConfigPath:       input.ClusterProxy.GetKubeconfigPath(),
		ClusterctlConfigPath: input.ClusterctlConfigPath,
		Namespace:            input.Namespace.Name,
		LogPath:              filepath.Join(input.ArtifactFolder, "clusters", input.ClusterProxy.GetName(), "resources"),
	})

	if input.SkipCleanup {
		return
	}

	Byf("Deleting all clusters in the %s namespace", input.Namespace.Name)
	// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
	// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
	// instead of DeleteClusterAndWait
	framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
		ClusterProxy:         input.ClusterProxy,
		ClusterctlConfigPath: input.ClusterctlConfigPath,
		Namespace:            input.Namespace.Name,
	}, input.IntervalsGetter(input.SpecName, "wait-delete-cluster")...)

	Byf("Deleting namespace used for hosting the %q test spec", input.SpecName)
	framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
		Deleter: input.ClusterProxy.GetClient(),
		Name:    input.Namespace.Name,
	})

	if input.AdditionalCleanup != nil {
		Byf("Running additional cleanup for the %q test spec", input.SpecName)
		input.AdditionalCleanup()
	}
}

func localLoadE2EConfig(configPath string) *clusterctl.E2EConfig {
	configData, err := os.ReadFile(configPath) //nolint:gosec
	Expect(err).ToNot(HaveOccurred(), "Failed to read the e2e test config file")
	Expect(configData).ToNot(BeEmpty(), "The e2e test config file should not be empty")

	config := &clusterctl.E2EConfig{}
	Expect(yaml.Unmarshal(configData, config)).To(Succeed(), "Failed to convert the e2e test config file to yaml")

	config.Defaults()
	config.AbsPaths(filepath.Dir(configPath))

	// TODO: this is the reason why we can't use this at present for the RKE2 tests
	// Expect(config.Validate()).To(Succeed(), "The e2e test config file is not valid")

	return config
}

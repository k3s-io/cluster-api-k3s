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

package main

import (
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/crdmigrator"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	bootstrapv1beta1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta1"
	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/bootstrap/controllers"
	"github.com/k3s-io/cluster-api-k3s/pkg/crdmigration"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1beta1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = bootstrapv1beta1.AddToScheme(scheme)
	_ = bootstrapv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions;customresourcedefinitions/status,verbs=update;patch,resourceNames=kthreesconfigs.bootstrap.cluster.x-k8s.io;kthreesconfigtemplates.bootstrap.cluster.x-k8s.io
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kthreesconfigs;kthreesconfigtemplates,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kthreesconfigs/status,verbs=update;patch

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var syncPeriod time.Duration
	var concurrencyNumber int
	var skipCRDMigrationPhases []string

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")

	flag.IntVar(&concurrencyNumber, "concurrency", 1,
		"Number of core resources to process simultaneously")

	crdmigration.AddSkipPhasesFlag(flag.CommandLine, &skipCRDMigrationPhases)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	ctx := ctrl.SetupSignalHandler()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "6b2b21b1.k8s.io",
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// NOTE: The kubebuilder RBAC markers above main have to be kept in sync with the CRDs migrated here.
	if err := crdmigration.SetupWithManager(ctx, mgr, skipCRDMigrationPhases, map[client.Object]crdmigrator.ByObjectConfig{
		&bootstrapv1.KThreesConfig{}:         {UseCache: false, UseStatusForStorageVersionMigration: true},
		&bootstrapv1.KThreesConfigTemplate{}: {UseCache: false},
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CRDMigrator")
		os.Exit(1)
	}

	if err = (&controllers.KThreesConfigReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("KThreesConfig"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, concurrencyNumber); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KThreesConfig")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = (&bootstrapv1.KThreesConfig{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "KThreesConfig")
			os.Exit(1)
		}
		if err = (&bootstrapv1.KThreesConfigTemplate{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "KThreesConfigTemplate")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("Starting manager", "concurrency", concurrencyNumber)
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

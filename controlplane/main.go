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

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	runtimev1 "sigs.k8s.io/cluster-api/api/runtime/v1beta2"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimeclient "sigs.k8s.io/cluster-api/exp/runtime/client"
	"sigs.k8s.io/cluster-api/feature"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	bootstrapv1beta1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta1"
	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1beta1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta1"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/controlplane/controllers"
	capiextensionconfig "github.com/k3s-io/cluster-api-k3s/pkg/capi/extensionconfig"
	capiregistry "github.com/k3s-io/cluster-api-k3s/pkg/capi/registry"
	capiruntimeclient "github.com/k3s-io/cluster-api-k3s/pkg/capi/runtimeclient"
	"github.com/k3s-io/cluster-api-k3s/pkg/etcd"
)

var (
	catalog  = runtimecatalog.New()
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1beta1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = bootstrapv1beta1.AddToScheme(scheme)
	_ = bootstrapv1.AddToScheme(scheme)
	_ = runtimev1.AddToScheme(scheme)
	_ = runtimehooksv1.AddToCatalog(catalog)

	_ = controlplanev1beta1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var syncPeriod time.Duration
	var etcdDialTimeout time.Duration
	var etcdCallTimeout time.Duration
	var concurrencyNumber int
	var runtimeExtensionCertFile string
	var runtimeExtensionKeyFile string

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")

	flag.DurationVar(&etcdDialTimeout, "etcd-dial-timeout-duration", 10*time.Second,
		"Duration that the etcd client waits at most to establish a connection with etcd")

	flag.DurationVar(&etcdCallTimeout, "etcd-call-timeout-duration", etcd.DefaultCallTimeout,
		"Duration that the etcd client waits at most for read and write operations to etcd.")

	flag.IntVar(&concurrencyNumber, "concurrency", 1,
		"Number of core resources to process simultaneously")

	flag.StringVar(&runtimeExtensionCertFile, "runtime-extension-client-cert-file", "",
		"Path of the PEM-encoded client certificate to be used when calling runtime extensions.")
	flag.StringVar(&runtimeExtensionKeyFile, "runtime-extension-client-key-file", "",
		"Path of the PEM-encoded client key to be used when calling runtime extensions.")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	feature.MutableGates.AddFlag(pflag.CommandLine)
	pflag.Parse()

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
		LeaderElectionID: "148fa072.controlplane.cluster.x-k8s.io",
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var runtimeClient runtimeclient.Client
	if feature.Gates.Enabled(feature.InPlaceUpdates) {
		createdRuntimeClient, certWatcher, err := capiruntimeclient.New(capiruntimeclient.Options{
			CertFile: runtimeExtensionCertFile,
			KeyFile:  runtimeExtensionKeyFile,
			Catalog:  catalog,
			Registry: capiregistry.New(),
			Client:   mgr.GetClient(),
		})
		if err != nil {
			setupLog.Error(err, "unable to create RuntimeSDK client")
			os.Exit(1)
		}
		runtimeClient = createdRuntimeClient

		if certWatcher != nil {
			if err := mgr.Add(certWatcher); err != nil {
				setupLog.Error(err, "unable to add RuntimeSDK client cert-watcher to the manager")
				os.Exit(1)
			}
		}

		if err := (&capiextensionconfig.Reconciler{
			Client:        mgr.GetClient(),
			APIReader:     mgr.GetAPIReader(),
			RuntimeClient: runtimeClient,
			ReadOnly:      true,
		}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: 10}); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ExtensionConfig")
			os.Exit(1)
		}
	}

	ctrPlaneLogger := ctrl.Log.WithName("controllers").WithName("KThreesControlPlane")
	if err = (&controllers.KThreesControlPlaneReconciler{
		Client:          mgr.GetClient(),
		Log:             ctrPlaneLogger,
		Scheme:          mgr.GetScheme(),
		RuntimeClient:   runtimeClient,
		EtcdDialTimeout: etcdDialTimeout,
		EtcdCallTimeout: etcdCallTimeout,
	}).SetupWithManager(ctx, mgr, &ctrPlaneLogger, concurrencyNumber); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KThreesControlPlane")
		os.Exit(1)
	}

	ctrMachineLogger := ctrl.Log.WithName("controllers").WithName("Machine")
	if err = (&controllers.MachineReconciler{
		Client:          mgr.GetClient(),
		Log:             ctrMachineLogger,
		Scheme:          mgr.GetScheme(),
		EtcdDialTimeout: etcdDialTimeout,
		EtcdCallTimeout: etcdCallTimeout,
	}).SetupWithManager(ctx, mgr, &ctrMachineLogger, concurrencyNumber); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Machine")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = (&controlplanev1.KThreesControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "KThreesControlPlane")
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

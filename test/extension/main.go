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

package main

import (
	"flag"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/k3s-io/cluster-api-k3s/test/extension/handlers/inplaceupdate"
)

var (
	catalog = runtimecatalog.New()
	scheme  = runtime.NewScheme()
)

func init() {
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = runtimehooksv1.AddToCatalog(catalog)
}

func main() {
	var enableLeaderElection bool
	var webhookCertDir string
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election")
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/certs", "Directory containing the webhook TLS certificate")

	logOptions := zap.Options{Development: true}
	logOptions.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&logOptions)))

	webhookServer, err := server.New(server.Options{
		Port:     9443,
		CertDir:  webhookCertDir,
		CertName: "tls.crt",
		KeyName:  "tls.key",
		Catalog:  catalog,
	})
	if err != nil {
		ctrl.Log.Error(err, "Unable to create Runtime Extension webhook server")
		os.Exit(1)
	}

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "k3s-test-extension",
		LeaderElectionNamespace:    os.Getenv("POD_NAMESPACE"),
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		HealthProbeBindAddress:     ":9440",
		Metrics:                    metricsserver.Options{BindAddress: "0"},
		WebhookServer:              webhookServer,
		Client: client.Options{Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&corev1.ConfigMap{},
				&clusterv1.Machine{},
			},
		}},
	})
	if err != nil {
		ctrl.Log.Error(err, "Unable to create manager")
		os.Exit(1)
	}

	handlers := inplaceupdate.NewExtensionHandlers(manager.GetClient())
	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.CanUpdateMachine,
		Name:        "can-update-machine",
		HandlerFunc: handlers.DoCanUpdateMachine,
	}); err != nil {
		ctrl.Log.Error(err, "Unable to register CanUpdateMachine handler")
		os.Exit(1)
	}
	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.UpdateMachine,
		Name:        "update-machine",
		HandlerFunc: handlers.DoUpdateMachine,
	}); err != nil {
		ctrl.Log.Error(err, "Unable to register UpdateMachine handler")
		os.Exit(1)
	}

	if err := manager.AddReadyzCheck("webhook", manager.GetWebhookServer().StartedChecker()); err != nil {
		ctrl.Log.Error(err, "Unable to add readiness check")
		os.Exit(1)
	}
	if err := manager.AddHealthzCheck("webhook", manager.GetWebhookServer().StartedChecker()); err != nil {
		ctrl.Log.Error(err, "Unable to add health check")
		os.Exit(1)
	}

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "Manager exited with an error")
		os.Exit(1)
	}
}

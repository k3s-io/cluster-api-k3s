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

// Package crdmigration sets up the Cluster API CRDMigrator for the CRDs owned by a provider manager.
//
// The CRDMigrator migrates stored objects of a CRD to the current storage version and cleans up
// managedFields of old API versions, and then removes old versions from the CRD's status.storedVersions.
// This is required before an old API version (e.g. v1beta1) can be removed from a CRD.
package crdmigration

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/cluster-api/controllers/crdmigrator"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// SkipPhasesFlagName is the name of the flag used to skip CRD migration phases.
const SkipPhasesFlagName = "skip-crd-migration-phases"

var validPhases = sets.New(
	string(crdmigrator.StorageVersionMigrationPhase),
	string(crdmigrator.CleanupManagedFieldsPhase),
)

// AddSkipPhasesFlag registers the --skip-crd-migration-phases flag on fs, storing the parsed phases in phases.
func AddSkipPhasesFlag(fs *flag.FlagSet, phases *[]string) {
	fs.Func(SkipPhasesFlagName,
		fmt.Sprintf("Comma-separated list of CRD migration phases to skip. Valid values are: %s.",
			strings.Join(sets.List(validPhases), ", ")),
		func(value string) error {
			for _, p := range strings.Split(value, ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if !validPhases.Has(p) {
					return fmt.Errorf("invalid CRD migration phase %q, valid values are: %s",
						p, strings.Join(sets.List(validPhases), ", "))
				}
				*phases = append(*phases, p)
			}
			return nil
		})
}

// SetupWithManager sets up a CRDMigrator for the given objects.
// The manager's RBAC must allow:
//   - get, list and watch on customresourcedefinitions,
//   - update and patch on customresourcedefinitions and customresourcedefinitions/status for the migrated CRDs,
//   - get, list, watch, patch and update on the migrated CRs,
//   - update and patch on the status subresource of CRs configured with UseStatusForStorageVersionMigration.
func SetupWithManager(ctx context.Context, mgr ctrl.Manager, skipPhases []string, config map[client.Object]crdmigrator.ByObjectConfig) error {
	crdMigratorSkipPhases := make([]crdmigrator.Phase, 0, len(skipPhases))
	for _, p := range skipPhases {
		crdMigratorSkipPhases = append(crdMigratorSkipPhases, crdmigrator.Phase(p))
	}

	return (&crdmigrator.CRDMigrator{
		Client:                 mgr.GetClient(),
		APIReader:              mgr.GetAPIReader(),
		SkipCRDMigrationPhases: crdMigratorSkipPhases,
		Config:                 config,
		// The CRDMigrator is run with only concurrency 1 to ensure we don't overwhelm the apiserver by patching a
		// lot of CRs concurrently.
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: 1})
}

/*
Copyright 2023 The Kubernetes Authors.

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

// Package ssa contains utils related to Server-Side-Apply.
package ssa

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/k3s-io/cluster-api-k3s/pkg/util/contract"
)

const classicManager = "manager"

// DropManagedFields modifies the managedFields entries on the object that belong to "manager" (Operation=Update)
// to drop ownership of the given paths if there is no field yet that is managed by `ssaManager`.
//
// If we want to be able to drop fields that were previously owned by the "manager" we have to ensure that
// fields are not co-owned by "manager" and `ssaManager`. Otherwise, when we drop the fields with SSA
// (i.e. `ssaManager`) the fields would remain as they are still owned by "manager".
// The following code will do a one-time update on the managed fields.
// We won't do this on subsequent reconciles. This case will be identified by checking if `ssaManager` owns any fields.
// Dropping ownership in paths for existing "manager" entries (which could also be from other controllers) is safe,
// as we assume that if other controllers are still writing fields on the object they will just do it again and thus
// gain ownership again.
func DropManagedFields(ctx context.Context, c client.Client, obj client.Object, ssaManager string, paths []contract.Path) error {
	// Return if `ssaManager` already owns any fields.
	if hasFieldsManagedBy(obj, ssaManager) {
		return nil
	}

	// Since there is no field managed by `ssaManager` it means that
	// this is the first time this object is being processed after the controller calling this function
	// started to use SSA patches.
	// It is required to clean-up managedFields from entries created by the regular patches.
	// This will ensure that `ssaManager` will be able to modify the fields that
	// were originally owned by "manager".
	base := obj.DeepCopyObject().(client.Object)

	// Modify managedFieldEntry for manager=manager and operation=update to drop ownership
	// for the given paths to avoid having two managers holding values.
	originalManagedFields := obj.GetManagedFields()
	managedFields := make([]metav1.ManagedFieldsEntry, 0, len(originalManagedFields))
	for _, managedField := range originalManagedFields {
		if managedField.Manager == classicManager &&
			managedField.Operation == metav1.ManagedFieldsOperationUpdate {
			// Unmarshal the managed fields into a map[string]interface{}
			fieldsV1 := map[string]interface{}{}
			if err := json.Unmarshal(managedField.FieldsV1.Raw, &fieldsV1); err != nil {
				return errors.Wrap(err, "failed to unmarshal managed fields")
			}

			// Filter out the ownership for the given paths.
			FilterIntent(&FilterIntentInput{
				Path:         contract.Path{},
				Value:        fieldsV1,
				ShouldFilter: IsPathIgnored(paths),
			})

			fieldsV1Raw, err := json.Marshal(fieldsV1)
			if err != nil {
				return errors.Wrap(err, "failed to marshal managed fields")
			}
			managedField.FieldsV1.Raw = fieldsV1Raw

			managedFields = append(managedFields, managedField)
		} else {
			// Do not modify the entry. Use as is.
			managedFields = append(managedFields, managedField)
		}
	}

	obj.SetManagedFields(managedFields)

	return c.Patch(ctx, obj, client.MergeFrom(base))
}

// MigrateManagedFields migrates managedFields when fieldManager owns the cluster-name label.
// The migration deletes all non-status managedFields entries for fieldManager:Apply and manager:Update.
// Additionally, it adds a seed entry for metadataFieldManager.
//
// This must be called once for every related object created with Cluster API K3s <= v0.4.0.
// Starting after v0.4.0, the metadata field manager owns labels and annotations while the main
// field manager owns everything else.
func MigrateManagedFields(ctx context.Context, c client.Client, obj client.Object, fieldManager, metadataFieldManager string) error {
	objectKey := client.ObjectKeyFromObject(obj)
	objectGVK, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return errors.Wrapf(err, "failed to migrate managedFields for object %s",
			klog.KRef(objectKey.Namespace, objectKey.Name))
	}

	// Check if a migration is still needed. This should be only done once per object.
	needsMigration, err := needsMigration(obj, fieldManager)
	if err != nil {
		return errors.Wrapf(err, "failed to migrate managedFields for %s %s",
			objectGVK.Kind, klog.KRef(objectKey.Namespace, objectKey.Name))
	}
	if !needsMigration {
		return nil
	}

	seedFieldsV1, err := migrationSeedFields(obj, fieldManager)
	if err != nil {
		return errors.Wrapf(err, "failed to migrate managedFields for %s %s",
			objectGVK.Kind, klog.KRef(objectKey.Namespace, objectKey.Name))
	}

	base := obj.DeepCopyObject().(client.Object)

	// Remove managedFields for fieldManager:Apply and manager:Update.
	originalManagedFields := obj.GetManagedFields()
	managedFields := make([]metav1.ManagedFieldsEntry, 0, len(originalManagedFields))
	for _, managedField := range originalManagedFields {
		if managedField.Manager == fieldManager &&
			managedField.Operation == metav1.ManagedFieldsOperationApply &&
			managedField.Subresource == "" {
			continue
		}
		if managedField.Manager == classicManager &&
			managedField.Operation == metav1.ManagedFieldsOperationUpdate &&
			managedField.Subresource == "" {
			continue
		}
		managedFields = append(managedFields, managedField)
	}

	// Add a seeding managedFields entry to prevent SSA from creating or inferring a default entry when the first
	// SSA is applied. FieldsV1 cannot be empty, so metadata.name is included and cleaned up by the next SSA patch
	// from metadataFieldManager. The old manager's label and annotation ownership is transferred in the same
	// optimistic-lock patch so the next SSA patch can delete omitted metadata without a partially migrated state.
	managedFields = append(managedFields, metav1.ManagedFieldsEntry{
		Manager:    metadataFieldManager,
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: objectGVK.GroupVersion().String(),
		Time:       ptr.To(metav1.Now()),
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: seedFieldsV1},
	})

	obj.SetManagedFields(managedFields)

	// Use optimistic locking to avoid accidentally rolling back managedFields.
	if err := c.Patch(ctx, obj, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})); err != nil {
		return errors.Wrapf(err, "failed to migrate managedFields for %s %s",
			objectGVK.Kind, klog.KRef(objectKey.Namespace, objectKey.Name))
	}
	return nil
}

// needsMigration returns true if fieldManager:Apply owns the clusterv1.ClusterNameLabel.
func needsMigration(obj client.Object, fieldManager string) (bool, error) {
	for _, managedField := range obj.GetManagedFields() {
		if managedField.Manager == fieldManager &&
			managedField.Operation == metav1.ManagedFieldsOperationApply &&
			managedField.Subresource == "" {
			// If fieldManager does not own the cluster-name label, migration is not needed.
			if !bytes.Contains(managedField.FieldsV1.Raw, []byte("f:cluster.x-k8s.io/cluster-name")) {
				return false, nil
			}

			fieldsV1 := map[string]interface{}{}
			if err := json.Unmarshal(managedField.FieldsV1.Raw, &fieldsV1); err != nil {
				return false, errors.Wrap(err, "failed to determine if migration is needed: failed to unmarshal managed fields")
			}

			// MigrateManagedFields is only called for BootstrapConfig and InfraMachine objects, which always have
			// the cluster-name label set.
			_, containsClusterNameLabel, err := unstructured.NestedFieldNoCopy(fieldsV1, "f:metadata", "f:labels", "f:"+clusterv1.ClusterNameLabel)
			if err != nil {
				return false, errors.Wrapf(err, "failed to determine if migration is needed: failed to determine if managed field contains the %s label", clusterv1.ClusterNameLabel)
			}
			return containsClusterNameLabel, nil
		}
	}
	return false, nil
}

func migrationSeedFields(obj client.Object, fieldManager string) ([]byte, error) {
	seedMetadata := map[string]interface{}{
		"f:name": map[string]interface{}{},
	}

	for _, managedField := range obj.GetManagedFields() {
		if managedField.Manager != fieldManager ||
			managedField.Operation != metav1.ManagedFieldsOperationApply ||
			managedField.Subresource != "" {
			continue
		}

		fieldsV1 := map[string]interface{}{}
		if err := json.Unmarshal(managedField.FieldsV1.Raw, &fieldsV1); err != nil {
			return nil, errors.Wrap(err, "failed to create migration seed: failed to unmarshal managed fields")
		}

		for _, field := range []string{"f:annotations", "f:labels"} {
			ownedFields, found, err := unstructured.NestedFieldNoCopy(fieldsV1, "f:metadata", field)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to create migration seed for %s", field)
			}
			if found {
				seedMetadata[field] = ownedFields
			}
		}
		break
	}

	seedFieldsV1, err := json.Marshal(map[string]interface{}{
		"f:metadata": seedMetadata,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal migration seed")
	}
	return seedFieldsV1, nil
}

// CleanUpManagedFieldsForSSAAdoption deletes the managedFields entries on the object that belong to "manager" (Operation=Update)
// if there is no field yet that is managed by `ssaManager`.
// It adds an "empty" entry in managedFields of the object if no field is currently managed by `ssaManager`.
//
// In previous versions of Cluster API K3S (<= v0.2.0) we were writing objects with Create and Patch which resulted in fields
// being owned by the "manager". After switching to Server-Side-Apply (SSA), fields will be owned by `ssaManager`.
//
// If we want to be able to drop fields that were previously owned by the "manager" we have to ensure that
// fields are not co-owned by "manager" and `ssaManager`. Otherwise, when we drop the fields with SSA
// (i.e. `ssaManager`) the fields would remain as they are still owned by "manager".
// The following code will do a one-time update on the managed fields to drop all entries for "manager".
// We won't do this on subsequent reconciles. This case will be identified by checking if `ssaManager` owns any fields.
// Dropping all existing "manager" entries (which could also be from other controllers) is safe, as we assume that if
// other controllers are still writing fields on the object they will just do it again and thus gain ownership again.
func CleanUpManagedFieldsForSSAAdoption(ctx context.Context, c client.Client, obj client.Object, ssaManager string) error {
	// Return if `ssaManager` already owns any fields.
	if hasFieldsManagedBy(obj, ssaManager) {
		return nil
	}

	// Since there is no field managed by `ssaManager` it means that
	// this is the first time this object is being processed after the controller calling this function
	// started to use SSA patches.
	// It is required to clean-up managedFields from entries created by the regular patches.
	// This will ensure that `ssaManager` will be able to modify the fields that
	// were originally owned by "manager".
	base := obj.DeepCopyObject().(client.Object)

	// Remove managedFieldEntry for manager=manager and operation=update to prevent having two managers holding values.
	originalManagedFields := obj.GetManagedFields()
	managedFields := make([]metav1.ManagedFieldsEntry, 0, len(originalManagedFields))
	for i := range originalManagedFields {
		if originalManagedFields[i].Manager == classicManager &&
			originalManagedFields[i].Operation == metav1.ManagedFieldsOperationUpdate {
			continue
		}
		managedFields = append(managedFields, originalManagedFields[i])
	}

	// Add a seeding managedFieldEntry for SSA executed by the management controller, to prevent SSA to create/infer
	// a default managedFieldEntry when the first SSA is applied.
	// More specifically, if an existing object doesn't have managedFields when applying the first SSA the API server
	// creates an entry with operation=Update (kind of guessing where the object comes from), but this entry ends up
	// acting as a co-ownership and we want to prevent this.
	// NOTE: fieldV1Map cannot be empty, so we add metadata.name which will be cleaned up at the first SSA patch.
	fieldV1Map := map[string]interface{}{
		"f:metadata": map[string]interface{}{
			"f:name": map[string]interface{}{},
		},
	}
	fieldV1, err := json.Marshal(fieldV1Map)
	if err != nil {
		return errors.Wrap(err, "failed to create seeding fieldV1Map for cleaning up legacy managed fields")
	}
	now := metav1.Now()
	gvk, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return errors.Wrapf(err, "failed to get GroupVersionKind of object %s", klog.KObj(obj))
	}
	managedFields = append(managedFields, metav1.ManagedFieldsEntry{
		Manager:    ssaManager,
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: gvk.GroupVersion().String(),
		Time:       &now,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: fieldV1},
	})

	obj.SetManagedFields(managedFields)

	return c.Patch(ctx, obj, client.MergeFrom(base))
}

// hasFieldsManagedBy returns true if any of the fields in obj are managed by manager.
func hasFieldsManagedBy(obj client.Object, manager string) bool {
	managedFields := obj.GetManagedFields()
	for _, mf := range managedFields {
		if mf.Manager == manager {
			return true
		}
	}
	return false
}

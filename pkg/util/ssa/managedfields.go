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
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const classicManager = "manager"

// RemoveManagedFieldsForLabelsAndAnnotations drops label and ordinary annotation ownership from fieldManager's main Apply entry.
func RemoveManagedFieldsForLabelsAndAnnotations(
	ctx context.Context,
	c client.Client,
	apiReader client.Reader,
	obj client.Object,
	fieldManager string,
	protectedAnnotations ...string,
) error {
	objectKey := client.ObjectKeyFromObject(obj)
	current := obj.DeepCopyObject().(client.Object)
	firstAttempt := true

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if !firstAttempt {
			current = obj.DeepCopyObject().(client.Object)
			current.SetResourceVersion("")
			current.SetManagedFields(nil)
			if err := apiReader.Get(ctx, objectKey, current); err != nil {
				return err
			}
		}
		firstAttempt = false

		managedFields, changed, err := removeManagedFieldsForLabelsAndAnnotations(
			current.GetManagedFields(),
			fieldManager,
			protectedAnnotations,
		)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}

		base := current.DeepCopyObject().(client.Object)
		current.SetManagedFields(managedFields)
		return c.Patch(ctx, current, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
	})
	if err != nil {
		return errors.Wrap(err, "failed to remove label and annotation managed fields")
	}

	if err := c.Scheme().Convert(current, obj, ctx); err != nil {
		return errors.Wrap(err, "failed to write migrated object")
	}
	obj.GetObjectKind().SetGroupVersionKind(current.GetObjectKind().GroupVersionKind())
	return nil
}

func removeManagedFieldsForLabelsAndAnnotations(
	originalManagedFields []metav1.ManagedFieldsEntry,
	fieldManager string,
	protectedAnnotations []string,
) ([]metav1.ManagedFieldsEntry, bool, error) {
	managedFields := make([]metav1.ManagedFieldsEntry, 0, len(originalManagedFields))
	changed := false
	for _, managedField := range originalManagedFields {
		if managedField.Manager != fieldManager ||
			managedField.Operation != metav1.ManagedFieldsOperationApply ||
			managedField.Subresource != "" {
			managedFields = append(managedFields, managedField)
			continue
		}

		fieldsV1, err := parseManagedFieldsEntry(managedField)
		if err != nil {
			return nil, false, err
		}
		originalFieldsV1 := runtime.DeepCopyJSON(fieldsV1)
		if _, _, err := moveOrdinaryMetadataManagedFields(
			fieldsV1,
			map[string]interface{}{},
			protectedAnnotationFieldNames(protectedAnnotations),
		); err != nil {
			return nil, false, err
		}
		if reflect.DeepEqual(fieldsV1, originalFieldsV1) {
			managedFields = append(managedFields, managedField)
			continue
		}

		changed = true
		if len(fieldsV1) == 0 {
			continue
		}

		fieldsV1Raw, err := json.Marshal(fieldsV1)
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to marshal managed fields")
		}
		managedField.FieldsV1 = &metav1.FieldsV1{Raw: fieldsV1Raw}
		managedFields = append(managedFields, managedField)
	}
	return managedFields, changed, nil
}

// ManagedFieldsMigrationOutcome describes whether and how managedFields migration was performed.
type ManagedFieldsMigrationOutcome string

const (
	ManagedFieldsMigrationUnchanged                ManagedFieldsMigrationOutcome = "Unchanged"
	ManagedFieldsMigrationCompleted                ManagedFieldsMigrationOutcome = "Completed"
	ManagedFieldsMigrationInPlaceUpdateUnsupported ManagedFieldsMigrationOutcome = "InPlaceUpdateUnsupported"

	managedFieldsMigrationUnsupportedReason = "related-object spec ownership spans multiple API versions"
)

// ManagedFieldsMigrationResult is the result of migrating a related object's managedFields.
type ManagedFieldsMigrationResult struct {
	Outcome ManagedFieldsMigrationOutcome
	Reason  string
}

// MigrateManagedFields transfers historical related-object ownership to the current field managers.
func MigrateManagedFields(
	ctx context.Context,
	c client.Client,
	apiReader client.Reader,
	obj client.Object,
	fieldManager string,
	metadataFieldManager string,
	protectedAnnotations ...string,
) (ManagedFieldsMigrationResult, error) {
	objectKey := client.ObjectKeyFromObject(obj)
	objectGVK, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return ManagedFieldsMigrationResult{}, errors.Wrapf(err, "failed to migrate managedFields for object %s",
			klog.KRef(objectKey.Namespace, objectKey.Name))
	}

	current := obj.DeepCopyObject().(client.Object)
	result := ManagedFieldsMigrationResult{Outcome: ManagedFieldsMigrationUnchanged}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current = obj.DeepCopyObject().(client.Object)
		current.SetResourceVersion("")
		current.SetManagedFields(nil)
		if err := apiReader.Get(ctx, objectKey, current); err != nil {
			return err
		}
		current.GetObjectKind().SetGroupVersionKind(objectGVK)

		managedFields, migrationResult, err := migrateManagedFields(
			current.GetManagedFields(),
			fieldManager,
			metadataFieldManager,
			objectGVK.GroupVersion().String(),
			protectedAnnotations,
		)
		if err != nil {
			return err
		}
		result = migrationResult
		if result.Outcome != ManagedFieldsMigrationCompleted {
			return nil
		}

		base := current.DeepCopyObject().(client.Object)
		current.SetManagedFields(managedFields)
		return c.Patch(ctx, current, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
	})
	if err != nil {
		return ManagedFieldsMigrationResult{}, errors.Wrapf(err, "failed to migrate managedFields for %s %s",
			objectGVK.Kind, klog.KRef(objectKey.Namespace, objectKey.Name))
	}

	if err := c.Scheme().Convert(current, obj, ctx); err != nil {
		return ManagedFieldsMigrationResult{}, errors.Wrap(err, "failed to write migrated object")
	}
	obj.GetObjectKind().SetGroupVersionKind(current.GetObjectKind().GroupVersionKind())
	return result, nil
}

type managedFieldsMigrationEntry struct {
	entry   metav1.ManagedFieldsEntry
	fields  map[string]interface{}
	changed bool
	removed bool
}

type managedFieldsMigration struct {
	entries              []managedFieldsMigrationEntry
	mainEntryIndexes     []int
	metadataEntryIndexes []int
	mainVersions         map[string]struct{}
	specByVersion        map[string]map[string]interface{}
	metadataFields       map[string]interface{}
	mainMetadataFields   map[string]interface{}
}

func migrateManagedFields(
	originalManagedFields []metav1.ManagedFieldsEntry,
	fieldManager string,
	metadataFieldManager string,
	objectAPIVersion string,
	protectedAnnotations []string,
) ([]metav1.ManagedFieldsEntry, ManagedFieldsMigrationResult, error) {
	migration, err := collectManagedFieldsMigration(
		originalManagedFields,
		fieldManager,
		metadataFieldManager,
		protectedAnnotationFieldNames(protectedAnnotations),
	)
	if err != nil {
		return nil, ManagedFieldsMigrationResult{}, err
	}

	destinationVersion, hasDestinationVersion, supported := migration.destinationVersion()
	if !supported {
		return originalManagedFields, ManagedFieldsMigrationResult{
			Outcome: ManagedFieldsMigrationInPlaceUpdateUnsupported,
			Reason:  managedFieldsMigrationUnsupportedReason,
		}, nil
	}
	if !hasDestinationVersion && len(migration.mainMetadataFields) > 0 {
		destinationVersion = objectAPIVersion
		hasDestinationVersion = true
	}

	if err := migration.mergeMainEntries(fieldManager, destinationVersion, hasDestinationVersion); err != nil {
		return nil, ManagedFieldsMigrationResult{}, err
	}
	if err := migration.mergeMetadataEntry(metadataFieldManager, objectAPIVersion); err != nil {
		return nil, ManagedFieldsMigrationResult{}, err
	}

	managedFields, err := migration.managedFields()
	if err != nil {
		return nil, ManagedFieldsMigrationResult{}, err
	}
	if reflect.DeepEqual(managedFields, originalManagedFields) {
		return originalManagedFields, ManagedFieldsMigrationResult{Outcome: ManagedFieldsMigrationUnchanged}, nil
	}
	return managedFields, ManagedFieldsMigrationResult{Outcome: ManagedFieldsMigrationCompleted}, nil
}

func collectManagedFieldsMigration(
	originalManagedFields []metav1.ManagedFieldsEntry,
	fieldManager string,
	metadataFieldManager string,
	protectedAnnotationFields map[string]struct{},
) (*managedFieldsMigration, error) {
	migration := &managedFieldsMigration{
		entries:            make([]managedFieldsMigrationEntry, len(originalManagedFields)),
		mainVersions:       map[string]struct{}{},
		specByVersion:      map[string]map[string]interface{}{},
		metadataFields:     map[string]interface{}{},
		mainMetadataFields: map[string]interface{}{},
	}

	for i := range originalManagedFields {
		entry := originalManagedFields[i]
		migration.entries[i].entry = entry
		entryType := managedFieldsMigrationEntryTypeFor(entry, fieldManager, metadataFieldManager)
		if entryType == managedFieldsMigrationEntryUnrelated {
			continue
		}

		fields, err := parseManagedFieldsEntry(entry)
		if err != nil {
			return nil, err
		}
		migration.entries[i].fields = fields

		if entryType == managedFieldsMigrationEntryMetadata {
			migration.metadataEntryIndexes = append(migration.metadataEntryIndexes, i)
			changed, err := moveProtectedAnnotationManagedFields(
				fields,
				migration.mainMetadataFields,
				protectedAnnotationFields,
			)
			if err != nil {
				return nil, errors.Wrapf(
					err,
					"failed to migrate FieldsV1 for managed fields entry %q",
					entry.Manager,
				)
			}
			migration.entries[i].changed = changed
			continue
		}

		if entryType == managedFieldsMigrationEntryClassic {
			changed, protectedChanged, err := moveOrdinaryMetadataManagedFields(
				fields,
				migration.metadataFields,
				protectedAnnotationFields,
			)
			if err != nil {
				return nil, errors.Wrapf(
					err,
					"failed to migrate FieldsV1 for managed fields entry %q",
					entry.Manager,
				)
			}
			if protectedChanged {
				protectedFields, found, err := removeNestedFieldSet(fields, "f:metadata", "f:annotations")
				if err != nil {
					return nil, err
				}
				if found {
					if err := mergeNestedFieldSet(
						migration.mainMetadataFields,
						protectedFields,
						"f:metadata",
						"f:annotations",
					); err != nil {
						return nil, err
					}
				}
			}
			migration.entries[i].changed = changed || protectedChanged
			if err := migration.moveClassicSpecFields(i); err != nil {
				return nil, err
			}
			continue
		}

		changed, _, err := moveOrdinaryMetadataManagedFields(
			fields,
			migration.metadataFields,
			protectedAnnotationFields,
		)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"failed to migrate FieldsV1 for managed fields entry %q",
				entry.Manager,
			)
		}
		migration.entries[i].changed = changed
		if err := migration.trackMainEntry(i); err != nil {
			return nil, err
		}
	}
	return migration, nil
}

type managedFieldsMigrationEntryType int

const (
	managedFieldsMigrationEntryUnrelated managedFieldsMigrationEntryType = iota
	managedFieldsMigrationEntryMain
	managedFieldsMigrationEntryClassic
	managedFieldsMigrationEntryMetadata
)

func managedFieldsMigrationEntryTypeFor(
	entry metav1.ManagedFieldsEntry,
	fieldManager string,
	metadataFieldManager string,
) managedFieldsMigrationEntryType {
	if entry.Subresource != "" {
		return managedFieldsMigrationEntryUnrelated
	}
	if entry.Manager == metadataFieldManager && entry.Operation == metav1.ManagedFieldsOperationApply {
		return managedFieldsMigrationEntryMetadata
	}
	if entry.Manager == classicManager && entry.Operation == metav1.ManagedFieldsOperationUpdate {
		return managedFieldsMigrationEntryClassic
	}
	if entry.Manager == fieldManager && entry.Operation == metav1.ManagedFieldsOperationApply {
		return managedFieldsMigrationEntryMain
	}
	return managedFieldsMigrationEntryUnrelated
}

func moveOrdinaryMetadataManagedFields(
	fields map[string]interface{},
	metadataFields map[string]interface{},
	protectedAnnotationFields map[string]struct{},
) (ordinaryChanged bool, protectedFound bool, err error) {
	changed := false
	labelFields, found, err := removeNestedFieldSet(fields, "f:metadata", "f:labels")
	if err != nil {
		return false, false, err
	}
	if found {
		if err := mergeNestedFieldSet(metadataFields, labelFields, "f:metadata", "f:labels"); err != nil {
			return false, false, err
		}
		changed = true
	}

	annotationFields, found, err := removeNestedFieldSet(fields, "f:metadata", "f:annotations")
	if err != nil {
		return false, false, err
	}
	if !found {
		return changed, false, nil
	}

	ordinaryAnnotations, protectedAnnotations := splitAnnotationManagedFields(
		annotationFields,
		protectedAnnotationFields,
	)
	if len(ordinaryAnnotations) > 0 {
		if err := mergeNestedFieldSet(
			metadataFields,
			ordinaryAnnotations,
			"f:metadata",
			"f:annotations",
		); err != nil {
			return false, false, err
		}
		changed = true
	}
	if len(protectedAnnotations) > 0 {
		if err := mergeNestedFieldSet(
			fields,
			protectedAnnotations,
			"f:metadata",
			"f:annotations",
		); err != nil {
			return false, false, err
		}
	}
	return changed, len(protectedAnnotations) > 0, nil
}

func moveProtectedAnnotationManagedFields(
	fields map[string]interface{},
	mainMetadataFields map[string]interface{},
	protectedAnnotationFields map[string]struct{},
) (bool, error) {
	annotationFields, found, err := removeNestedFieldSet(fields, "f:metadata", "f:annotations")
	if err != nil || !found {
		return false, err
	}
	ordinaryAnnotations, protectedAnnotations := splitAnnotationManagedFields(
		annotationFields,
		protectedAnnotationFields,
	)
	if len(ordinaryAnnotations) > 0 {
		if err := mergeNestedFieldSet(fields, ordinaryAnnotations, "f:metadata", "f:annotations"); err != nil {
			return false, err
		}
	}
	if len(protectedAnnotations) == 0 {
		return false, nil
	}
	if err := mergeNestedFieldSet(
		mainMetadataFields,
		protectedAnnotations,
		"f:metadata",
		"f:annotations",
	); err != nil {
		return false, err
	}
	return true, nil
}

func splitAnnotationManagedFields(
	annotationFields map[string]interface{},
	protectedAnnotationFields map[string]struct{},
) (ordinary, protected map[string]interface{}) {
	ordinary = map[string]interface{}{}
	protected = map[string]interface{}{}
	for field, value := range annotationFields {
		if _, ok := protectedAnnotationFields[field]; ok {
			protected[field] = runtime.DeepCopyJSONValue(value)
			continue
		}
		ordinary[field] = runtime.DeepCopyJSONValue(value)
	}
	return ordinary, protected
}

func protectedAnnotationFieldNames(annotations []string) map[string]struct{} {
	fields := make(map[string]struct{}, len(annotations))
	for _, annotation := range annotations {
		fields["f:"+annotation] = struct{}{}
	}
	return fields
}

func (m *managedFieldsMigration) moveClassicSpecFields(index int) error {
	entry := &m.entries[index]
	specFields, found, err := removeNestedFieldSet(entry.fields, "f:spec")
	if err != nil {
		return errors.Wrapf(
			err,
			"failed to migrate FieldsV1 for managed fields entry %q",
			entry.entry.Manager,
		)
	}
	if !found {
		return nil
	}
	if m.specByVersion[entry.entry.APIVersion] == nil {
		m.specByVersion[entry.entry.APIVersion] = map[string]interface{}{}
	}
	if err := mergeFieldSet(m.specByVersion[entry.entry.APIVersion], specFields); err != nil {
		return errors.Wrap(err, "failed to merge spec managed fields")
	}
	entry.changed = true
	return nil
}

func (m *managedFieldsMigration) trackMainEntry(index int) error {
	entry := m.entries[index]
	m.mainEntryIndexes = append(m.mainEntryIndexes, index)
	if _, found, err := nestedFieldSet(entry.fields, "f:spec"); err != nil {
		return errors.Wrap(err, "failed to inspect main manager spec FieldsV1")
	} else if found && m.specByVersion[entry.entry.APIVersion] == nil {
		m.specByVersion[entry.entry.APIVersion] = map[string]interface{}{}
	}
	if len(entry.fields) > 0 {
		m.mainVersions[entry.entry.APIVersion] = struct{}{}
	}
	return nil
}

func (m *managedFieldsMigration) destinationVersion() (string, bool, bool) {
	destinationVersions := map[string]struct{}{}
	for version := range m.specByVersion {
		destinationVersions[version] = struct{}{}
	}
	for version := range m.mainVersions {
		destinationVersions[version] = struct{}{}
	}
	if len(destinationVersions) > 1 {
		return "", false, false
	}

	for version := range destinationVersions {
		return version, true, true
	}
	return "", false, true
}

func (m *managedFieldsMigration) mergeMainEntries(
	fieldManager string,
	destinationVersion string,
	hasDestinationVersion bool,
) error {
	mainDestinationIndex := -1
	mainFields := map[string]interface{}{}
	for _, i := range m.mainEntryIndexes {
		if len(m.entries[i].fields) == 0 {
			m.entries[i].removed = true
			continue
		}
		if m.entries[i].entry.APIVersion == destinationVersion && mainDestinationIndex == -1 {
			mainDestinationIndex = i
		}
		if err := mergeFieldSet(mainFields, m.entries[i].fields); err != nil {
			return errors.Wrap(err, "failed to merge main manager fields")
		}
		if i != mainDestinationIndex {
			m.entries[i].removed = true
		}
	}
	if !hasDestinationVersion {
		return nil
	}
	if specFields := m.specByVersion[destinationVersion]; specFields != nil {
		if err := mergeNestedFieldSet(mainFields, specFields, "f:spec"); err != nil {
			return errors.Wrap(err, "failed to merge migrated spec fields")
		}
	}
	if err := mergeFieldSet(mainFields, m.mainMetadataFields); err != nil {
		return errors.Wrap(err, "failed to merge controller-owned annotation managed fields")
	}
	if len(mainFields) == 0 {
		return nil
	}
	if mainDestinationIndex == -1 {
		m.entries = append(m.entries, managedFieldsMigrationEntry{
			entry: metav1.ManagedFieldsEntry{
				Manager:    fieldManager,
				Operation:  metav1.ManagedFieldsOperationApply,
				APIVersion: destinationVersion,
				Time:       ptr.To(metav1.Now()),
				FieldsType: "FieldsV1",
			},
			fields:  mainFields,
			changed: true,
		})
		return nil
	}
	m.entries[mainDestinationIndex].fields = mainFields
	m.entries[mainDestinationIndex].removed = false
	m.entries[mainDestinationIndex].changed = true
	return nil
}

func (m *managedFieldsMigration) mergeMetadataEntry(metadataFieldManager, objectAPIVersion string) error {
	if len(m.metadataFields) == 0 {
		return nil
	}

	metadataDestinationIndex := len(m.entries)
	if len(m.metadataEntryIndexes) > 0 {
		metadataDestinationIndex = m.metadataEntryIndexes[0]
	} else {
		m.entries = append(m.entries, managedFieldsMigrationEntry{
			entry: metav1.ManagedFieldsEntry{
				Manager:    metadataFieldManager,
				Operation:  metav1.ManagedFieldsOperationApply,
				APIVersion: objectAPIVersion,
				Time:       ptr.To(metav1.Now()),
				FieldsType: "FieldsV1",
			},
			fields:  map[string]interface{}{},
			changed: true,
		})
	}
	if err := mergeFieldSet(m.entries[metadataDestinationIndex].fields, m.metadataFields); err != nil {
		return errors.Wrap(err, "failed to merge metadata manager fields")
	}
	m.entries[metadataDestinationIndex].changed = true
	return nil
}

func (m *managedFieldsMigration) managedFields() ([]metav1.ManagedFieldsEntry, error) {
	managedFields := make([]metav1.ManagedFieldsEntry, 0, len(m.entries))
	for i := range m.entries {
		entry := m.entries[i]
		if entry.removed || (entry.fields != nil && len(entry.fields) == 0 && entry.changed) {
			continue
		}
		if entry.changed {
			raw, err := json.Marshal(entry.fields)
			if err != nil {
				return nil, errors.Wrap(err, "failed to marshal migrated managed fields")
			}
			entry.entry.FieldsV1 = &metav1.FieldsV1{Raw: raw}
		}
		managedFields = append(managedFields, entry.entry)
	}
	return managedFields, nil
}

func parseManagedFieldsEntry(entry metav1.ManagedFieldsEntry) (map[string]interface{}, error) {
	if entry.FieldsV1 == nil {
		return nil, errors.Errorf(
			"managed fields entry %q (%s, apiVersion %q) has nil FieldsV1",
			entry.Manager,
			entry.Operation,
			entry.APIVersion,
		)
	}
	fields := map[string]interface{}{}
	if err := json.Unmarshal(entry.FieldsV1.Raw, &fields); err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to parse FieldsV1 for managed fields entry %q (%s, apiVersion %q)",
			entry.Manager,
			entry.Operation,
			entry.APIVersion,
		)
	}
	if fields == nil {
		return nil, errors.Errorf(
			"managed fields entry %q (%s, apiVersion %q) has invalid null FieldsV1",
			entry.Manager,
			entry.Operation,
			entry.APIVersion,
		)
	}
	return fields, nil
}

func nestedFieldSet(fields map[string]interface{}, path ...string) (map[string]interface{}, bool, error) {
	current := fields
	for i, element := range path {
		value, found := current[element]
		if !found {
			return nil, false, nil
		}
		fieldSet, ok := value.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("managed field path %q is not a field set", path[:i+1])
		}
		current = fieldSet
	}
	return current, true, nil
}

func removeNestedFieldSet(fields map[string]interface{}, path ...string) (map[string]interface{}, bool, error) {
	fieldSet, found, err := nestedFieldSet(fields, path...)
	if err != nil || !found {
		return nil, found, err
	}
	removeNestedField(fields, path...)
	return fieldSet, true, nil
}

func removeNestedField(fields map[string]interface{}, path ...string) bool {
	if len(path) == 1 {
		delete(fields, path[0])
		return len(fields) == 0
	}
	child, ok := fields[path[0]].(map[string]interface{})
	if !ok {
		return false
	}
	if removeNestedField(child, path[1:]...) {
		delete(fields, path[0])
	}
	return len(fields) == 0
}

func mergeNestedFieldSet(fields map[string]interface{}, fieldSet map[string]interface{}, path ...string) error {
	current := fields
	for _, element := range path {
		value, found := current[element]
		if !found {
			next := map[string]interface{}{}
			current[element] = next
			current = next
			continue
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("managed field path %q is not a field set", element)
		}
		current = next
	}
	return mergeFieldSet(current, fieldSet)
}

func mergeFieldSet(destination map[string]interface{}, source map[string]interface{}) error {
	for key, sourceValue := range source {
		sourceFieldSet, ok := sourceValue.(map[string]interface{})
		if !ok {
			return fmt.Errorf("managed field %q is not a field set", key)
		}
		destinationValue, found := destination[key]
		if !found {
			destination[key] = runtime.DeepCopyJSONValue(sourceFieldSet)
			continue
		}
		destinationFieldSet, ok := destinationValue.(map[string]interface{})
		if !ok {
			return fmt.Errorf("managed field %q is not a field set", key)
		}
		if err := mergeFieldSet(destinationFieldSet, sourceFieldSet); err != nil {
			return err
		}
	}
	return nil
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

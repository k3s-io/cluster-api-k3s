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

package controllers

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/util/ssa"
)

func (r *KThreesControlPlaneReconciler) createRelatedObject(
	ctx context.Context,
	obj client.Object,
	gvk schema.GroupVersionKind,
	kcp *controlplanev1.KThreesControlPlane,
	cluster *clusterv1.Cluster,
) error {
	if err := ssa.Patch(ctx, r.Client, kcpManagerName, obj); err != nil {
		return errors.Wrapf(err, "failed to create %s", gvk.Kind)
	}
	if err := ssa.RemoveManagedFieldsForLabelsAndAnnotations(
		ctx,
		r.Client,
		r.apiReader,
		obj,
		kcpManagerName,
	); err != nil {
		return errors.Wrapf(err, "failed to split managedFields ownership for %s", gvk.Kind)
	}
	if err := r.updateExternalObject(ctx, obj, gvk, kcp, cluster); err != nil {
		return errors.Wrapf(err, "failed to establish metadata ownership for %s", gvk.Kind)
	}
	return nil
}

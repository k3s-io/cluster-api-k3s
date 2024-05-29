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
package v1beta1

import (
	"fmt"
	unsafe "unsafe"

	"k8s.io/apimachinery/pkg/conversion"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	ctrlconversion "sigs.k8s.io/controller-runtime/pkg/conversion"

	bootstrapv1beta1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta1"
	bootstrapv1beta2 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1beta2 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
)

func Convert_v1beta1_KThreesControlPlaneSpec_To_v1beta2_KThreesControlPlaneSpec(in *KThreesControlPlaneSpec, out *controlplanev1beta2.KThreesControlPlaneSpec, s conversion.Scope) error { //nolint: stylecheck
	out.Replicas = in.Replicas
	out.Version = in.Version
	if err := Convert_v1beta1_KThreesConfigSpec_To_v1beta2_KThreesConfigSpec(&in.KThreesConfigSpec, &out.KThreesConfigSpec, s); err != nil {
		return fmt.Errorf("converting KThreesConfigSpec field from v1beta1 to v1beta2: %w", err)
	}
	out.RolloutAfter = in.UpgradeAfter
	if err := Convert_v1beta1_KThreesControlPlaneMachineTemplate_To_v1beta2_KThreesControlPlaneMachineTemplate(&in.MachineTemplate, &out.MachineTemplate, s); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneMachineTemplate field from v1beta1 to v1beta2: %w", err)
	}
	out.MachineTemplate.NodeDrainTimeout = in.NodeDrainTimeout
	out.MachineTemplate.InfrastructureRef = in.InfrastructureTemplate
	out.RemediationStrategy = (*controlplanev1beta2.RemediationStrategy)(unsafe.Pointer(in.RemediationStrategy))
	return nil
}

func Convert_v1beta2_KThreesControlPlaneSpec_To_v1beta1_KThreesControlPlaneSpec(in *controlplanev1beta2.KThreesControlPlaneSpec, out *KThreesControlPlaneSpec, s conversion.Scope) error { //nolint: stylecheck
	out.Replicas = in.Replicas
	out.Version = in.Version
	out.InfrastructureTemplate = in.MachineTemplate.InfrastructureRef
	if err := Convert_v1beta2_KThreesConfigSpec_To_v1beta1_KThreesConfigSpec(&in.KThreesConfigSpec, &out.KThreesConfigSpec, s); err != nil {
		return fmt.Errorf("converting KThreesConfigSpec field from v1beta2 to v1beta1: %w", err)
	}
	out.UpgradeAfter = in.RolloutAfter
	out.NodeDrainTimeout = in.MachineTemplate.NodeDrainTimeout
	if err := Convert_v1beta2_KThreesControlPlaneMachineTemplate_To_v1beta1_KThreesControlPlaneMachineTemplate(&in.MachineTemplate, &out.MachineTemplate, s); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneMachineTemplate field from v1beta2 to v1beta1: %w", err)
	}
	out.RemediationStrategy = (*RemediationStrategy)(unsafe.Pointer(in.RemediationStrategy))
	return nil
}

func Convert_v1beta2_KThreesControlPlaneMachineTemplate_To_v1beta1_KThreesControlPlaneMachineTemplate(in *controlplanev1beta2.KThreesControlPlaneMachineTemplate, out *KThreesControlPlaneMachineTemplate, s conversion.Scope) error { //nolint: stylecheck
	out.ObjectMeta = in.ObjectMeta
	return nil
}

func Convert_v1beta1_KThreesConfigSpec_To_v1beta2_KThreesConfigSpec(in *bootstrapv1beta1.KThreesConfigSpec, out *bootstrapv1beta2.KThreesConfigSpec, s conversion.Scope) error { //nolint: stylecheck
	return bootstrapv1beta1.Convert_v1beta1_KThreesConfigSpec_To_v1beta2_KThreesConfigSpec(in, out, s)
}

func Convert_v1beta2_KThreesConfigSpec_To_v1beta1_KThreesConfigSpec(in *bootstrapv1beta2.KThreesConfigSpec, out *bootstrapv1beta1.KThreesConfigSpec, s conversion.Scope) error { //nolint: stylecheck
	return bootstrapv1beta1.Convert_v1beta2_KThreesConfigSpec_To_v1beta1_KThreesConfigSpec(in, out, s)
}

func Convert_v1beta2_KThreesControlPlaneStatus_To_v1beta1_KThreesControlPlaneStatus(in *controlplanev1beta2.KThreesControlPlaneStatus, out *KThreesControlPlaneStatus, s conversion.Scope) error { //nolint: stylecheck
	return autoConvert_v1beta2_KThreesControlPlaneStatus_To_v1beta1_KThreesControlPlaneStatus(in, out, s)
}

// ConvertTo converts the v1beta1 KThreesControlPlane receiver to a v1beta2 KThreesControlPlane.
func (in *KThreesControlPlane) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*controlplanev1beta2.KThreesControlPlane)
	if err := Convert_v1beta1_KThreesControlPlane_To_v1beta2_KThreesControlPlane(in, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlane v1beta1 to v1beta2: %w", err)
	}

	restored := &controlplanev1beta2.KThreesControlPlane{}
	if ok, err := utilconversion.UnmarshalData(in, restored); err != nil {
		return fmt.Errorf("unmarshalling stored conversion data: %w", err)
	} else if !ok {
		// No stored data.
		return nil
	}

	dst.Spec.KThreesConfigSpec.ServerConfig.CloudProviderName = restored.Spec.KThreesConfigSpec.ServerConfig.CloudProviderName
	dst.Spec.KThreesConfigSpec.ServerConfig.DeprecatedDisableExternalCloudProvider = restored.Spec.KThreesConfigSpec.ServerConfig.DeprecatedDisableExternalCloudProvider
	dst.Spec.KThreesConfigSpec.ServerConfig.DisableCloudController = restored.Spec.KThreesConfigSpec.ServerConfig.DisableCloudController
	dst.Spec.MachineTemplate.NodeVolumeDetachTimeout = restored.Spec.MachineTemplate.NodeVolumeDetachTimeout
	dst.Spec.MachineTemplate.NodeDeletionTimeout = restored.Spec.MachineTemplate.NodeDeletionTimeout
	dst.Status.Version = restored.Status.Version
	return nil
}

// ConvertFrom converts the v1beta1 KThreesControlPlane receiver from a v1beta2 KThreesControlPlane.
func (in *KThreesControlPlane) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*controlplanev1beta2.KThreesControlPlane)
	if err := Convert_v1beta2_KThreesControlPlane_To_v1beta1_KThreesControlPlane(src, in, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlane v1beta1 from v1beta2: %w", err)
	}

	if err := utilconversion.MarshalData(src, in); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta1 KThreesControlPlaneList receiver to a v1beta2 KThreesControlPlaneList.
func (in *KThreesControlPlaneList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*controlplanev1beta2.KThreesControlPlaneList)
	if err := Convert_v1beta1_KThreesControlPlaneList_To_v1beta2_KThreesControlPlaneList(in, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneList v1beta1 to v1beta2: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta1 KThreesControlPlaneList receiver from a v1beta2 KThreesControlPlaneList.
func (in *KThreesControlPlaneList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*controlplanev1beta2.KThreesControlPlaneList)
	if err := Convert_v1beta2_KThreesControlPlaneList_To_v1beta1_KThreesControlPlaneList(src, in, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneList v1beta1 from v1beta2: %w", err)
	}
	return nil
}

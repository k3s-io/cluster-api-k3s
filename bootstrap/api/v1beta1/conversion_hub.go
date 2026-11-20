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
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	ctrlconversion "sigs.k8s.io/controller-runtime/pkg/conversion"

	bootstrapv1beta2 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta3"
)

// HubDataAnnotation is the annotation used to store the data of the hub (v1beta3) object
// on v1beta1 objects, in order to avoid data loss in v1beta3 => v1beta1 => v1beta3 round trips.
//
// v1beta1 objects are converted to the hub through v1beta2. The standard conversion data annotation
// (utilconversion.DataAnnotation) stores the v1beta2 data, which is used to restore fields that
// do not exist in v1beta1 when converting to v1beta2; this annotation stores the v1beta3 data, which is used
// to restore fields that do not exist in v1beta2 when converting to v1beta3.
const HubDataAnnotation = "bootstrap.cluster.x-k8s.io/conversion-data-v1beta3"

// ConvertTo converts the v1beta1 KThreesConfig receiver to a v1beta3 KThreesConfig.
func (c *KThreesConfig) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1.KThreesConfig)
	hubData := popAnnotation(c, HubDataAnnotation)

	mid := &bootstrapv1beta2.KThreesConfig{}
	if err := c.convertToV1beta2(mid); err != nil {
		return err
	}
	prepareForHubConversion(mid, hubData)
	if err := mid.ConvertTo(dst); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta2 to v1beta3: %w", err)
	}
	cleanupConversionAnnotations(dst)
	return nil
}

// ConvertFrom converts the v1beta1 KThreesConfig receiver from a v1beta3 KThreesConfig.
func (c *KThreesConfig) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1.KThreesConfig)

	mid := &bootstrapv1beta2.KThreesConfig{}
	if err := mid.ConvertFrom(src); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta2 from v1beta3: %w", err)
	}
	hubData := popAnnotation(mid, utilconversion.DataAnnotation)
	if err := c.convertFromV1beta2(mid); err != nil {
		return err
	}
	setAnnotation(c, HubDataAnnotation, hubData)
	return nil
}

// ConvertTo converts the v1beta1 KThreesConfigList receiver to a v1beta3 KThreesConfigList.
func (c *KThreesConfigList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	mid := &bootstrapv1beta2.KThreesConfigList{}
	if err := c.convertToV1beta2(mid); err != nil {
		return err
	}
	return mid.ConvertTo(dstRaw)
}

// ConvertFrom converts the v1beta1 KThreesConfigList receiver from a v1beta3 KThreesConfigList.
func (c *KThreesConfigList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	mid := &bootstrapv1beta2.KThreesConfigList{}
	if err := mid.ConvertFrom(srcRaw); err != nil {
		return err
	}
	return c.convertFromV1beta2(mid)
}

// ConvertTo converts the v1beta1 KThreesConfigTemplate receiver to a v1beta3 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1.KThreesConfigTemplate)
	hubData := popAnnotation(r, HubDataAnnotation)

	mid := &bootstrapv1beta2.KThreesConfigTemplate{}
	if err := r.convertToV1beta2(mid); err != nil {
		return err
	}
	prepareForHubConversion(mid, hubData)
	if err := mid.ConvertTo(dst); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta2 to v1beta3: %w", err)
	}
	cleanupConversionAnnotations(dst)
	return nil
}

// ConvertFrom converts the v1beta1 KThreesConfigTemplate receiver from a v1beta3 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1.KThreesConfigTemplate)

	mid := &bootstrapv1beta2.KThreesConfigTemplate{}
	if err := mid.ConvertFrom(src); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta2 from v1beta3: %w", err)
	}
	hubData := popAnnotation(mid, utilconversion.DataAnnotation)
	if err := r.convertFromV1beta2(mid); err != nil {
		return err
	}
	setAnnotation(r, HubDataAnnotation, hubData)
	return nil
}

// ConvertTo converts the v1beta1 KThreesConfigTemplateList receiver to a v1beta3 KThreesConfigTemplateList.
func (r *KThreesConfigTemplateList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	mid := &bootstrapv1beta2.KThreesConfigTemplateList{}
	if err := r.convertToV1beta2(mid); err != nil {
		return err
	}
	return mid.ConvertTo(dstRaw)
}

// ConvertFrom converts the v1beta1 KThreesConfigTemplateList receiver from a v1beta3 KThreesConfigTemplateList.
func (r *KThreesConfigTemplateList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	mid := &bootstrapv1beta2.KThreesConfigTemplateList{}
	if err := mid.ConvertFrom(srcRaw); err != nil {
		return err
	}
	return r.convertFromV1beta2(mid)
}

// popAnnotation removes the annotation with the given key from obj and returns its value.
// NOTE: The annotations map is cloned before mutating it, because generated conversions copy
// ObjectMeta by value and thus objects on both sides of a conversion can share the same map.
func popAnnotation(obj metav1.Object, key string) string {
	annotations := obj.GetAnnotations()
	value, ok := annotations[key]
	if !ok {
		return ""
	}
	annotations = maps.Clone(annotations)
	delete(annotations, key)
	if len(annotations) == 0 {
		annotations = nil
	}
	obj.SetAnnotations(annotations)
	return value
}

// setAnnotation sets the annotation with the given key on obj, if value is not empty.
func setAnnotation(obj metav1.Object, key, value string) {
	if value == "" {
		return
	}
	annotations := maps.Clone(obj.GetAnnotations())
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	obj.SetAnnotations(annotations)
}

// prepareForHubConversion makes sure the intermediate v1beta2 object only carries the hub data
// as conversion data before it is converted to v1beta3.
func prepareForHubConversion(mid metav1.Object, hubData string) {
	popAnnotation(mid, utilconversion.DataAnnotation)
	setAnnotation(mid, utilconversion.DataAnnotation, hubData)
}

// cleanupConversionAnnotations makes sure no conversion data ends up on the hub object.
func cleanupConversionAnnotations(dst metav1.Object) {
	popAnnotation(dst, utilconversion.DataAnnotation)
	popAnnotation(dst, HubDataAnnotation)
}

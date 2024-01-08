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

import "sigs.k8s.io/controller-runtime/pkg/conversion"

// import (
// 	cabp3v1 "github.com/cluster-api-provider-k3s/cluster-api-k3s/bootstrap/api/v1beta2"
// 	"sigs.k8s.io/controller-runtime/pkg/conversion"
// )

// Hub marks KThreesConfig as a conversion hub.
func (r *KThreesConfig) ConvertTo(dstRaw conversion.Hub) error {
	//_ := dstRaw.(*cabp3v1.KThreesConfig)
	return nil
}

// // Hub marks KThreesConfigList as a conversion hub.
// func (*KThreesConfigList) Hub() {}

// // Hub marks KThreesConfigTemplate as a conversion hub.
// func (*KThreesConfigTemplate) Hub() {}

// // Hub marks KThreesConfigTemplateList as a conversion hub.
// func (*KThreesConfigTemplateList) Hub() {}

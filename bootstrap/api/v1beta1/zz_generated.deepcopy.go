//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	apiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *File) DeepCopyInto(out *File) {
	*out = *in
	if in.ContentFrom != nil {
		in, out := &in.ContentFrom, &out.ContentFrom
		*out = new(FileSource)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new File.
func (in *File) DeepCopy() *File {
	if in == nil {
		return nil
	}
	out := new(File)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FileSource) DeepCopyInto(out *FileSource) {
	*out = *in
	out.Secret = in.Secret
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FileSource.
func (in *FileSource) DeepCopy() *FileSource {
	if in == nil {
		return nil
	}
	out := new(FileSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesAgentConfig) DeepCopyInto(out *KThreesAgentConfig) {
	*out = *in
	if in.NodeLabels != nil {
		in, out := &in.NodeLabels, &out.NodeLabels
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodeTaints != nil {
		in, out := &in.NodeTaints, &out.NodeTaints
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.KubeletArgs != nil {
		in, out := &in.KubeletArgs, &out.KubeletArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.KubeProxyArgs != nil {
		in, out := &in.KubeProxyArgs, &out.KubeProxyArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesAgentConfig.
func (in *KThreesAgentConfig) DeepCopy() *KThreesAgentConfig {
	if in == nil {
		return nil
	}
	out := new(KThreesAgentConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesConfig) DeepCopyInto(out *KThreesConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesConfig.
func (in *KThreesConfig) DeepCopy() *KThreesConfig {
	if in == nil {
		return nil
	}
	out := new(KThreesConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KThreesConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesConfigList) DeepCopyInto(out *KThreesConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KThreesConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesConfigList.
func (in *KThreesConfigList) DeepCopy() *KThreesConfigList {
	if in == nil {
		return nil
	}
	out := new(KThreesConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KThreesConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesConfigSpec) DeepCopyInto(out *KThreesConfigSpec) {
	*out = *in
	if in.Files != nil {
		in, out := &in.Files, &out.Files
		*out = make([]File, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.PreK3sCommands != nil {
		in, out := &in.PreK3sCommands, &out.PreK3sCommands
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PostK3sCommands != nil {
		in, out := &in.PostK3sCommands, &out.PostK3sCommands
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.AgentConfig.DeepCopyInto(&out.AgentConfig)
	in.ServerConfig.DeepCopyInto(&out.ServerConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesConfigSpec.
func (in *KThreesConfigSpec) DeepCopy() *KThreesConfigSpec {
	if in == nil {
		return nil
	}
	out := new(KThreesConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesConfigStatus) DeepCopyInto(out *KThreesConfigStatus) {
	*out = *in
	if in.BootstrapData != nil {
		in, out := &in.BootstrapData, &out.BootstrapData
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	if in.DataSecretName != nil {
		in, out := &in.DataSecretName, &out.DataSecretName
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(apiv1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesConfigStatus.
func (in *KThreesConfigStatus) DeepCopy() *KThreesConfigStatus {
	if in == nil {
		return nil
	}
	out := new(KThreesConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesConfigTemplate) DeepCopyInto(out *KThreesConfigTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesConfigTemplate.
func (in *KThreesConfigTemplate) DeepCopy() *KThreesConfigTemplate {
	if in == nil {
		return nil
	}
	out := new(KThreesConfigTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KThreesConfigTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesConfigTemplateList) DeepCopyInto(out *KThreesConfigTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KThreesConfigTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesConfigTemplateList.
func (in *KThreesConfigTemplateList) DeepCopy() *KThreesConfigTemplateList {
	if in == nil {
		return nil
	}
	out := new(KThreesConfigTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KThreesConfigTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesConfigTemplateResource) DeepCopyInto(out *KThreesConfigTemplateResource) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesConfigTemplateResource.
func (in *KThreesConfigTemplateResource) DeepCopy() *KThreesConfigTemplateResource {
	if in == nil {
		return nil
	}
	out := new(KThreesConfigTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesConfigTemplateSpec) DeepCopyInto(out *KThreesConfigTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesConfigTemplateSpec.
func (in *KThreesConfigTemplateSpec) DeepCopy() *KThreesConfigTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(KThreesConfigTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KThreesServerConfig) DeepCopyInto(out *KThreesServerConfig) {
	*out = *in
	if in.KubeAPIServerArgs != nil {
		in, out := &in.KubeAPIServerArgs, &out.KubeAPIServerArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.KubeControllerManagerArgs != nil {
		in, out := &in.KubeControllerManagerArgs, &out.KubeControllerManagerArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.KubeSchedulerArgs != nil {
		in, out := &in.KubeSchedulerArgs, &out.KubeSchedulerArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.TLSSan != nil {
		in, out := &in.TLSSan, &out.TLSSan
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DisableComponents != nil {
		in, out := &in.DisableComponents, &out.DisableComponents
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KThreesServerConfig.
func (in *KThreesServerConfig) DeepCopy() *KThreesServerConfig {
	if in == nil {
		return nil
	}
	out := new(KThreesServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretFileSource) DeepCopyInto(out *SecretFileSource) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretFileSource.
func (in *SecretFileSource) DeepCopy() *SecretFileSource {
	if in == nil {
		return nil
	}
	out := new(SecretFileSource)
	in.DeepCopyInto(out)
	return out
}

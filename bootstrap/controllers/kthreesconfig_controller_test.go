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

package controllers

import (
	"testing"

	. "github.com/onsi/gomega"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
)

func TestKThreesConfigReconciler_ResolveEtcdProxyFile(t *testing.T) {
	g := NewWithT(t)
	// If EtcdProxyImage is set, it should override the system default registry
	config := &bootstrapv1.KThreesConfig{
		Spec: bootstrapv1.KThreesConfigSpec{
			ServerConfig: bootstrapv1.KThreesServerConfig{
				EtcdProxyImage:        "etcd-proxy-image",
				SystemDefaultRegistry: "system-default-registry",
			},
		},
	}
	r := &KThreesConfigReconciler{}
	etcdProxyFile, err := r.resolveEtcdProxyFile(config)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(etcdProxyFile.Content).To(ContainSubstring("etcd-proxy-image"), "generated etcd proxy image should contain EtcdProxyImage")
	g.Expect(etcdProxyFile.Content).ToNot(ContainSubstring("system-default-registry"), "system-default-registry should be overwritten by EtcdProxyImage")

	// If EtcdProxyImage is not set, the system default registry should be used
	config2 := &bootstrapv1.KThreesConfig{
		Spec: bootstrapv1.KThreesConfigSpec{
			ServerConfig: bootstrapv1.KThreesServerConfig{
				SystemDefaultRegistry: "system-default-registry2",
			},
		},
	}
	etcdProxyFile, err = r.resolveEtcdProxyFile(config2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(etcdProxyFile.Content).To(ContainSubstring("system-default-registry2/"), "generated etcd proxy image should be prefixed with SystemDefaultRegistry")
}

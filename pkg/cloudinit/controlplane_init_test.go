/*
Copyright 2019 The Kubernetes Authors.

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

package cloudinit

import (
	"testing"

	. "github.com/onsi/gomega"
	infrav1 "github.com/zawachte-msft/cluster-api-k3s/bootstrap/api/v1alpha3"
	"github.com/zawachte-msft/cluster-api-k3s/pkg/secret"
)

func TestControlPlaneInit(t *testing.T) {

	g := NewWithT(t)

	cpinput := &ControlPlaneInput{
		BaseUserData: BaseUserData{
			PreK3sCommands:  nil,
			PostK3sCommands: nil,
			AdditionalFiles: []infrav1.File{
				{
					Path:     "/tmp/my-path",
					Encoding: infrav1.Base64,
					Content:  "aGk=",
				},
				{
					Path:    "/tmp/my-other-path",
					Content: "hi",
				},
			},
		},
		Certificates: secret.Certificates{},
	}

	out, err := NewInitControlPlane(cpinput)
	g.Expect(err).NotTo(HaveOccurred())
	t.Log(string(out))
}

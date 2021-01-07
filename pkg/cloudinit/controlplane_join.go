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

const (
	controlPlaneCloudJoin = `{{.Header}}
{{template "files" .WriteFiles}}
runcmd:
{{- template "commands" .PreK3sCommands }}
  - 'curl -sfL https://get.k3s.io | sh -s - server'
{{- template "commands" .PostK3sCommands }}
`
)

// NewInitControlPlane returns the user data string to be used on a controlplane instance.
func NewJoinControlPlane(input *ControlPlaneInput) ([]byte, error) {
	input.Header = cloudConfigHeader
	input.WriteFiles = append(input.WriteFiles, input.AdditionalFiles...)
	input.WriteFiles = append(input.WriteFiles, input.ConfigFile)

	userData, err := generate("JoinControlplane", controlPlaneCloudJoin, input)
	if err != nil {
		return nil, err
	}

	return userData, nil
}

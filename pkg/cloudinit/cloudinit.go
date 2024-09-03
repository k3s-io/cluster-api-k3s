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
	"bytes"
	"fmt"
	"strings"
	"text/template"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
)

var (
	defaultTemplateFuncMap = template.FuncMap{
		"Indent": templateYAMLIndent,
	}
)

func templateYAMLIndent(i int, input string) string {
	split := strings.Split(input, "\n")
	ident := "\n" + strings.Repeat(" ", i)
	return strings.Repeat(" ", i) + strings.Join(split, ident)
}

const (
	k3sScriptName        = "/usr/local/bin/k3s"
	k3sScriptOwner       = "root"
	k3sScriptPermissions = "0755"
	cloudConfigHeader    = `## template: jinja
#cloud-config
`

	filesTemplate = `{{ define "files" -}}
write_files:{{ range . }}
-   path: {{.Path}}
    {{ if ne .Encoding "" -}}
    encoding: "{{.Encoding}}"
    {{ end -}}
    {{ if ne .Owner "" -}}
    owner: {{.Owner}}
    {{ end -}}
    {{ if ne .Permissions "" -}}
    permissions: '{{.Permissions}}'
    {{ end -}}
    content: |
{{.Content | Indent 6}}
{{- end -}}
{{- end -}}
`

	commandsTemplate = `{{- define "commands" -}}
{{ range . }}
  - {{printf "%q" .}}
{{- end -}}
{{- end -}}
`
	sentinelFileCommand               = "mkdir -p /run/cluster-api && echo success > /run/cluster-api/bootstrap-success.complete"
	defaultAirGappedInstallScriptPath = "/opt/install.sh"
)

// BaseUserData is shared across all the various types of files written to disk.
type BaseUserData struct {
	Header                     string
	PreK3sCommands             []string
	PostK3sCommands            []string
	AdditionalFiles            []bootstrapv1.File
	WriteFiles                 []bootstrapv1.File
	ConfigFile                 bootstrapv1.File
	K3sVersion                 string
	AirGapped                  bool
	AirGappedInstallScriptPath string
	SentinelFileCommand        string
}

func (input *BaseUserData) prepare() {
	input.Header = cloudConfigHeader
	input.WriteFiles = append(input.WriteFiles, input.AdditionalFiles...)
	input.WriteFiles = append(input.WriteFiles, input.ConfigFile)
	if input.AirGappedInstallScriptPath == "" {
		input.AirGappedInstallScriptPath = defaultAirGappedInstallScriptPath
	}

	input.SentinelFileCommand = sentinelFileCommand
}

func generate(kind string, tpl string, data interface{}) ([]byte, error) {
	tm := template.New(kind).Funcs(defaultTemplateFuncMap)
	if _, err := tm.Parse(filesTemplate); err != nil {
		return nil, fmt.Errorf("failed to parse files template: %w", err)
	}

	if _, err := tm.Parse(commandsTemplate); err != nil {
		return nil, fmt.Errorf("failed to parse commands template: %w", err)
	}

	t, err := tm.Parse(tpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s template: %w", kind, err)
	}

	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("failed to generate %s template: %w", kind, err)
	}

	return out.Bytes(), nil
}

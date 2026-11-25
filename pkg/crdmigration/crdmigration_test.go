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

package crdmigration

import (
	"flag"
	"io"
	"slices"
	"testing"
)

func TestAddSkipPhasesFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr bool
	}{
		{
			name: "flag not set",
			args: nil,
			want: nil,
		},
		{
			name: "single phase",
			args: []string{"--skip-crd-migration-phases=StorageVersionMigration"},
			want: []string{"StorageVersionMigration"},
		},
		{
			name: "comma separated phases with spaces",
			args: []string{"--skip-crd-migration-phases=StorageVersionMigration, CleanupManagedFields"},
			want: []string{"StorageVersionMigration", "CleanupManagedFields"},
		},
		{
			name: "flag set multiple times",
			args: []string{
				"--skip-crd-migration-phases=StorageVersionMigration",
				"--skip-crd-migration-phases=CleanupManagedFields",
			},
			want: []string{"StorageVersionMigration", "CleanupManagedFields"},
		},
		{
			name:    "invalid phase",
			args:    []string{"--skip-crd-migration-phases=Foo"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			var got []string
			AddSkipPhasesFlag(fs, &got)

			err := fs.Parse(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got phases %v, want %v", got, tt.want)
			}
		})
	}
}

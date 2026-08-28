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

// Package capi contains the minimal Cluster API v1.12.9 internal helper
// closure required to implement Runtime SDK in-place updates in an external
// control-plane provider.
//
// The copied Cluster API v1.12.9 source paths are:
//   - internal/util/client/client.go
//   - internal/util/client/metrics.go
//   - internal/util/compare/equal.go
//   - internal/controllers/extensionconfig/extensionconfig_controller.go
//   - internal/controllers/extensionconfig/index.go
//   - internal/controllers/extensionconfig/warmup.go
//   - internal/hooks/tracking.go
//   - internal/util/inplace/inplace.go
//   - internal/util/patch/patch.go
//   - internal/runtime/registry/registry.go
//   - internal/runtime/client/client.go
//   - internal/runtime/metrics/metrics.go
//
// Source paths and version are documented in this package so the copied code
// can be compared with upstream and removed if public equivalents become
// available.
//
// The patch helper deliberately diverges from Cluster API v1.12.9 and CAPRKE2
// by using fixed diagnostics for patch parsing, application, conversion, and
// panic recovery. Those logs and returned errors exclude patch and object
// bodies and underlying error text because Runtime Extension patches can
// contain secrets. Successful verbose logs include only the recognized patch
// type and, for decoded JSON patches, the operation count.
package capi

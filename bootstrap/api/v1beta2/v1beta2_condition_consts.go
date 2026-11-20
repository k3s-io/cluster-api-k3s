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

package v1beta2

import clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

// Conditions that will be used for the KThreesConfig object in status.v1beta2.conditions.
// NOTE: These conditions follow the Cluster API v1beta2 conventions (metav1.Condition), and they will
// become the main conditions when moving to the next API version.

// KThreesConfig's Ready condition and corresponding reasons.
const (
	// KThreesConfigReadyV1Beta2Condition is true if the KThreesConfig is not deleted,
	// and both DataSecretAvailable, CertificatesAvailable conditions are true.
	KThreesConfigReadyV1Beta2Condition = clusterv1.ReadyCondition

	// KThreesConfigReadyV1Beta2Reason surfaces when the KThreesConfig is ready.
	KThreesConfigReadyV1Beta2Reason = clusterv1.ReadyReason

	// KThreesConfigNotReadyV1Beta2Reason surfaces when the KThreesConfig is not ready.
	KThreesConfigNotReadyV1Beta2Reason = clusterv1.NotReadyReason

	// KThreesConfigReadyUnknownV1Beta2Reason surfaces when KThreesConfig readiness is unknown.
	KThreesConfigReadyUnknownV1Beta2Reason = clusterv1.ReadyUnknownReason
)

// KThreesConfig's CertificatesAvailable condition and corresponding reasons.
const (
	// KThreesConfigCertificatesAvailableV1Beta2Condition documents that cluster certificates required
	// for generating the bootstrap data secret are available.
	KThreesConfigCertificatesAvailableV1Beta2Condition = "CertificatesAvailable"

	// KThreesConfigCertificatesAvailableV1Beta2Reason surfaces when certificates required for machine bootstrap are available.
	KThreesConfigCertificatesAvailableV1Beta2Reason = clusterv1.AvailableReason

	// KThreesConfigCertificatesAvailableInternalErrorV1Beta2Reason surfaces unexpected failures when reading or
	// generating certificates required for machine bootstrap.
	KThreesConfigCertificatesAvailableInternalErrorV1Beta2Reason = clusterv1.InternalErrorReason
)

// KThreesConfig's DataSecretAvailable condition and corresponding reasons.
const (
	// KThreesConfigDataSecretAvailableV1Beta2Condition is true if the bootstrap secret is available.
	KThreesConfigDataSecretAvailableV1Beta2Condition = "DataSecretAvailable"

	// KThreesConfigDataSecretAvailableV1Beta2Reason surfaces when the bootstrap secret is available.
	KThreesConfigDataSecretAvailableV1Beta2Reason = clusterv1.AvailableReason

	// KThreesConfigDataSecretNotAvailableV1Beta2Reason surfaces when the bootstrap secret is not available.
	KThreesConfigDataSecretNotAvailableV1Beta2Reason = clusterv1.NotAvailableReason
)

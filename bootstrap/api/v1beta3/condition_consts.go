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

package v1beta3

import clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

// KThreesConfig's Ready condition and corresponding reasons.
const (
	// KThreesConfigReadyCondition is true if the KThreesConfig is not deleted,
	// and both DataSecretAvailable, CertificatesAvailable conditions are true.
	KThreesConfigReadyCondition = clusterv1.ReadyCondition

	// KThreesConfigReadyReason surfaces when the KThreesConfig is ready.
	KThreesConfigReadyReason = clusterv1.ReadyReason

	// KThreesConfigNotReadyReason surfaces when the KThreesConfig is not ready.
	KThreesConfigNotReadyReason = clusterv1.NotReadyReason

	// KThreesConfigReadyUnknownReason surfaces when KThreesConfig readiness is unknown.
	KThreesConfigReadyUnknownReason = clusterv1.ReadyUnknownReason
)

// KThreesConfig's CertificatesAvailable condition and corresponding reasons.
const (
	// KThreesConfigCertificatesAvailableCondition documents that cluster certificates required
	// for generating the bootstrap data secret are available.
	KThreesConfigCertificatesAvailableCondition = "CertificatesAvailable"

	// KThreesConfigCertificatesAvailableReason surfaces when certificates required for machine bootstrap are available.
	KThreesConfigCertificatesAvailableReason = clusterv1.AvailableReason

	// KThreesConfigCertificatesAvailableInternalErrorReason surfaces unexpected failures when reading or
	// generating certificates required for machine bootstrap.
	KThreesConfigCertificatesAvailableInternalErrorReason = clusterv1.InternalErrorReason
)

// KThreesConfig's DataSecretAvailable condition and corresponding reasons.
const (
	// KThreesConfigDataSecretAvailableCondition is true if the bootstrap secret is available.
	KThreesConfigDataSecretAvailableCondition = "DataSecretAvailable"

	// KThreesConfigDataSecretAvailableReason surfaces when the bootstrap secret is available.
	KThreesConfigDataSecretAvailableReason = clusterv1.AvailableReason

	// KThreesConfigDataSecretNotAvailableReason surfaces when the bootstrap secret is not available.
	KThreesConfigDataSecretNotAvailableReason = clusterv1.NotAvailableReason
)

// Conditions and condition Reasons for the KThreesConfig object, stored in status.deprecated.v1beta1.conditions.
//
// Deprecated: These conditions are only kept for v1beta1 compatibility and will be removed together with
// status.deprecated.v1beta1.

const (
	// DataSecretAvailableV1Beta1Condition documents the status of the bootstrap secret generation process.
	//
	// NOTE: When the DataSecret generation starts the process completes immediately and within the
	// same reconciliation, so the user will always see a transition from Wait to Generated without having
	// evidence that BootstrapSecret generation is started/in progress.
	DataSecretAvailableV1Beta1Condition clusterv1.ConditionType = "DataSecretAvailable"

	// WaitingForClusterInfrastructureV1Beta1Reason (Severity=Info) document a bootstrap secret generation process
	// waiting for the cluster infrastructure to be ready.
	//
	// NOTE: Having the cluster infrastructure ready is a pre-condition for starting to create machines;
	// the KThreesConfig controller ensure this pre-condition is satisfied.
	WaitingForClusterInfrastructureV1Beta1Reason = "WaitingForClusterInfrastructure"

	// DataSecretGenerationFailedV1Beta1Reason (Severity=Warning) documents a KThreesConfig controller detecting
	// an error while generating a data secret; those kind of errors are usually due to misconfigurations
	// and user intervention is required to get them fixed.
	DataSecretGenerationFailedV1Beta1Reason = "DataSecretGenerationFailed"
)

const (
	// CertificatesAvailableV1Beta1Condition documents that cluster certificates are available.
	//
	// NOTE: Cluster certificates are generated only for the KThreesConfig object linked to the initial control plane
	// machine, if the cluster is not using a control plane ref object, if the certificates are not provided
	// by the users.
	// IMPORTANT: This condition won't be re-created after clusterctl move.
	CertificatesAvailableV1Beta1Condition clusterv1.ConditionType = "CertificatesAvailable"

	// CertificatesGenerationFailedV1Beta1Reason (Severity=Warning) documents a KThreesConfig controller detecting
	// an error while generating certificates; those kind of errors are usually temporary and the controller
	// automatically recover from them.
	CertificatesGenerationFailedV1Beta1Reason = "CertificatesGenerationFailed"

	// CertificatesCorruptedV1Beta1Reason (Severity=Error) documents a KThreesConfig controller detecting
	// an error while retrieving certificates for a joining node.
	CertificatesCorruptedV1Beta1Reason = "CertificatesCorrupted"
)

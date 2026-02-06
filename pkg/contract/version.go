package contract

// NOTE: this is taken from the internal package:
// https://github.com/kubernetes-sigs/cluster-api/blob/main/internal/contract/version.go

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sversion "k8s.io/apimachinery/pkg/version"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/contract"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Version is the contract version supported by this provider.
// Note: Each provider version supports one contract version, and by convention the
// contract version matches the current API version.
var Version = "v1beta2"

// GetCompatibleVersions return the list of contract version compatible with a given contract version.
// NOTE: A contract version might be temporarily compatible with older contract versions e.g. to allow
// users time to transition to the new API.
// NOTE: The return value must include also the contract version received in input.
func GetCompatibleVersions(contract string) sets.Set[string] {
	compatibleContracts := sets.New(contract)
	// v1beta2 contract is temporarily be compatible with v1beta1 (until v1beta1 is EOL).
	if contract == "v1beta2" {
		compatibleContracts.Insert("v1beta1")
	}
	return compatibleContracts
}

// GetAPIVersion gets the latest compatible apiVersion from a CRD based on the current contract Version.
func GetAPIVersion(ctx context.Context, c client.Reader, gk schema.GroupKind) (string, error) {
	crdMetadata, err := GetGKMetadata(ctx, c, gk)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get apiVersion for kind %s", gk.Kind)
	}

	_, version, err := GetLatestContractAndAPIVersionFromContract(crdMetadata, Version)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get apiVersion for kind %s", gk.Kind)
	}

	return schema.GroupVersion{
		Group:   gk.Group,
		Version: version,
	}.String(), nil
}

// GetGKMetadata retrieves a CustomResourceDefinition metadata from the API server using partial object metadata.
//
// This function is greatly more efficient than GetCRDWithContract and should be preferred in most cases.
func GetGKMetadata(ctx context.Context, c client.Reader, gk schema.GroupKind) (*metav1.PartialObjectMetadata, error) {
	meta := &metav1.PartialObjectMetadata{}
	meta.SetName(contract.CalculateCRDName(gk.Group, gk.Kind))
	meta.SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	if err := c.Get(ctx, client.ObjectKeyFromObject(meta), meta); err != nil {
		return meta, errors.Wrap(err, "failed to get CustomResourceDefinition metadata")
	}
	return meta, nil
}

// GetLatestContractAndAPIVersionFromContract gets the latest compatible contract version and apiVersion from
// a CRD based on the passed in currentContractVersion.
func GetLatestContractAndAPIVersionFromContract(
	metadata metav1.Object,
	currentContractVersion string,
) (string, string, error) {
	if currentContractVersion == "" {
		return "", "", errors.Errorf("current contract version cannot be empty")
	}

	labels := metadata.GetLabels()

	sortedCompatibleContractVersions := kubeAwareAPIVersions(GetCompatibleVersions(currentContractVersion).UnsortedList())
	sort.Sort(sort.Reverse(sortedCompatibleContractVersions))

	for _, contractVersion := range sortedCompatibleContractVersions {
		contractGroupVersion := fmt.Sprintf("%s/%s", clusterv1.GroupVersion.Group, contractVersion)

		// If there is no label, return early without changing the reference.
		supportedVersions, ok := labels[contractGroupVersion]
		if !ok || supportedVersions == "" {
			continue
		}

		// Pick the latest version in the slice and validate it.
		kubeVersions := kubeAwareAPIVersions(strings.Split(supportedVersions, "_"))
		sort.Sort(kubeVersions)
		return contractVersion, kubeVersions[len(kubeVersions)-1], nil
	}

	return "", "", errors.Errorf("cannot find any versions matching contract versions %q for CRD %v as contract version label(s) are either missing or empty (see https://cluster-api.sigs.k8s.io/developer/providers/contracts/overview.html#api-version-labels)", sortedCompatibleContractVersions, metadata.GetName()) //nolint:lll
}

// kubeAwareAPIVersions is a sortable slice of kube-like version strings.
//
// Kube-like version strings are starting with a v, followed by a major version,
// optional "alpha" or "beta" strings followed by a minor version (e.g. v1, v2beta1).
// Versions will be sorted based on GA/alpha/beta first and then major and minor
// versions. e.g. v2, v1, v1beta2, v1beta1, v1alpha1.
type kubeAwareAPIVersions []string

func (k kubeAwareAPIVersions) Len() int      { return len(k) }
func (k kubeAwareAPIVersions) Swap(i, j int) { k[i], k[j] = k[j], k[i] }
func (k kubeAwareAPIVersions) Less(i, j int) bool {
	return k8sversion.CompareKubeAwareVersionStrings(k[i], k[j]) < 0
}

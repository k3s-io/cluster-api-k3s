package errors

type KThreesControlPlaneStatusError string

const (
	// InvalidConfigurationKThreesControlPlaneError indicates that the KThrees control plane
	// configuration is invalid.
	InvalidConfigurationKThreesControlPlaneError KThreesControlPlaneStatusError = "InvalidConfiguration"

	// UnsupportedChangeKThreesControlPlaneError indicates that the KThrees control plane
	// spec has been updated in an unsupported way that cannot be
	// reconciled.
	UnsupportedChangeKThreesControlPlaneError KThreesControlPlaneStatusError = "UnsupportedChange"

	// CreateKThreesControlPlaneError indicates that an error was encountered
	// when trying to create the KThrees control plane.
	CreateKThreesControlPlaneError KThreesControlPlaneStatusError = "CreateError"

	// UpdateKThreesControlPlaneError indicates that an error was encountered
	// when trying to update the KThrees control plane.
	UpdateKThreesControlPlaneError KThreesControlPlaneStatusError = "UpdateError"

	// DeleteKThreesControlPlaneError indicates that an error was encountered
	// when trying to delete the KThrees control plane.
	DeleteKThreesControlPlaneError KThreesControlPlaneStatusError = "DeleteError"
)

package auth

const (
	// AdminRoleClaim is the Zitadel project role claim key used
	// to identify admins in the ID token claims map.
	AdminRoleClaim = "urn:zitadel:iam:org:project:roles"

	// AdminRole is the role name that grants admin access.
	AdminRole = "admin"
)
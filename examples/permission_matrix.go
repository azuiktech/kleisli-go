package examples

import (
	"github.com/azuiktech/kleisli-go/stream"
)

// ============================================================================
// PROBLEM: Permission Bitmask Matrix & Access Control Filter (Security / RBAC)
// ============================================================================
// Mini Problem Definition:
// Given a list of user accounts with permission bitmasks (e.g. Read=1, Write=2, Admin=4):
// 1. Evaluate if a user possesses all required permission flags.
// 2. Filter qualified accounts and extract their user IDs using functional streams.

const (
	PermissionRead  uint32 = 1 << 0
	PermissionWrite uint32 = 1 << 1
	PermissionAdmin uint32 = 1 << 2
)

type UserAccount struct {
	ID          string
	Department  string
	Permissions uint32
}

// FilterAuthorizedUsers returns IDs of users having all required permissions.
func FilterAuthorizedUsers(accounts []UserAccount, requiredPermissions uint32) []string {
	return stream.Of(accounts).
		Filter(func(u UserAccount) bool {
			// Check bitmask: user must have all required permission bits set
			return (u.Permissions & requiredPermissions) == requiredPermissions
		}).
		Map(func(u UserAccount) string {
			return u.ID
		}).
		Collect()
}

// HasAllPermissions checks if all users in a department satisfy a required permission set.
func HasAllPermissions(accounts []UserAccount, department string, requiredPermissions uint32) bool {
	deptUsers := stream.Of(accounts).
		Filter(func(u UserAccount) bool { return u.Department == department }).
		Collect()

	if len(deptUsers) == 0 {
		return false
	}

	return stream.Of(deptUsers).All(func(u UserAccount) bool {
		return (u.Permissions & requiredPermissions) == requiredPermissions
	})
}

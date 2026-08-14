package examples

import (
	"github.com/azuiktech/kleisli-go/value"
)

// ============================================================================
// PROBLEM: Struct Patch Mutation & Default Pointer Allocator (Data Mutation)
// ============================================================================
// Mini Problem Definition:
// 1. Construct partial patch objects with optional pointer fields (`*string`, `*int`, `*bool`)
//    without creating temporary local variables for addresses.
// 2. Apply patch fields onto an existing entity safely using `value.Deref` and `value.Ptr`.

type UserEntity struct {
	ID          string
	DisplayName string
	Role        string
	Active      bool
}

type UserPatch struct {
	DisplayName *string
	Role        *string
	Active      *bool
}

// BuildDefaultPatch creates a patch object inline using value.Ptr.
func BuildDefaultPatch(role string) UserPatch {
	return UserPatch{
		Role:   value.Ptr(role),
		Active: value.Ptr(true),
	}
}

// ApplyUserPatch merges a partial patch into an existing UserEntity safely.
func ApplyUserPatch(entity UserEntity, patch UserPatch) UserEntity {
	return UserEntity{
		ID:          entity.ID,
		DisplayName: value.Deref(patch.DisplayName, entity.DisplayName),
		Role:        value.Deref(patch.Role, entity.Role),
		Active:      value.Deref(patch.Active, entity.Active),
	}
}

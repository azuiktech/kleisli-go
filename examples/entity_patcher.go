package examples

import (
	"github.com/azuiktech/kleisli-go/fn"
)

// ============================================================================
// PROBLEM: Struct Patch Mutation & Default Pointer Allocator (Data Mutation)
// ============================================================================
// Mini Problem Definition:
// 1. Construct partial patch objects with optional pointer fields (`*string`, `*int`, `*bool`)
//    without creating temporary local variables for addresses.
// 2. Apply patch fields onto an existing entity safely using `fn.Deref` and `fn.Ptr`.

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

// BuildDefaultPatch creates a patch object inline using fn.Ptr.
func BuildDefaultPatch(role string) UserPatch {
	return UserPatch{
		Role:   fn.Ptr(role),
		Active: fn.Ptr(true),
	}
}

// ApplyUserPatch merges a partial patch into an existing UserEntity safely.
func ApplyUserPatch(entity UserEntity, patch UserPatch) UserEntity {
	return UserEntity{
		ID:          entity.ID,
		DisplayName: fn.Deref(patch.DisplayName, entity.DisplayName),
		Role:        fn.Deref(patch.Role, entity.Role),
		Active:      fn.Deref(patch.Active, entity.Active),
	}
}

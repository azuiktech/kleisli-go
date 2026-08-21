package examples

import (
	"github.com/azuiktech/kleisli-go/adt"
)



// ============================================================================
// PROBLEM: Monadic Error Guard & Safe Template Error Access (System Safety)
// ============================================================================
// Mini Problem Definition:
// In HTML/text template rendering or test assertions, calling `.Error()` on a Result[T]
// returns `""` or panics when the result is OK.
//
// Solution:
// 1. Use `Result.IsErr()` and `Result.MustErr()` to safely inspect errors in templates.
// 2. Use `value.Cond` to format template error strings symmetrically.

type ViewState struct {
	HasError bool
	ErrMsg   string
	Data     string
}

// RenderViewState transforms a fallible Result[string] into a safe ViewState for templates.
func RenderViewState(res adt.Result[string]) ViewState {
	if res.IsErr() {
		return ViewState{
			HasError: true,
			ErrMsg:   res.MustErr().Error(),
		}
	}
	return ViewState{
		Data: res.Expect("data present"),
	}
}


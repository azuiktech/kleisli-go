package examples

import (
	"fmt"
	"strings"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/stream"
)

// ============================================================================
// PROBLEM: LLM Prompt Template Loader & Schema Substitution (LLM Infrastructure)
// ============================================================================
// Mini Problem Definition:
// 1. Thread-safely load and compile system instruction templates on demand without live LLM calls.
// 2. Safely perform placeholder variable substitution (e.g. `{role}`, `{domain}`).
// 3. Fall back to default instruction strings if custom prompts are missing or empty using `fn.Cond`.

type PromptTemplate struct {
	Slug         string
	RawBody      string
	RequiredVars []string
}

// Global deferred initialization of core prompt instruction templates using adt.Defer
var DefaultSystemInstructions = adt.Defer(func() adt.Result[PromptTemplate] {
	return adt.OK(PromptTemplate{
		Slug:         "system_base_v1",
		RawBody:      "You are an AI Assistant for {domain}. Role: {role}.",
		RequiredVars: []string{"{domain}", "{role}"},
	})
})

// BuildInstructionPrompt compiles and substitutes template variables safely.
func BuildInstructionPrompt(customBody adt.Option[string], domain, role string) adt.Result[string] {
	// Fall back to default template if custom prompt is missing
	defaultPrompt, err := DefaultSystemInstructions.Get().Unwrap()
	if err != nil {
		return adt.Err[string](fmt.Errorf("loading default template: %w", err))
	}

	body := customBody.OrElse(defaultPrompt.RawBody)

	// Validate required placeholders are present
	if missing := stream.Of(defaultPrompt.RequiredVars).First(func(v string) bool {
		return !strings.Contains(body, v)
	}); missing.IsSome() {
		return adt.Err[string](fmt.Errorf("template %q missing required variable %q", defaultPrompt.Slug, missing.MustGet()))
	}

	// Substitute variables
	substituted := strings.ReplaceAll(body, "{domain}", domain)
	substituted = strings.ReplaceAll(substituted, "{role}", role)

	return adt.OK(substituted)
}

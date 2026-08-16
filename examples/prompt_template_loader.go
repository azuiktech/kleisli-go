package examples

import (
	"fmt"
	"strings"

	"github.com/azuiktech/kleisli-go/lazy"
	"github.com/azuiktech/kleisli-go/option"
	"github.com/azuiktech/kleisli-go/result"
	"github.com/azuiktech/kleisli-go/stream"
)



// ============================================================================
// PROBLEM: LLM Prompt Template Loader & Schema Substitution (LLM Infrastructure)
// ============================================================================
// Mini Problem Definition:
// 1. Thread-safely load and compile system instruction templates on demand without live LLM calls.
// 2. Safely perform placeholder variable substitution (e.g. `{role}`, `{domain}`).
// 3. Fall back to default instruction strings if custom prompts are missing or empty using `value.Cond`.

type PromptTemplate struct {
	Slug         string
	RawBody      string
	RequiredVars []string
}

// Global deferred initialization of core prompt instruction templates using lazy.New
var DefaultSystemInstructions = lazy.New(func() result.Result[PromptTemplate] {
	return result.OK(PromptTemplate{
		Slug:         "system_base_v1",
		RawBody:      "You are an AI Assistant for {domain}. Role: {role}.",
		RequiredVars: []string{"{domain}", "{role}"},
	})
})

// BuildInstructionPrompt compiles and substitutes template variables safely.
func BuildInstructionPrompt(customBody option.Option[string], domain, role string) result.Result[string] {
	// Fall back to default template if custom prompt is missing using value.Cond
	defaultPrompt, err := DefaultSystemInstructions.Get().Unwrap()
	if err != nil {
		return result.Err[string](fmt.Errorf("loading default template: %w", err))
	}

	body := customBody.OrElse(defaultPrompt.RawBody)



	// Validate required placeholders are present
	if missing := stream.Of(defaultPrompt.RequiredVars).First(func(v string) bool {
		return !strings.Contains(body, v)
	}); missing.IsSome() {
		return result.Err[string](fmt.Errorf("template %q missing required variable %q", defaultPrompt.Slug, missing.MustGet()))
	}

	// Substitute variables
	substituted := strings.ReplaceAll(body, "{domain}", domain)
	substituted = strings.ReplaceAll(substituted, "{role}", role)

	return result.OK(substituted)
}

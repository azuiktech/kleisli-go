package examples

import (
	"errors"
	"net/http"
	"strings"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/stream"
)

// ============================================================================
// PROBLEM: Bearer Token Parser & CORS Policy Evaluator (Security / HTTP)
// ============================================================================
// Mini Problem Definition:
// 1. Safely extract an optional `Authorization: Bearer <token>` header without string splits.
// 2. Validate the request's `Origin` header against a whitelist supporting exact domain
//    matches and wildcard domain patterns (e.g. `*.acme.com` or `*`).
// 3. Return a `adt.Result[string]` containing the token if valid, or a domain error.

// ExtractBearerToken extracts an optional Bearer token using adt.FromOk.
func ExtractBearerToken(r *http.Request) adt.Option[string] {
	return adt.FromOk(strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "))
}

// IsOriginAllowed checks if an origin is permitted using declarative stream matching.
func IsOriginAllowed(allowedOrigins []string, requestOrigin string) bool {
	if requestOrigin == "" {
		return true // Same-origin or non-browser request
	}

	return stream.Of(allowedOrigins).Any(func(allowed string) bool {
		if allowed == "*" || allowed == requestOrigin {
			return true
		}
		// Wildcard domain matching: "*.acme.com" matches "sub.acme.com"
		if strings.HasPrefix(allowed, "*.") {
			domainSuffix := strings.TrimPrefix(allowed, "*")
			return strings.HasSuffix(requestOrigin, domainSuffix)
		}
		return false
	})
}

// ResolveAuthenticatedSession combines token extraction, CORS validation, and token verification.
func ResolveAuthenticatedSession(r *http.Request, allowedOrigins []string) adt.Result[string] {
	requestOrigin := r.Header.Get("Origin")

	// Step 1: Validate CORS Origin Policy
	if !IsOriginAllowed(allowedOrigins, requestOrigin) {
		return adt.Err[string](errors.New("CORS policy error: origin forbidden"))
	}

	// Step 2: Extract Bearer token as Option[string] and convert to Result[string]
	return ExtractBearerToken(r).
		ToResultGet(func() error {
			return errors.New("unauthorized: missing or malformed Bearer token")
		}).
		FlatMap(func(token string) adt.Result[string] {
			if token == "revoked-token" {
				return adt.Err[string](errors.New("unauthorized: token revoked"))
			}
			return adt.OK(token)
		})
}

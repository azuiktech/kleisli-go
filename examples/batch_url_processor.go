package examples

import (
	"errors"
	"fmt"
	"strings"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/stream"
)

// ============================================================================
// PROBLEM: Resilient Concurrency-Limited Batch Fetcher & Byte Counter (Networking)
// ============================================================================
// Mini Problem Definition:
// Given a slice of target URLs:
// 1. Process requests concurrently across `maxConcurrency` workers.
// 2. Wrap errors with URL context using `MapErrf`.
// 3. Partition results into successful payloads and failed errors, computing total byte counts.

type FetchResponse struct {
	URL       string
	ByteCount int
}

type BatchFetchMetrics struct {
	TotalBytes int
	Successes  []FetchResponse
	Failures   []error
}

// ProcessBatchURLs executes parallel fetch simulation and computes byte metrics.
func ProcessBatchURLs(urls []string, maxConcurrency int) BatchFetchMetrics {
	fetcher := func(url string) adt.Result[FetchResponse] {
		if strings.Contains(url, "unreachable") {
			return adt.Err[FetchResponse](errors.New("connection timeout"))
		}
		// Simulate response body
		body := fmt.Sprintf("content_for_%s", url)
		return adt.OK(FetchResponse{
			URL:       url,
			ByteCount: len(body),
		})
	}

	// Step 1: Run fetch workers in parallel with bounded concurrency and contextual error wrapping
	results := stream.Of(urls).
		Parallel(maxConcurrency, func(u string) adt.Result[FetchResponse] {
			return fetcher(u).MapErrf("fetch URL %q", u)
		}).
		Collect()

	// Step 2: Partition results into successes and failures
	succs := adt.Successes(results)
	errs := adt.Failures(results)

	// Step 3: Compute aggregate total bytes using stream.Of
	totalBytes := stream.Of(succs).
		Map(func(res FetchResponse) int { return res.ByteCount }).
		Collect()


	sumBytes := 0
	for _, b := range totalBytes {
		sumBytes += b
	}

	return BatchFetchMetrics{
		TotalBytes: sumBytes,
		Successes:  succs,
		Failures:   errs,
	}
}

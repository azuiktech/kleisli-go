package examples

import (
	"fmt"
	"strings"

	"github.com/azuiktech/kleisli-go/result"
	"github.com/azuiktech/kleisli-go/stream"
)

// ============================================================================
// PROBLEM: Bounded Parallel Task Pipeline & Failure Partitioning (Concurrency)
// ============================================================================
// Mini Problem Definition:
// 1. Process a slice of input task payloads concurrently across `concurrency` worker goroutines.
// 2. Wrap task processing in `result.Result[ProcessedItem]`.
// 3. Partition results into successful outputs and errors without manual mutexes or channels.

type TaskItem struct {
	ID    string
	Input string
}

type ProcessedItem struct {
	TaskID string
	Output string
}

type BatchProcessingReport struct {
	Successes []ProcessedItem
	Failures  []error
}

// ExecuteBatchTasksParallel executes task processing concurrently and partitions results.
func ExecuteBatchTasksParallel(tasks []TaskItem, concurrency int) BatchProcessingReport {
	worker := func(t TaskItem) result.Result[ProcessedItem] {
		if strings.HasPrefix(t.Input, "invalid") {
			return result.Err[ProcessedItem](fmt.Errorf("task %s failed: invalid payload", t.ID))
		}
		return result.OK(ProcessedItem{
			TaskID: t.ID,
			Output: strings.ToUpper(t.Input),
		})
	}

	// Step 1: Execute worker in parallel over streams
	results := stream.Of(tasks).
		Parallel(concurrency, worker).
		Collect()

	// Step 2: Partition results into successes and failures
	succs := result.Successes(results)
	errs := result.Failures(results)

	return BatchProcessingReport{
		Successes: succs,
		Failures:  errs,
	}
}


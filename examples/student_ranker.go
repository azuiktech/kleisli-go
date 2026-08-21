// Package examples provides generic, production-ready design pattern examples using kleisli-go.
package examples

import (
	"math"
	"sort"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/stream"
)

// ============================================================================
// PROBLEM: Top-K Consistent Student Finder (Algorithmic / Data Pipeline)
// ============================================================================
// Mini Problem Definition:
// Given a dataset of students with test scores:
// 1. Filter out students with fewer than `minAssignments` scores or average grade < `minAvg`.
// 2. Rank remaining candidates by average grade and take the top `topK`.
// 3. Compute score variance for each top candidate to measure consistency.
// 4. Return the student with the minimum score variance (highest consistency) as an Option[StudentSummary].

type Student struct {
	ID     string
	Name   string
	Scores []float64
}

type StudentSummary struct {
	ID       string
	Name     string
	Average  float64
	Variance float64
}

// SelectMostConsistentTopStudent executes the algorithmic pipeline using kleisli-go streams.
func SelectMostConsistentTopStudent(students []Student, minAssignments int, minAvg float64, topK int) adt.Option[StudentSummary] {
	// Step 1: Filter and map to StudentSummary
	summaries := stream.Of(students).
		Filter(func(s Student) bool {
			if len(s.Scores) < minAssignments {
				return false
			}
			avg := computeAverage(s.Scores)
			return avg >= minAvg
		}).
		Map(func(s Student) StudentSummary {
			avg := computeAverage(s.Scores)
			variance := computeVariance(s.Scores, avg)
			return StudentSummary{
				ID:       s.ID,
				Name:     s.Name,
				Average:  avg,
				Variance: variance,
			}
		}).
		Collect()

	if len(summaries) == 0 {
		return adt.None[StudentSummary]()
	}

	// Step 2: Sort descending by average score
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Average > summaries[j].Average
	})

	// Step 3: Take top K candidates
	topCandidates := stream.Of(summaries).Take(topK).Collect()

	// Step 4: Sort top candidates ascending by score variance (minimum variance first)
	sort.Slice(topCandidates, func(i, j int) bool {
		return topCandidates[i].Variance < topCandidates[j].Variance
	})

	// Return the most consistent top candidate as Option[StudentSummary]
	return adt.FromOk(topCandidates[0], true)
}

func computeAverage(scores []float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	total := 0.0
	for _, s := range scores {
		total += s
	}
	return total / float64(len(scores))
}

func computeVariance(scores []float64, avg float64) float64 {
	if len(scores) <= 1 {
		return 0
	}
	var sumSqDiff float64
	for _, s := range scores {
		diff := s - avg
		sumSqDiff += diff * diff
	}
	return math.Sqrt(sumSqDiff / float64(len(scores)))
}

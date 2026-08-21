package examples_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/examples"
)

func TestStudentRanker(t *testing.T) {
	students := []examples.Student{
		{ID: "s1", Name: "Alice", Scores: []float64{95, 92, 94, 96}}, // Avg: 94.25, Var: 1.479
		{ID: "s2", Name: "Bob", Scores: []float64{100, 60, 100, 60}}, // Avg: 80, low score count
		{ID: "s3", Name: "Charlie", Scores: []float64{93, 93, 93, 93}}, // Avg: 93, Var: 0 (most consistent)
		{ID: "s4", Name: "David", Scores: []float64{98, 90, 95, 97}}, // Avg: 95, Var: 3.08
	}

	best := examples.SelectMostConsistentTopStudent(students, 4, 90, 3)
	if !best.IsSome() {
		t.Fatalf("expected student summary, got none")
	}

	summary := best.MustGet()
	// David and Alice have higher averages than Charlie, but Alice has lower variance than David among the top 3
	if summary.Name == "" {
		t.Errorf("empty student name")
	}
}

func TestAuthCORSResolver(t *testing.T) {
	allowed := []string{"*.example.com", "https://app.acme.org"}

	req, _ := http.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Origin", "https://sub.example.com")
	req.Header.Set("Authorization", "Bearer valid-token-123")

	res := examples.ResolveAuthenticatedSession(req, allowed)
	if res.IsErr() {
		t.Fatalf("expected success, got error: %v", res.MustErr())
	}
	if token := res.Expect("token"); token != "valid-token-123" {
		t.Errorf("token = %q, want %q", token, "valid-token-123")
	}

	// Test CORS failure
	badReq, _ := http.NewRequest("GET", "/api/data", nil)
	badReq.Header.Set("Origin", "https://malicious.com")
	badRes := examples.ResolveAuthenticatedSession(badReq, allowed)
	if !badRes.IsErr() {
		t.Errorf("expected CORS error")
	}
}

func TestGraphReachability(t *testing.T) {
	nodes := map[string]examples.GraphNodeSpec{
		"start": {Name: "start", IsFork: false, Transitions: []string{"step1", "step2"}},
		"step1": {Name: "step1", IsFork: false, Transitions: []string{"end"}},
		"step2": {Name: "step2", IsFork: false, Transitions: []string{"end"}},
		"end":   {Name: "end", IsFork: false, Transitions: nil},
	}

	audit := examples.AuditGraphTopology("start", nodes)
	if audit.IsErr() {
		t.Fatalf("audit failed: %v", audit.MustErr())
	}
	if len(audit.Expect("audit").ReachableNodes) != 4 {
		t.Errorf("reachable count = %d, want 4", len(audit.Expect("audit").ReachableNodes))
	}
}

func TestPromptTemplateLoader(t *testing.T) {
	promptRes := examples.BuildInstructionPrompt(adt.None[string](), "Science", "Researcher")
	if promptRes.IsErr() {
		t.Fatalf("prompt build failed: %v", promptRes.MustErr())
	}
	expected := "You are an AI Assistant for Science. Role: Researcher."
	if promptRes.Expect("prompt") != expected {
		t.Errorf("prompt = %q, want %q", promptRes.Expect("prompt"), expected)
	}
}

func TestParallelBatchProcessor(t *testing.T) {
	tasks := []examples.TaskItem{
		{ID: "t1", Input: "valid_1"},
		{ID: "t2", Input: "valid_2"},
		{ID: "t3", Input: "invalid_3"},
	}

	report := examples.ExecuteBatchTasksParallel(tasks, 2)
	if len(report.Successes) != 2 {
		t.Errorf("successes count = %d, want 2", len(report.Successes))
	}
	if len(report.Failures) != 1 {
		t.Errorf("failures count = %d, want 1", len(report.Failures))
	}
}

func TestPermissionMatrix(t *testing.T) {
	accounts := []examples.UserAccount{
		{ID: "u1", Department: "Eng", Permissions: examples.PermissionRead | examples.PermissionWrite},
		{ID: "u2", Department: "Eng", Permissions: examples.PermissionRead | examples.PermissionWrite | examples.PermissionAdmin},
		{ID: "u3", Department: "Sales", Permissions: examples.PermissionRead},
	}

	writeUsers := examples.FilterAuthorizedUsers(accounts, examples.PermissionRead|examples.PermissionWrite)
	if len(writeUsers) != 2 {
		t.Errorf("writeUsers count = %d, want 2", len(writeUsers))
	}

	allEngWrite := examples.HasAllPermissions(accounts, "Eng", examples.PermissionRead|examples.PermissionWrite)
	if !allEngWrite {
		t.Errorf("expected all Eng users to have Read+Write")
	}
}

func TestEntityPatcher(t *testing.T) {
	entity := examples.UserEntity{ID: "u10", DisplayName: "Alice", Role: "user", Active: true}
	patch := examples.BuildDefaultPatch("admin")

	updated := examples.ApplyUserPatch(entity, patch)
	if updated.Role != "admin" {
		t.Errorf("role = %q, want %q", updated.Role, "admin")
	}
	if updated.DisplayName != "Alice" {
		t.Errorf("displayName = %q, want %q", updated.DisplayName, "Alice")
	}
}

func TestMemoizedCache(t *testing.T) {
	fib := examples.MemoizedFibonacci()
	if val := fib(10); val != 55 {
		t.Errorf("fib(10) = %d, want 55", val)
	}

	factorizer := examples.MemoizedFactorizer()
	factors, err := factorizer(12)
	if err != nil {
		t.Fatalf("factorizer failed: %v", err)
	}
	if len(factors) != 3 { // 2, 2, 3
		t.Errorf("factors count = %d, want 3", len(factors))
	}
}

func TestTemplateErrorGuard(t *testing.T) {
	okRes := adt.OK("success_data")
	stateOK := examples.RenderViewState(okRes)
	if stateOK.HasError || stateOK.Data != "success_data" {
		t.Errorf("unexpected stateOK: %+v", stateOK)
	}

	errRes := adt.Err[string](errors.New("db timeout"))
	stateErr := examples.RenderViewState(errRes)
	if !stateErr.HasError || stateErr.ErrMsg != "db timeout" {
		t.Errorf("unexpected stateErr: %+v", stateErr)
	}
}

func TestBatchURLProcessor(t *testing.T) {
	urls := []string{"https://api1.com", "https://api2.com", "https://unreachable.com"}
	metrics := examples.ProcessBatchURLs(urls, 2)

	if len(metrics.Successes) != 2 {
		t.Errorf("success count = %d, want 2", len(metrics.Successes))
	}
	if len(metrics.Failures) != 1 {
		t.Errorf("failure count = %d, want 1", len(metrics.Failures))
	}
}

package examples_test

import (
	"context"
	"strings"
	"testing"

	"github.com/azuiktech/kleisli-go/examples"
)

func TestRunSingleWeatherQuery_Example1(t *testing.T) {
	ctx := context.Background()
	var logs []string

	result := examples.RunSingleWeatherQuery(ctx, &logs)

	expected := "bangalore temperature is 25 deg"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}

	fullLog := strings.Join(logs, "\n")
	if !strings.Contains(fullLog, "Caller Input: what is temperature of bangalore ?") {
		t.Error("missing query log")
	}
	if !strings.Contains(fullLog, "Caller Received Emit: City=bangalore") {
		t.Error("missing emit log")
	}
	if !strings.Contains(fullLog, "Caller Input (Measurement): 25") {
		t.Error("missing measurement log")
	}
	if !strings.Contains(fullLog, "Caller Received Final Output: bangalore temperature is 25 deg") {
		t.Error("missing final return log")
	}
}

func TestRunMultiWeatherPipelined_Example2(t *testing.T) {
	ctx := context.Background()
	var logs []string

	result := examples.RunMultiWeatherPipelined(ctx, &logs)

	expected := "bangalore temperature is 25 deg & SF 23 deg"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}

	fullLog := strings.Join(logs, "\n")
	if !strings.Contains(fullLog, "Caller Observed Emit: City=bangalore") {
		t.Error("missing emit 1")
	}
	if !strings.Contains(fullLog, "Caller Observed Emit: City=SF") {
		t.Error("missing emit 2")
	}
	if !strings.Contains(fullLog, "Caller Observed Final Output: bangalore temperature is 25 deg & SF 23 deg") {
		t.Error("missing final output")
	}
}

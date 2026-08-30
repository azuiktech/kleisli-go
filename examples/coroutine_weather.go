package examples

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/async"
)

// Temperature represents a query or measurement.
type Temperature struct {
	City  string
	Value float64
}

// RunSingleWeatherQuery demonstrates Example 1 (Typed Promise & Task):
// 1. Task launches with typed Promise[string] context.
// 2. Task yields/emits Temperature query and suspends waiting for measurement string.
// 3. Caller feeds measurement "25".
// 4. Task completes with final text.
func RunSingleWeatherQuery(ctx context.Context, logs *[]string) string {
	var mu sync.Mutex
	log := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		if logs != nil {
			*logs = append(*logs, msg)
		}
		fmt.Println(msg)
	}

	task := async.Launch(ctx, func(p *async.Promise[string]) adt.Result[string] {
		query := async.Receive(p).OrElse("")
		city := parseCity(query)

		// Emit query to caller
		async.Emit(p, Temperature{City: city})

		// Suspend and wait for temperature value
		tempVal := async.Receive(p).OrElse("")

		return adt.OK(fmt.Sprintf("%s temperature is %s deg", city, tempVal))
	})

	task.OnEmit(func(val any) {
		t := val.(Temperature)
		log(fmt.Sprintf("Caller Received Emit: City=%s", t.City))
	})

	// 1. Send query
	input1 := "what is temperature of bangalore ?"
	log(fmt.Sprintf("Caller Input: %s", input1))
	task.Send(input1)

	time.Sleep(10 * time.Millisecond)

	// 2. Send measurement
	input2 := "25"
	log(fmt.Sprintf("Caller Input (Measurement): %s", input2))
	task.Send(input2)

	// 3. Await final output
	finalResult := task.Await().MustGet()
	log(fmt.Sprintf("Caller Received Final Output: %s", finalResult))

	return finalResult
}

// RunMultiWeatherPipelined demonstrates Example 2:
// Pipelined multi-city query with asynchronous OnDone.
func RunMultiWeatherPipelined(ctx context.Context, logs *[]string) string {
	var mu sync.Mutex
	log := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		if logs != nil {
			*logs = append(*logs, msg)
		}
		fmt.Println(msg)
	}

	task := async.Launch(ctx, func(p *async.Promise[string]) adt.Result[string] {
		// Read cities
		c1 := parseCity(async.Receive(p).OrElse(""))
		async.Emit(p, Temperature{City: c1})

		c2 := parseCity(async.Receive(p).OrElse(""))
		async.Emit(p, Temperature{City: c2})

		// Read measurements
		t1 := async.Receive(p).OrElse("")
		t2 := async.Receive(p).OrElse("")

		return adt.OK(fmt.Sprintf("%s temperature is %s deg & %s %s deg", c1, t1, c2, t2))
	})

	task.OnEmit(func(val any) {
		t := val.(Temperature)
		log(fmt.Sprintf("Caller Observed Emit: City=%s", t.City))
	})

	var wg sync.WaitGroup
	var finalOutput string
	wg.Add(1)

	task.OnDone(func(res adt.Result[string]) {
		defer wg.Done()
		if res.IsOK() {
			finalOutput = res.MustGet()
			log(fmt.Sprintf("Caller Observed Final Output: %s", finalOutput))
		}
	})

	// Caller pushes all inputs in a loop without waiting
	inputs := []string{
		"what is temperature of bangalore ?",
		"what is temperature of SF ?",
		"25",
		"23",
	}

	for _, in := range inputs {
		log(fmt.Sprintf("Caller Sent: %s", in))
		task.Send(in)
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()
	return finalOutput
}

func parseCity(query string) string {
	lower := strings.ToLower(query)
	if strings.Contains(lower, "bangalore") {
		return "bangalore"
	}
	if strings.Contains(lower, "sf") || strings.Contains(lower, "san francisco") {
		return "SF"
	}
	return "unknown"
}

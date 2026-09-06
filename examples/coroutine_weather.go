package examples

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/async"
)

// Temperature represents a query or measurement.
type Temperature struct {
	City  string
	Value float64
}

// WeatherQuery requests a temperature measurement for a city.
type WeatherQuery struct {
	City string
}

// QueryWeather defines the strongly-typed operation: WeatherQuery -> string
var QueryWeather = async.DefineOp[WeatherQuery, string]("query_weather")

// RunSingleWeatherQuery demonstrates:
// 1. Task launches with Config containing context and callbacks.
// 2. Coroutine emits Temperature query to caller (Case B).
// 3. Coroutine calls caller for measurement value and suspends (Case E).
// 4. Caller responds with measurement "25" via strongly typed handler.
// 5. Task completes with final text.
func RunSingleWeatherQuery(ctx context.Context, logs *[]string) string {
	var mu sync.Mutex
	log := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		adt.Opt(logs).Tap(func(l *[]string) {
			*l = append(*l, msg)
		})
		fmt.Println(msg)
	}

	city := "bangalore"
	input1 := "what is temperature of bangalore ?"
	log(fmt.Sprintf("Caller Input: %s", input1))

	cfg := async.Config{
		Context: ctx,
		OnEmit: func(val any) {
			t := val.(Temperature)
			log(fmt.Sprintf("Caller Received Emit: City=%s", t.City))
		},
	}
	async.RegisterHandler(&cfg, QueryWeather, func(q WeatherQuery) adt.Result[string] {
		input2 := "25"
		log(fmt.Sprintf("Caller Input (Measurement): %s", input2))
		return adt.OK(input2)
	})

	task := async.Launch[string, string](cfg, city, func(co *async.Co, inCity string) adt.Result[string] {
		// Emit query to caller
		co.Emit(Temperature{City: inCity})

		// Suspend and ask caller for temperature measurement
		tempVal := co.Call(QueryWeather, WeatherQuery{City: inCity}).Await().MustGet()

		return adt.OK(fmt.Sprintf("%s temperature is %s deg", inCity, tempVal))
	})

	finalResult := task.Await().MustGet()
	log(fmt.Sprintf("Caller Received Final Output: %s", finalResult))

	return finalResult
}

// RunMultiWeatherPipelined demonstrates:
// Multiple emissions and calls in a coroutine workflow configured via Config.
func RunMultiWeatherPipelined(ctx context.Context, logs *[]string) string {
	var mu sync.Mutex
	log := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		adt.Opt(logs).Tap(func(l *[]string) {
			*l = append(*l, msg)
		})
		fmt.Println(msg)
	}

	cfg := async.Config{
		Context: ctx,
		OnEmit: func(val any) {
			t := val.(Temperature)
			log(fmt.Sprintf("Caller Observed Emit: City=%s", t.City))
		},
	}
	async.RegisterHandler(&cfg, QueryWeather, func(q WeatherQuery) adt.Result[string] {
		switch q.City {
		case "bangalore":
			return adt.OK("25")
		case "SF":
			return adt.OK("23")
		default:
			return adt.Err[string](fmt.Errorf("unknown city: %s", q.City))
		}
	})

	task := async.Launch[adt.Unit, string](cfg, adt.Void, func(co *async.Co, _ adt.Unit) adt.Result[string] {
		// Emit cities to caller
		co.Emit(Temperature{City: "bangalore"})
		co.Emit(Temperature{City: "SF"})

		// Call for measurements
		t1 := co.Call(QueryWeather, WeatherQuery{City: "bangalore"}).Await().MustGet()
		t2 := co.Call(QueryWeather, WeatherQuery{City: "SF"}).Await().MustGet()

		return adt.OK(fmt.Sprintf("bangalore temperature is %s deg & SF %s deg", t1, t2))
	})

	finalResult := task.Await().MustGet()
	log(fmt.Sprintf("Caller Observed Final Output: %s", finalResult))

	return finalResult
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

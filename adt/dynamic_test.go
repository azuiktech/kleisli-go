package adt_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/azuiktech/kleisli-go/adt"
)

type dynAlpha struct{ X int }
type dynBeta struct{ Y string }

func init() {
	adt.Register[dynAlpha]("adt_test.dynAlpha")
	adt.Register[dynBeta]("adt_test.dynBeta")
}

func TestDynamic_ConcurrentReads(t *testing.T) {
	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Test Dyn
				a := adt.Dyn(dynAlpha{X: idx*1000 + j})
				opt := adt.As[dynAlpha](a)
				if !opt.IsSome() || opt.MustGet().X != idx*1000+j {
					t.Errorf("unexpected value in Dyn/As: %v", opt)
					return
				}

				// Test MarshalJSON / UnmarshalJSON
				data, err := json.Marshal(a)
				if err != nil {
					t.Errorf("Marshal error: %v", err)
					return
				}

				var unmarshaled adt.Any
				if err := json.Unmarshal(data, &unmarshaled); err != nil {
					t.Errorf("Unmarshal error: %v", err)
					return
				}

				opt2 := adt.As[dynAlpha](unmarshaled)
				if !opt2.IsSome() || opt2.MustGet().X != idx*1000+j {
					t.Errorf("unexpected unmarshaled value: %v", opt2)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestDynamic_DuplicateRegistration_Panics(t *testing.T) {
	// Attempting to register the same type again should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when registering duplicate type")
		}
	}()
	adt.Register[dynAlpha]("adt_test.dynAlpha_duplicate")
}

func TestDynamic_DuplicateName_Panics(t *testing.T) {
	type dynGamma struct{ Z bool }
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when registering duplicate name")
		}
	}()
	// Using existing name with different type
	adt.Register[dynGamma]("adt_test.dynAlpha")
}

func TestDynamic_ConcurrentRegistration_Race(t *testing.T) {
	// Dynamically register unique types concurrently and ensure no data race
	type regType1 struct{ A int }
	type regType2 struct{ B int }
	type regType3 struct{ C int }
	type regType4 struct{ D int }

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		adt.Register[regType1]("adt_test.regType1")
	}()
	go func() {
		defer wg.Done()
		adt.Register[regType2]("adt_test.regType2")
	}()
	go func() {
		defer wg.Done()
		adt.Register[regType3]("adt_test.regType3")
	}()
	go func() {
		defer wg.Done()
		adt.Register[regType4]("adt_test.regType4")
	}()

	wg.Wait()

	// Verify all 4 are usable
	if a := adt.Dyn(regType1{A: 1}); !adt.As[regType1](a).IsSome() {
		t.Fatal("regType1 failed")
	}
	if a := adt.Dyn(regType2{B: 2}); !adt.As[regType2](a).IsSome() {
		t.Fatal("regType2 failed")
	}
	if a := adt.Dyn(regType3{C: 3}); !adt.As[regType3](a).IsSome() {
		t.Fatal("regType3 failed")
	}
	if a := adt.Dyn(regType4{D: 4}); !adt.As[regType4](a).IsSome() {
		t.Fatal("regType4 failed")
	}
}

func TestDynamic_ConcurrentSameTypeRegistration_OneWins(t *testing.T) {
	type contestedType struct{ Val int }
	const competitors = 8

	var wg sync.WaitGroup
	wg.Add(competitors)

	panics := make([]bool, competitors)
	for i := 0; i < competitors; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics[idx] = true
				}
			}()
			// Each tries to register contestedType under a unique name
			adt.Register[contestedType](fmt.Sprintf("adt_test.contested_%d", idx))
		}(i)
	}

	wg.Wait()

	// Exactly 1 should have succeeded, competitors - 1 should have panicked
	panicCount := 0
	for _, p := range panics {
		if p {
			panicCount++
		}
	}
	if panicCount != competitors-1 {
		t.Fatalf("expected exactly %d panics, got %d", competitors-1, panicCount)
	}
}

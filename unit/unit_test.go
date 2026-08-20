package unit_test

import (
	"testing"

	"github.com/azuiktech/kleisli-go/unit"
)

func TestUnit_IsStructAlias(t *testing.T) {
	var u unit.Unit
	var s struct{}
	if u != s {
		t.Fatal("unit.Unit must equal struct{}")
	}
}

package fn_test

import (
	json "encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/fn"
	"github.com/azuiktech/kleisli-go/stream"
)

type sampleUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestFromJSON(t *testing.T) {
	t.Run("valid json bytes", func(t *testing.T) {
		input := []byte(`{"id":42,"name":"Alice"}`)
		res := fn.FromJSON[sampleUser](input)
		if res.IsErr() {
			t.Fatalf("expected OK, got err: %v", res.MustErr())
		}
		user := res.MustGet()
		if user.ID != 42 || user.Name != "Alice" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})

	t.Run("primitive type", func(t *testing.T) {
		input := []byte(`12345`)
		res := fn.FromJSON[int](input)
		if res.IsErr() || res.MustGet() != 12345 {
			t.Fatalf("expected 12345, got %v", res)
		}
	})

	t.Run("slice of structs", func(t *testing.T) {
		input := []byte(`[{"id":1,"name":"A"},{"id":2,"name":"B"}]`)
		res := fn.FromJSON[[]sampleUser](input)
		if res.IsErr() {
			t.Fatalf("expected OK, got err: %v", res.MustErr())
		}
		users := res.MustGet()
		if len(users) != 2 || users[0].ID != 1 || users[1].ID != 2 {
			t.Fatalf("unexpected users: %+v", users)
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		input := []byte(`{invalid json}`)
		res := fn.FromJSON[sampleUser](input)
		if !res.IsErr() {
			t.Fatal("expected error on invalid json, got OK")
		}
	})

	t.Run("with json options reject unknown members", func(t *testing.T) {
		input := []byte(`{"id":1,"name":"A","extra":true}`)
		res := fn.FromJSONOpts[sampleUser](input, json.RejectUnknownMembers(true))
		if !res.IsErr() {
			t.Fatal("expected error with RejectUnknownMembers, got OK")
		}
	})
}

func TestFromJSONString(t *testing.T) {
	t.Run("valid json string", func(t *testing.T) {
		res := fn.FromJSONString[sampleUser](`{"id":100,"name":"Bob"}`)
		if res.IsErr() {
			t.Fatalf("expected OK, got err: %v", res.MustErr())
		}
		if user := res.MustGet(); user.ID != 100 || user.Name != "Bob" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})

	t.Run("invalid json string", func(t *testing.T) {
		res := fn.FromJSONString[sampleUser](`not json`)
		if !res.IsErr() {
			t.Fatal("expected error on invalid string, got OK")
		}
	})
}

func TestToJSON(t *testing.T) {
	t.Run("valid struct to bytes", func(t *testing.T) {
		u := sampleUser{ID: 1, Name: "Carol"}
		res := fn.ToJSON(u)
		if res.IsErr() {
			t.Fatalf("expected OK, got err: %v", res.MustErr())
		}
		decoded := fn.FromJSON[sampleUser](res.MustGet()).MustGet()
		if !reflect.DeepEqual(decoded, u) {
			t.Fatalf("expected %+v, got %+v", u, decoded)
		}
	})

	t.Run("error on unsupported type", func(t *testing.T) {
		type unmarshalable struct {
			Ch chan int `json:"ch"`
		}
		res := fn.ToJSON(unmarshalable{Ch: make(chan int)})
		if !res.IsErr() {
			t.Fatal("expected error on unsupported channel type, got OK")
		}
	})
}

func TestToJSONIndent(t *testing.T) {
	u := sampleUser{ID: 2, Name: "Dave"}
	res := fn.ToJSONIndent(u, "", "  ")
	if res.IsErr() {
		t.Fatalf("expected OK, got err: %v", res.MustErr())
	}
	str := string(res.MustGet())
	if !strings.Contains(str, "\n") || !strings.Contains(str, "  ") {
		t.Fatalf("expected indented json, got: %s", str)
	}
	decoded := fn.FromJSON[sampleUser](res.MustGet()).MustGet()
	if !reflect.DeepEqual(decoded, u) {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", decoded, u)
	}
}

func TestToJSONString(t *testing.T) {
	u := sampleUser{ID: 3, Name: "Eve"}
	res := fn.ToJSONString(u)
	if res.IsErr() {
		t.Fatalf("expected OK, got err: %v", res.MustErr())
	}
	str := res.MustGet()
	if !strings.Contains(str, `"id":3`) || !strings.Contains(str, `"name":"Eve"`) {
		t.Fatalf("unexpected json string: %s", str)
	}
}

func TestJSONPipelineIntegration(t *testing.T) {
	// Integration test: round-tripping a stream of items through JSON serialization & deserialization
	users := []sampleUser{
		{ID: 1, Name: "User 1"},
		{ID: 2, Name: "User 2"},
		{ID: 3, Name: "User 3"},
	}

	// 1. Serialize stream of structs to stream of JSON strings
	jsonStrings := stream.Of(users).
		Map(fn.ToJSONString[sampleUser]).
		Filter(func(r adt.Result[string]) bool { return r.IsOK() }).
		Map(func(r adt.Result[string]) string { return r.MustGet() }).
		Collect()

	if len(jsonStrings) != 3 {
		t.Fatalf("expected 3 json strings, got %d", len(jsonStrings))
	}

	// 2. Deserialize stream of JSON strings back to structs
	roundtripped := stream.Of(jsonStrings).
		Map(fn.FromJSONString[sampleUser]).
		Filter(func(r adt.Result[sampleUser]) bool { return r.IsOK() }).
		Map(func(r adt.Result[sampleUser]) sampleUser { return r.MustGet() }).
		Collect()

	if !reflect.DeepEqual(users, roundtripped) {
		t.Fatalf("integration stream roundtrip mismatch: got %+v, want %+v", roundtripped, users)
	}
}

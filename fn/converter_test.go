package fn_test

import (
	json "encoding/json/v2"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

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

// ── Numeric parser tests ──────────────────────────────────────────────────────

func TestParseNumeric(t *testing.T) {
	t.Run("ParseInt", func(t *testing.T) {
		if res := fn.ParseInt("42"); res.IsErr() || res.MustGet() != 42 {
			t.Fatalf("ParseInt(42) failed: %v", res)
		}
		if res := fn.ParseInt("-99"); res.IsErr() || res.MustGet() != -99 {
			t.Fatalf("ParseInt(-99) failed: %v", res)
		}
		if res := fn.ParseInt("abc"); !res.IsErr() {
			t.Fatal("ParseInt(abc) expected error")
		}
	})

	t.Run("ParseInt64", func(t *testing.T) {
		if res := fn.ParseInt64("9223372036854775807"); res.IsErr() || res.MustGet() != 9223372036854775807 {
			t.Fatalf("ParseInt64 failed: %v", res)
		}
	})

	t.Run("ParseUint", func(t *testing.T) {
		if res := fn.ParseUint("100"); res.IsErr() || res.MustGet() != 100 {
			t.Fatalf("ParseUint(100) failed: %v", res)
		}
		if res := fn.ParseUint("-1"); !res.IsErr() {
			t.Fatal("ParseUint(-1) expected error")
		}
	})

	t.Run("ParseUint64", func(t *testing.T) {
		if res := fn.ParseUint64("18446744073709551615"); res.IsErr() || res.MustGet() != 18446744073709551615 {
			t.Fatalf("ParseUint64 failed: %v", res)
		}
	})

	t.Run("ParseFloat", func(t *testing.T) {
		if res := fn.ParseFloat("3.14159"); res.IsErr() || res.MustGet() != 3.14159 {
			t.Fatalf("ParseFloat(3.14159) failed: %v", res)
		}
		if res := fn.ParseFloat("bad"); !res.IsErr() {
			t.Fatal("ParseFloat(bad) expected error")
		}
	})

	t.Run("ParseFloat32", func(t *testing.T) {
		if res := fn.ParseFloat32("1.5"); res.IsErr() || res.MustGet() != 1.5 {
			t.Fatalf("ParseFloat32(1.5) failed: %v", res)
		}
	})

	t.Run("Generic Parse[N]", func(t *testing.T) {
		if res := fn.Parse[int]("10"); res.IsErr() || res.MustGet() != 10 {
			t.Fatalf("Parse[int] failed: %v", res)
		}
		if res := fn.Parse[int8]("127"); res.IsErr() || res.MustGet() != 127 {
			t.Fatalf("Parse[int8](127) failed: %v", res)
		}
		if res := fn.Parse[int8]("128"); !res.IsErr() {
			t.Fatal("Parse[int8](128) expected overflow error")
		}
		if res := fn.Parse[int16]("32767"); res.IsErr() || res.MustGet() != 32767 {
			t.Fatalf("Parse[int16](32767) failed: %v", res)
		}
		if res := fn.Parse[int16]("32768"); !res.IsErr() {
			t.Fatal("Parse[int16](32768) expected overflow error")
		}
		if res := fn.Parse[uint8]("255"); res.IsErr() || res.MustGet() != 255 {
			t.Fatalf("Parse[uint8](255) failed: %v", res)
		}
		if res := fn.Parse[uint8]("256"); !res.IsErr() {
			t.Fatal("Parse[uint8](256) expected overflow error")
		}
		if res := fn.Parse[float64]("2.718"); res.IsErr() || res.MustGet() != 2.718 {
			t.Fatalf("Parse[float64] failed: %v", res)
		}
	})
}

// ── Time & Duration tests ─────────────────────────────────────────────────────

func TestParseTimeAndDuration(t *testing.T) {
	t.Run("ParseDuration", func(t *testing.T) {
		if res := fn.ParseDuration("5s"); res.IsErr() || res.MustGet() != 5*time.Second {
			t.Fatalf("ParseDuration(5s) failed: %v", res)
		}
		if res := fn.ParseDuration("invalid"); !res.IsErr() {
			t.Fatal("ParseDuration(invalid) expected error")
		}
	})

	t.Run("ParseTime", func(t *testing.T) {
		res := fn.ParseTime("2006-01-02", "2026-09-03")
		if res.IsErr() {
			t.Fatalf("ParseTime failed: %v", res.MustErr())
		}
		parsed := res.MustGet()
		if parsed.Year() != 2026 || parsed.Month() != 9 || parsed.Day() != 3 {
			t.Fatalf("unexpected parsed time: %v", parsed)
		}
		if errRes := fn.ParseTime("2006-01-02", "not-a-date"); !errRes.IsErr() {
			t.Fatal("ParseTime expected error on invalid date")
		}
	})

	t.Run("ParseRFC3339", func(t *testing.T) {
		res := fn.ParseRFC3339("2026-09-03T20:00:00Z")
		if res.IsErr() {
			t.Fatalf("ParseRFC3339 failed: %v", res.MustErr())
		}
		if res.MustGet().Year() != 2026 {
			t.Fatalf("unexpected year: %v", res.MustGet().Year())
		}
		if errRes := fn.ParseRFC3339("bad-rfc3339"); !errRes.IsErr() {
			t.Fatal("ParseRFC3339 expected error")
		}
	})
}

// ── Base64 tests ──────────────────────────────────────────────────────────────

func TestBase64(t *testing.T) {
	raw := []byte("kleisli-go base64 test payload 12345!?")

	t.Run("standard base64 roundtrip", func(t *testing.T) {
		encoded := fn.ToBase64(raw)
		decodedRes := fn.FromBase64(encoded)
		if decodedRes.IsErr() {
			t.Fatalf("FromBase64 failed: %v", decodedRes.MustErr())
		}
		if string(decodedRes.MustGet()) != string(raw) {
			t.Fatalf("roundtrip mismatch: got %q, want %q", decodedRes.MustGet(), raw)
		}
		strRes := fn.FromBase64String(encoded)
		if strRes.IsErr() || strRes.MustGet() != string(raw) {
			t.Fatalf("FromBase64String mismatch: %v", strRes)
		}
	})

	t.Run("standard base64 invalid", func(t *testing.T) {
		if res := fn.FromBase64("not valid base64!!!"); !res.IsErr() {
			t.Fatal("expected error on invalid base64")
		}
	})

	t.Run("url-safe base64 roundtrip", func(t *testing.T) {
		encoded := fn.ToBase64URL(raw)
		if strings.ContainsAny(encoded, "+/=") {
			t.Fatalf("ToBase64URL contains illegal chars or padding: %s", encoded)
		}
		decodedRes := fn.FromBase64URL(encoded)
		if decodedRes.IsErr() {
			t.Fatalf("FromBase64URL failed: %v", decodedRes.MustErr())
		}
		if string(decodedRes.MustGet()) != string(raw) {
			t.Fatalf("roundtrip mismatch: got %q, want %q", decodedRes.MustGet(), raw)
		}
		strRes := fn.FromBase64URLString(encoded)
		if strRes.IsErr() || strRes.MustGet() != string(raw) {
			t.Fatalf("FromBase64URLString mismatch: %v", strRes)
		}
	})
}

// ── URL & Query tests ─────────────────────────────────────────────────────────

func TestURLEncoding(t *testing.T) {
	t.Run("query escape and unescape", func(t *testing.T) {
		input := "hello world & special=chars?"
		escaped := fn.ToQueryEscape(input)
		unescapedRes := fn.FromQueryUnescape(escaped)
		if unescapedRes.IsErr() {
			t.Fatalf("FromQueryUnescape failed: %v", unescapedRes.MustErr())
		}
		if unescapedRes.MustGet() != input {
			t.Fatalf("got %q, want %q", unescapedRes.MustGet(), input)
		}
		if badRes := fn.FromQueryUnescape("%ZZ"); !badRes.IsErr() {
			t.Fatal("expected error on bad hex escape")
		}
	})

	t.Run("path escape and unescape", func(t *testing.T) {
		path := "a/b c/d"
		escaped := fn.ToPathEscape(path)
		unescapedRes := fn.FromPathUnescape(escaped)
		if unescapedRes.IsErr() {
			t.Fatalf("FromPathUnescape failed: %v", unescapedRes.MustErr())
		}
		if unescapedRes.MustGet() != path {
			t.Fatalf("got %q, want %q", unescapedRes.MustGet(), path)
		}
	})

	t.Run("query string values roundtrip", func(t *testing.T) {
		rawQuery := "filter=active&sort=desc"
		valuesRes := fn.FromQueryString(rawQuery)
		if valuesRes.IsErr() {
			t.Fatalf("FromQueryString failed: %v", valuesRes.MustErr())
		}
		var values url.Values = valuesRes.MustGet()
		if values.Get("filter") != "active" || values.Get("sort") != "desc" {
			t.Fatalf("unexpected query values: %v", values)
		}
		encoded := fn.ToQueryString(values)
		if !strings.Contains(encoded, "filter=active") || !strings.Contains(encoded, "sort=desc") {
			t.Fatalf("ToQueryString failed: %s", encoded)
		}
		if badRes := fn.FromQueryString("key=%ZZ"); !badRes.IsErr() {
			t.Fatal("expected error on invalid query string")
		}
	})
}

// ── Stream Integration tests ──────────────────────────────────────────────────

func TestConverter_StreamIntegration(t *testing.T) {
	t.Run("stream parsing numbers", func(t *testing.T) {
		rawNumbers := []string{"10", "20", "invalid", "30"}
		parsed := stream.Of(rawNumbers).
			Map(fn.Parse[int]).
			Filter(func(r adt.Result[int]) bool { return r.IsOK() }).
			Map(func(r adt.Result[int]) int { return r.MustGet() }).
			Collect()

		expected := []int{10, 20, 30}
		if !reflect.DeepEqual(parsed, expected) {
			t.Fatalf("stream Parse[int] failed: got %v, want %v", parsed, expected)
		}
	})

	t.Run("stream parsing durations", func(t *testing.T) {
		rawDurations := []string{"1s", "2m", "3h"}
		durations := stream.Of(rawDurations).
			Map(fn.ParseDuration).
			Filter(func(r adt.Result[time.Duration]) bool { return r.IsOK() }).
			Map(func(r adt.Result[time.Duration]) time.Duration { return r.MustGet() }).
			Collect()

		expected := []time.Duration{time.Second, 2 * time.Minute, 3 * time.Hour}
		if !reflect.DeepEqual(durations, expected) {
			t.Fatalf("stream ParseDuration failed: got %v, want %v", durations, expected)
		}
	})

	t.Run("stream base64 roundtrip", func(t *testing.T) {
		items := []string{"apple", "banana", "cherry"}
		encoded := stream.Of(items).
			Map(func(s string) []byte { return []byte(s) }).
			Map(fn.ToBase64).
			Collect()

		decoded := stream.Of(encoded).
			Map(fn.FromBase64String).
			Filter(func(r adt.Result[string]) bool { return r.IsOK() }).
			Map(func(r adt.Result[string]) string { return r.MustGet() }).
			Collect()

		if !reflect.DeepEqual(items, decoded) {
			t.Fatalf("stream base64 roundtrip failed: got %v, want %v", decoded, items)
		}
	})
}


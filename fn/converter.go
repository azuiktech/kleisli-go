package fn

import (
	json "encoding/json/v2"
	"encoding/json/jsontext"

	"github.com/azuiktech/kleisli-go/adt"
)

// FromJSON parses JSON-encoded data into type T using encoding/json/v2.
func FromJSON[T any](data []byte) adt.Result[T] {
	var target T
	return adt.From(target, json.Unmarshal(data, &target))
}

// FromJSONOpts parses JSON-encoded data into type T with custom json.Options.
func FromJSONOpts[T any](data []byte, opts ...json.Options) adt.Result[T] {
	var target T
	return adt.From(target, json.Unmarshal(data, &target, opts...))
}

// FromJSONString parses a JSON string into type T using encoding/json/v2.
func FromJSONString[T any](s string) adt.Result[T] {
	return FromJSON[T]([]byte(s))
}

// ToJSON encodes v into JSON bytes using encoding/json/v2.
func ToJSON[T any](v T) adt.Result[[]byte] {
	return adt.From(json.Marshal(v))
}

// ToJSONOpts encodes v into JSON bytes with custom json.Options.
func ToJSONOpts[T any](v T, opts ...json.Options) adt.Result[[]byte] {
	return adt.From(json.Marshal(v, opts...))
}

// ToJSONIndent formats v with prefix and indent using encoding/json/v2.
func ToJSONIndent[T any](v T, prefix, indent string) adt.Result[[]byte] {
	return adt.From(json.Marshal(v, jsontext.WithIndentPrefix(prefix), jsontext.WithIndent(indent)))
}

// ToJSONString encodes v into a JSON string using encoding/json/v2.
func ToJSONString[T any](v T) adt.Result[string] {
	return ToJSON(v).Map(func(b []byte) string { return string(b) })
}

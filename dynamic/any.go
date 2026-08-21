// Package dynamic provides Any, a type-erased value that remembers enough
// to safely recover its concrete type later — Haskell's Data.Dynamic, or
// the same shape as protobuf's google.protobuf.Any (a type name plus a
// payload, resolved through a registry), adapted for JSON.
//
// Unlike Rust's std::any::Any (backed by a language-level TypeId, no
// registration needed) or Go's own reflect.Type (not stable across process
// restarts or a JSON round-trip), Any needs every concrete type it will
// ever hold to be Registered under a caller-chosen name first — JSON has
// no native concept of a stable, portable type identity.
package dynamic

import (
	json "encoding/json/v2"
	"encoding/json/jsontext"
	"fmt"
	"reflect"

	"github.com/azuiktech/kleisli-go/option"
)

var (
	typeToName = map[reflect.Type]string{}
	nameToType = map[string]reflect.Type{}
)

// Register makes T recoverable from an Any under name — call once,
// typically from an init(), before any New or UnmarshalJSON call: the
// registry has no locking, the same assumption gob.Register and protobuf's
// registry make. name is entirely the caller's choice, never derived from
// T itself, the same way a protobuf message's type URL comes from its
// package + name declaration.
func Register[T any](name string) {
	if _, exists := nameToType[name]; exists {
		panic(fmt.Sprintf("dynamic: type name %q already registered", name))
	}
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		panic("dynamic.Register: cannot register nil interface type")
	}
	typeToName[t] = name
	nameToType[name] = t
}


// Any holds a type-erased value of type T alongside the name it was
// Registered under. The zero value is empty (name "", holds no value).
type Any struct {
	name  string
	value any
}

// New constructs an Any from value — panics if value's concrete type was
// not Registered first. Calling New(nil) is safe (returns the empty Any).
func New(value any) Any {
	if value == nil {
		return Any{}
	}
	t := reflect.TypeOf(value)
	name, ok := typeToName[t]
	if !ok {
		panic(fmt.Sprintf("dynamic.New: type %v is not registered — call Register first", t))
	}
	return Any{name: name, value: value}
}

// Name returns the name value's concrete type was Registered under, or ""
// if empty.
func (d Any) Name() string { return d.name }

// Value returns the underlying erased value, or nil if empty.
func (d Any) Value() any { return d.value }

// IsZero reports whether d is empty.
func (d Any) IsZero() bool { return d.name == "" && d.value == nil }

// Get extracts the underlying value as T — returns option.Some(v) if d's
// held value has type T, option.None if empty or if held value is not a T.
func Get[T any](d Any) option.Option[T] {
	if v, ok := d.value.(T); ok {
		return option.Some(v)
	}
	return option.None[T]()
}

// As extracts the boxed value as T, returning None if the assertion fails or
// d holds no value — the Option-native alternative to a raw type assertion on
// Value().
func As[T any](d Any) option.Option[T] {
	v, ok := d.value.(T)
	return option.FromOk(v, ok)
}


type wireAny struct {
	Type  string `json:"@type"`
	Value any    `json:"value"`
}

// MarshalJSON writes the Any as {"@type":<name>,"value":<value>}. The zero
// value writes as null.
func (d Any) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(wireAny{Type: d.name, Value: d.value})
}

// UnmarshalJSON reads back the shape MarshalJSON writes — looks up @type in
// the registry, unmarshals value into a fresh T, and stores it. Errs (does
// not panic, matching UnmarshalJSON convention across the standard library,
// unlike New) if that name was never Registered.
func (d *Any) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = Any{}
		return nil
	}
	var wire struct {
		Type  string         `json:"@type"`
		Value jsontext.Value `json:"value"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	t, ok := nameToType[wire.Type]
	if !ok {
		return fmt.Errorf("dynamic: type name %q not registered", wire.Type)
	}
	ptr := reflect.New(t)
	if err := json.Unmarshal(wire.Value, ptr.Interface()); err != nil {
		return err
	}
	d.name, d.value = wire.Type, ptr.Elem().Interface()
	return nil
}

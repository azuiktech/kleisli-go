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
	"encoding/json"
	"fmt"
	"reflect"
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
// .proto package+name rather than its generated Go type. Register panics
// on a duplicate name, so a collision fails at init time rather than
// resolving to the wrong type at some later Unmarshal.
func Register[T any](name string) {
	if _, exists := nameToType[name]; exists {
		panic(fmt.Sprintf("dynamic: type name %q already registered", name))
	}
	t := reflect.TypeOf(*new(T))
	typeToName[t] = name
	nameToType[name] = t
}

// Any holds a value of Registered type, type-erased. Use New to construct;
// never use the zero value directly (though the zero value does happen to
// hold no value, the same way None[T]() does for Option).
type Any struct {
	name  string
	value any
}

// New boxes v, identified by its own runtime type. Panics if that type was
// never Registered — the same fail-fast stance Register itself takes, and
// the same shape as option.Some panicking on a nil value: a programmer
// error, not a condition to recover from.
func New(v any) Any {
	if v == nil {
		return Any{}
	}
	name, ok := typeToName[reflect.TypeOf(v)]
	if !ok {
		panic(fmt.Sprintf("dynamic: type %T not registered", v))
	}
	return Any{name: name, value: v}
}

// Value returns the boxed value, or nil if none was ever set.
func (d Any) Value() any { return d.value }

// wireAny is Any's JSON wire shape: {"@type":"<name>","value":<payload>}.
type wireAny struct {
	Type  string `json:"@type"`
	Value any    `json:"value"`
}

// MarshalJSON writes null for a zero-value Any (no value ever boxed), or
// {"@type":...,"value":...} otherwise.
func (d Any) MarshalJSON() ([]byte, error) {
	if d.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(wireAny{Type: d.name, Value: d.value})
}

// UnmarshalJSON reads null as a zero-value Any, and
// {"@type":...,"value":...} as a boxed value of the type "@type" names —
// an exact lookup, erroring (not panicking — this is untrusted input,
// unlike New) if that name was never Registered.
func (d *Any) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = Any{}
		return nil
	}
	var wire struct {
		Type  string          `json:"@type"`
		Value json.RawMessage `json:"value"`
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

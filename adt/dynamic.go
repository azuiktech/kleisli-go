package adt

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
// typically from an init(), before any Dyn or UnmarshalJSON call: the
// registry has no locking, the same assumption gob.Register and protobuf's
// registry make. Register panics on a duplicate name or if the same Go
// type is registered under a second, different name (which would leave
// the two maps inconsistent).
func Register[T any](name string) {
	if _, exists := nameToType[name]; exists {
		panic(fmt.Sprintf("adt: type name %q already registered", name))
	}
	t := reflect.TypeOf(*new(T))
	if existing, exists := typeToName[t]; exists {
		panic(fmt.Sprintf("adt: type %v already registered as %q; cannot also register as %q", t, existing, name))
	}
	typeToName[t] = name
	nameToType[name] = t
}

// Any holds a value of Registered type, type-erased. Use Dyn to construct.
type Any struct {
	name  string
	value any
}

// Dyn boxes v, identified by its own runtime type. Panics if that type was
// never Registered. Named Dyn (not New) to avoid the type/constructor name
// collision: Go does not allow a function and a type to share the same
// identifier at package scope.
func Dyn(v any) Any {
	if v == nil {
		return Any{}
	}
	name, ok := typeToName[reflect.TypeOf(v)]
	if !ok {
		panic(fmt.Sprintf("adt: type %T not registered", v))
	}
	return Any{name: name, value: v}
}

// Value returns the boxed value, or nil if none was ever set.
func (d Any) Value() any { return d.value }

// As extracts the boxed value as T, returning None if the assertion fails or
// d holds no value.
func As[T any](d Any) Option[T] {
	v, ok := d.value.(T)
	return FromOk(v, ok)
}

type wireAny struct {
	Type  string `json:"@type"`
	Value any    `json:"value"`
}

// MarshalJSON writes null for a zero-value Any, or {"@type":...,"value":...}.
func (d Any) MarshalJSON() ([]byte, error) {
	if d.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(wireAny{Type: d.name, Value: d.value})
}

// UnmarshalJSON reads null as a zero-value Any, and {"@type":...,"value":...}
// as a boxed value — erroring (not panicking) if the type name was never Registered.
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
		return fmt.Errorf("adt: type name %q not registered", wire.Type)
	}
	ptr := reflect.New(t)
	if err := json.Unmarshal(wire.Value, ptr.Interface()); err != nil {
		return err
	}
	d.name, d.value = wire.Type, ptr.Elem().Interface()
	return nil
}

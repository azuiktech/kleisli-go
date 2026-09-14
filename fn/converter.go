package fn

import (
	"encoding/base64"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/azuiktech/kleisli-go/adt"
)

// ── JSON converters ───────────────────────────────────────────────────────────

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

// ── Numeric types & parsers ───────────────────────────────────────────────────

// Number is the set of Go's built-in numeric types.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// ParseInt parses a decimal integer string into an int.
func ParseInt(s string) adt.Result[int] {
	return adt.From(strconv.Atoi(s))
}

// ParseInt64 parses a decimal integer string into an int64.
func ParseInt64(s string) adt.Result[int64] {
	return adt.From(strconv.ParseInt(s, 10, 64))
}

// ParseUint parses a decimal unsigned integer string into a uint.
func ParseUint(s string) adt.Result[uint] {
	v, err := strconv.ParseUint(s, 10, 0)
	return adt.From(uint(v), err)
}

// ParseUint64 parses a decimal unsigned integer string into a uint64.
func ParseUint64(s string) adt.Result[uint64] {
	return adt.From(strconv.ParseUint(s, 10, 64))
}

// ParseFloat parses a floating-point number string into a float64.
func ParseFloat(s string) adt.Result[float64] {
	return adt.From(strconv.ParseFloat(s, 64))
}

// ParseFloat32 parses a floating-point number string into a float32.
func ParseFloat32(s string) adt.Result[float32] {
	v, err := strconv.ParseFloat(s, 32)
	return adt.From(float32(v), err)
}

// parseSigned parses s as a bitSize-bit signed integer and narrows it to N.
func parseSigned[N Number](s string, bitSize int) adt.Result[N] {
	v, err := strconv.ParseInt(s, 10, bitSize)
	return adt.From(N(v), err)
}

// parseUnsigned parses s as a bitSize-bit unsigned integer and narrows it to N.
func parseUnsigned[N Number](s string, bitSize int) adt.Result[N] {
	v, err := strconv.ParseUint(s, 10, bitSize)
	return adt.From(N(v), err)
}

// Parse parses s into any numeric type N with bit-size range enforcement.
func Parse[N Number](s string) adt.Result[N] {
	var zero N
	switch any(zero).(type) {
	case int:
		return ParseInt(s).Map(func(v int) N { return N(v) })
	case int8:
		return parseSigned[N](s, 8)
	case int16:
		return parseSigned[N](s, 16)
	case int32:
		return parseSigned[N](s, 32)
	case int64:
		return ParseInt64(s).Map(func(v int64) N { return N(v) })
	case uint:
		return ParseUint(s).Map(func(v uint) N { return N(v) })
	case uint8:
		return parseUnsigned[N](s, 8)
	case uint16:
		return parseUnsigned[N](s, 16)
	case uint32:
		return parseUnsigned[N](s, 32)
	case uint64:
		return ParseUint64(s).Map(func(v uint64) N { return N(v) })
	case float32:
		return ParseFloat32(s).Map(func(v float32) N { return N(v) })
	case float64:
		return ParseFloat(s).Map(func(v float64) N { return N(v) })
	default:
		return adt.Err[N](fmt.Errorf("fn.Parse: unsupported numeric type %T", zero))
	}
}

// ── Time & Duration parsers ───────────────────────────────────────────────────

// ParseDuration parses a duration string into a time.Duration.
func ParseDuration(s string) adt.Result[time.Duration] {
	return adt.From(time.ParseDuration(s))
}

// ParseTime parses a formatted time string using layout into a time.Time.
func ParseTime(layout, value string) adt.Result[time.Time] {
	return adt.From(time.Parse(layout, value))
}

// ParseRFC3339 parses an RFC3339/ISO8601 time string into a time.Time.
func ParseRFC3339(value string) adt.Result[time.Time] {
	return ParseTime(time.RFC3339, value)
}

// ── Base64 encoding & decoding ────────────────────────────────────────────────

// ToBase64 encodes data to standard base64 string.
func ToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// FromBase64 decodes a standard base64 string into bytes.
func FromBase64(s string) adt.Result[[]byte] {
	return adt.From(base64.StdEncoding.DecodeString(s))
}

// FromBase64String decodes a standard base64 string into a plain string.
func FromBase64String(s string) adt.Result[string] {
	return FromBase64(s).Map(func(b []byte) string { return string(b) })
}

// ToBase64URL encodes data to unpadded URL-safe base64 string.
func ToBase64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// FromBase64URL decodes an unpadded URL-safe base64 string into bytes.
func FromBase64URL(s string) adt.Result[[]byte] {
	return adt.From(base64.RawURLEncoding.DecodeString(s))
}

// FromBase64URLString decodes an unpadded URL-safe base64 string into a plain string.
func FromBase64URLString(s string) adt.Result[string] {
	return FromBase64URL(s).Map(func(b []byte) string { return string(b) })
}

// ── URL & Query encoding ──────────────────────────────────────────────────────

// ToQueryEscape escapes s for use in a URL query string.
func ToQueryEscape(s string) string {
	return url.QueryEscape(s)
}

// FromQueryUnescape unescapes a URL query string.
func FromQueryUnescape(s string) adt.Result[string] {
	return adt.From(url.QueryUnescape(s))
}

// ToPathEscape escapes s for use in a URL path segment.
func ToPathEscape(s string) string {
	return url.PathEscape(s)
}

// FromPathUnescape unescapes a URL path segment.
func FromPathUnescape(s string) adt.Result[string] {
	return adt.From(url.PathUnescape(s))
}

// FromQueryString parses a URL query string into url.Values.
func FromQueryString(raw string) adt.Result[url.Values] {
	return adt.From(url.ParseQuery(raw))
}

// ToQueryString encodes url.Values into a query string.
func ToQueryString(v url.Values) string {
	return v.Encode()
}

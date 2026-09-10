# Chapter 1: Algebraic Data Types (`adt`)

The `adt` package consolidates foundational generic algebraic data types: `Result[T]`, `Option[T]`, `Unit`/`Void`, `Lazy[T]`, and `Any`. It replaces fragmented implementations, eliminates repetitive package naming (`result.Result`, `option.Option`), and enables clean generic method chaining across type boundaries (`Map[U]`, `FlatMap[U]`, `Then[U]`).

```go
import "github.com/azuiktech/kleisli-go/adt"
```

---

## 1. Result[T]

`Result[T]` models the outcome of a fallible computation, holding either a success value of type `T` or a non-nil `error`. The two states are mutually exclusive by construction.

### Type Definition

```go
type Result[T any] struct { /* unexported fields */ }
```

### Constructors

| Function | Signature | Description |
|---|---|---|
| `OK[T]` | `func OK[T any](val T) Result[T]` | Wraps a successful value. |
| `Err[T]` | `func Err[T any](err error) Result[T]` | Wraps an error. Panics if `err == nil`. |
| `From[T]` | `func From[T any](val T, err error) Result[T]` | Converts standard Go `(val, error)` multi-return into `Result[T]`. If `err != nil`, returns `Err`, else `OK(val)`. |
| `FromNonZero[T]` | `func FromNonZero[T comparable](val T, err error) Result[T]` | Returns `OK(val)` if `err == nil` and `val` is not the zero value of `T`. Panics if both `err == nil` and `val == zero`. |

```go
// Wrapping standard returns
res := adt.From(os.ReadFile("config.json"))

// Manual construction
ok := adt.OK(42)
fail := adt.Err[int](errors.New("not found"))
```

### State Inspection & Value Extraction

| Method | Signature | Description |
|---|---|---|
| `IsOK()` | `func (r Result[T]) IsOK() bool` | Returns `true` if success (`err == nil`). |
| `IsErr()` | `func (r Result[T]) IsErr() bool` | Returns `true` if failure (`err != nil`). |
| `Unwrap()` | `func (r Result[T]) Unwrap() (T, error)` | Deconstructs into standard Go `(T, error)` pair. |
| `MustGet()` | `func (r Result[T]) MustGet() T` | Returns `val` on success. Panics with the underlying error if failure. |
| `MustErr()` | `func (r Result[T]) MustErr() error` | Returns `err` on failure. Panics if success. |
| `Expect(msg)` | `func (r Result[T]) Expect(msg string) T` | Returns `val` on success. Panics formatted as `"<msg>: <err>"` if failure. |
| `OrElse(fallback)` | `func (r Result[T]) OrElse(fallback T) T` | Returns `val` on success, or `fallback` on failure. |
| `OrElseGet(fn)` | `func (r Result[T]) OrElseGet(fn func(error) T) T` | Returns `val` on success, or computes fallback via `fn(err)`. |
| `Or(other)` | `func (r Result[T]) Or(other Result[T]) Result[T]` | Returns `r` if success, otherwise returns `other`. |

### Monadic Chaining & Transformation

| Method | Signature | Description |
|---|---|---|
| `Map[U](fn)` | `func (r Result[T]) Map[U any](fn func(T) U) Result[U]` | Transforms success value `T -> U`. Preserves error if failed. |
| `Map0[U](fn)` | `func (r Result[T]) Map0[U any](fn func(T) (U, error)) Result[U]` | Transforms success with a fallible Go function `func(T) (U, error)`. |
| `FlatMap[U](fn)`| `func (r Result[T]) FlatMap[U any](fn func(T) Result[U]) Result[U]` | Chains sequential fallible operation returning `Result[U]`. |
| `Then[U](fn)` | `func (r Result[T]) Then[U any](fn func(T) (U, error)) Result[U]` | Alias for `Map0`. Bridges seamlessly with Go functions returning `(U, error)`. |
| `Recover(fn)` | `func (r Result[T]) Recover(fn func(error) Result[T]) Result[T]` | Recovers from an error by returning an alternative `Result[T]`. |
| `Fold[U](onOK, onErr)` | `func (r Result[T]) Fold[U any](onOK func(T) U, onErr func(error) U) U` | Exhaustively unifies both branches into a single type `U`. |

```go
dtoResult := adt.From(fetchUser(id)).
    Map(User.ToDTO).
    FlatMap(validateDTO).
    Recover(fallbackGuestDTO)

// Dispatch cleanly at boundary using Fold:
dtoResult.Fold(renderSuccessJSON, renderErrorJSON)
```

### Error Handling & Side Effects

| Method | Signature | Description |
|---|---|---|
| `MapErr(fn)` | `func (r Result[T]) MapErr(fn func(error) error) Result[T]` | Replaces or maps the error using `fn`. |
| `MapErrf(format, args...)` | `func (r Result[T]) MapErrf(format string, args ...any) Result[T]` | Wraps error using `fmt.Errorf` (appends `: %w` if missing). |
| `WrapErr(format, args...)` | `func (r Result[T]) WrapErr(format string, args ...any) Result[T]` | Alias for `MapErrf`. |
| `Tap(fn)` | `func (r Result[T]) Tap(fn func(T)) Result[T]` | Runs side effect on success value; returns `r` unchanged. |
| `TapErr(fn)` | `func (r Result[T]) TapErr(fn func(error)) Result[T]` | Runs side effect on failure error; returns `r` unchanged. |
| `ToOption()` | `func (r Result[T]) ToOption() Option[T]` | Converts to `Option[T]`: `OK(v)` -> `Some(v)`, `Err` -> `None`. |

### Slice & Free Functions (`Results` Namespace)

| Function | Signature | Description |
|---|---|---|
| `Successes[T]` | `func Successes[T any](results []Result[T]) []T` | Filters slice to only success values. |
| `Failures[T]` | `func Failures[T any](results []Result[T]) []error` | Filters slice to only error values. |
| `Results.Zip2` | `func (resultsNamespace) Zip2[A, B, U any](ra Result[A], rb Result[B], fn func(A, B) U) Result[U]` | Combines two results with `fn` if both succeed. |
| `Results.Zip3` | `func (resultsNamespace) Zip3[A, B, C, U any](ra Result[A], rb Result[B], rc Result[C], fn func(A, B, C) U) Result[U]` | Combines three results with `fn` if all succeed. |
| `Results.Flatten` | `func (resultsNamespace) Flatten[T any](r Result[Result[T]]) Result[T]` | Flattens nested `Result[Result[T]]` to `Result[T]`. |
| `Results.Contains` | `func (resultsNamespace) Contains[T comparable](r Result[T], val T) bool` | Reports whether result is OK and equals `val`. |
| `Results.Sequence` | `func (resultsNamespace) Sequence[T any](results []Result[T]) Result[[]T]` | Inverts `[]Result[T]` to `Result[[]T]`. Fails fast on first error. |

### JSON Serialization

Serializes using an externally tagged JSON format: `{"ok": <value>}` or `{"err": "<message>"}`.

```go
jsonStr := fn.ToJSONString(adt.OK("success")).MustGet()        // {"ok":"success"}
jsonStr = fn.ToJSONString(adt.Err[string](errFail)).MustGet() // {"err":"operation failed"}
```

---

## 2. Option[T]

`Option[T]` models optionality without nil pointers or loose `(T, bool)` pairs. An option is either `Some(v)` (present) or `None` (absent).

### Type Definition

```go
type Option[T any] struct { /* unexported fields */ }
```

### Constructors

| Function | Signature | Description |
|---|---|---|
| `Some[T](val)` | `func Some[T any](val T) Option[T]` | Wraps a present value (including typed nil pointers). |
| `None[T]()` | `func None[T any]() Option[T]` | Returns an empty/absent Option. |
| `Opt[T](val)` | `func Opt[T any](val T) Option[T]` | Converts value to Option: if `val` is nil (pointer, slice, map, channel, func, interface), returns `None`, otherwise `Some(val)`. |
| `FromMap(m, key)` | `func FromMap[K comparable, V any](m map[K]V, key K) Option[V]` | Looks up `key` in map `m`. Returns `Some(val)` if present and non-nil. |
| `FromOk(val, ok)` | `func FromOk[T any](val T, ok bool) Option[T]` | Converts `(val, ok)` idiom (map lookup, type assertion, channel receive). |
| `OptNonZero(val)` | `func OptNonZero[T comparable](val T) Option[T]` | Returns `Some(val)` if `val != zero`, else `None`. |
| `FromSlice(s, i)` | `func FromSlice[T any](slice []T, idx int) Option[T]` | Safe slice index access: returns `Some(s[idx])` if in bounds, else `None`. |
| `FromResult(r)` | `func FromResult[T any](r Result[T]) Option[T]` | Converts `Result[T]` to `Option[T]` (`OK` -> `Some`, `Err` -> `None`). |

### State Inspection & Extraction

| Method | Signature | Description |
|---|---|---|
| `IsSome()` | `func (o Option[T]) IsSome() bool` | Returns `true` if value is present. |
| `IsNone()` | `func (o Option[T]) IsNone() bool` | Returns `true` if value is absent. |
| `Unwrap()` | `func (o Option[T]) Unwrap() (T, bool)` | Returns `(val, ok)` pair. |
| `MustGet()` | `func (o Option[T]) MustGet() T` | Returns `val` if present; panics if `None`. |
| `Expect(msg)` | `func (o Option[T]) Expect(msg string) T` | Returns `val` if present; panics with `msg` if `None`. |
| `OrElse(fallback)` | `func (o Option[T]) OrElse(fallback T) T` | Returns `val` if present, or `fallback`. |
| `OrElseGet(fn)` | `func (o Option[T]) OrElseGet(fn func() T) T` | Returns `val` if present, or evaluates `fn()`. |
| `Or(other)` | `func (o Option[T]) Or(other Option[T]) Option[T]` | Returns `o` if `Some`, otherwise returns `other`. |
| `ToPtr()` | `func (o Option[T]) ToPtr() *T` | Returns pointer to value, or `nil` if absent. |
| `ToSlice()` | `func (o Option[T]) ToSlice() []T` | Returns single-element slice `[]T{val}` if present, or empty slice `[]T{}`. |
| `All()` | `func (o Option[T]) All() iter.Seq[T]` | Returns iterator yielding the value once if present, or zero times if absent. |
| `ToResult(err)` | `func (o Option[T]) ToResult(err error) Result[T]` | Converts to `Result[T]`: `Some(v)` -> `OK(v)`, `None` -> `Err(err)`. |
| `ToResultGet(fn)` | `func (o Option[T]) ToResultGet(fn func() error) Result[T]` | Converts to `Result[T]`, lazily evaluating `fn()` only if `None`. |

### Monadic Chaining & Transformation

| Method | Signature | Description |
|---|---|---|
| `Map[U](fn)` | `func (o Option[T]) Map[U any](fn func(T) U) Option[U]` | Transforms present value `T -> U`. |
| `Map0[U](fn)` | `func (o Option[T]) Map0[U any](fn func(T) (U, bool)) Option[U]` | Transforms using a fallible `(U, bool)` function. |
| `FlatMap[U](fn)` | `func (o Option[T]) FlatMap[U any](fn func(T) Option[U]) Option[U]` | Chains sequential operation returning `Option[U]`. |
| `Then[U](fn)` | `func (o Option[T]) Then[U any](fn func(T) (U, bool)) Option[U]` | Alias for `Map0`. |
| `Filter(fn)` | `func (o Option[T]) Filter(fn func(T) bool) Option[T]` | Keeps value only if `fn(val)` is true, otherwise `None`. |
| `Tap(fn)` | `func (o Option[T]) Tap(fn func(T)) Option[T]` | Invokes side-effect if present; returns `o` unchanged. |
| `Fold[U](some, none)` | `func (o Option[T]) Fold[U any](onSome func(T) U, onNone func() U) U` | Exhaustively unifies both branches into `U`. |

### Slice & Free Functions (`Options` Namespace)

| Function | Signature | Description |
|---|---|---|
| `Somes[T]` | `func Somes[T any](opts []Option[T]) []T` | Extracts only present values from a slice of options. |
| `Options.Zip2` | `func (optionsNamespace) Zip2[A, B, U any](oa Option[A], ob Option[B], fn func(A, B) U) Option[U]` | Combines two options if both are present. |
| `Options.Zip3` | `func (optionsNamespace) Zip3[A, B, C, U any](oa Option[A], ob Option[B], oc Option[C], fn func(A, B, C) U) Option[U]` | Combines three options if all are present. |
| `Options.Flatten` | `func (optionsNamespace) Flatten[T any](o Option[Option[T]]) Option[T]` | Flattens nested `Option[Option[T]]` to `Option[T]`. |
| `Options.Contains` | `func (optionsNamespace) Contains[T comparable](o Option[T], val T) bool` | Reports whether option is `Some` and equals `val`. |
| `Options.Sequence` | `func (optionsNamespace) Sequence[T any](opts []Option[T]) Option[[]T]` | Inverts `[]Option[T]` to `Option[[]T]`. Returns `None` if any element is `None`. |

### JSON Serialization

Serializes directly to the inner JSON value if present, or `null` if absent.

```go
jsonStr := fn.ToJSONString(adt.Some("hello")).MustGet() // "hello"
jsonStr = fn.ToJSONString(adt.None[string]()).MustGet() // null
```

---

## 3. Unit and Void

Represents void/empty outcomes cleanly without arbitrary structs.

```go
type Unit = struct{}
var Void = Unit{}
```

- Return `adt.OK(adt.Void)` for fallible operations with no return payload (`Result[Unit]`).
- Return `adt.Some(adt.Void)` for flags or presence signals (`Option[Unit]`).

---

## 4. Lazy[T]

`Lazy[T]` defers computation of a value until first access (`Get()`). Thread-safe and memoized using Go's `sync.OnceValue`.

### Type Definition & Constructors

```go
type Lazy[T any] struct { /* unexported fields */ }
```

| Function / Method | Signature | Description |
|---|---|---|
| `Defer[T](fn)` | `func Defer[T any](fn func() T) Lazy[T]` | Constructs lazy value computed at most once by `fn`. |
| `DeferErr[T](fn)` | `func DeferErr[T any](fn func() (T, error)) Lazy[Result[T]]` | Constructs lazy `Result[T]` from `(T, error)` function. |
| `Get()` | `func (l Lazy[T]) Get() T` | Evaluates `fn` on first call; returns cached result subsequently. |
| `Map[U](fn)` | `func (l Lazy[T]) Map[U any](fn func(T) U) Lazy[U]` | Transforms lazy value without evaluating until `Get()`. |
| `FlatMap[U](fn)` | `func (l Lazy[T]) FlatMap[U any](fn func(T) Lazy[U]) Lazy[U]` | Lazily chains deferred computations. |
| `ToOption()` | `func (l Lazy[T]) ToOption() Option[T]` | Evaluates and wraps in `Option[T]`. |
| `ToResult(err)` | `func (l Lazy[T]) ToResult(err error) Result[T]` | Evaluates and converts via `ToOption().ToResult(err)`. |
| `ToResultGet(fn)` | `func (l Lazy[T]) ToResultGet(fn func() error) Result[T]` | Evaluates and converts with lazy error producer. |

### Memoization Utilities

| Function | Signature | Description |
|---|---|---|
| `Memoize[K, V](fn)` | `func Memoize[K comparable, V any](fn func(K) V) func(K) V` | Returns a thread-safe memoized version of `fn` cached per key `K`. |
| `MemoizeErr[K, V](fn)` | `func MemoizeErr[K comparable, V any](fn func(K) (V, error)) func(K) (V, error)` | Thread-safe memoization caching both value and error per key `K`. |

```go
expensiveOp := adt.Memoize(func(path string) string {
    return parseTemplate(path)
})
result1 := expensiveOp("file.txt") // executes
result2 := expensiveOp("file.txt") // instant cache hit
```

---

## 5. Any (Dynamic Type-Erased Box)

`Any` holds a type-erased value of an explicitly registered type, allowing safe runtime dynamic typing and polymorphic JSON serialization.

### Type Definition & Registry

```go
type Any struct { /* unexported fields */ }
```

| Function / Method | Signature | Description |
|---|---|---|
| `Register[T](name)` | `func Register[T any](name string)` | Registers type `T` under a unique string identifier (call in `init()`). |
| `Dyn(v)` | `func Dyn(v any) Any` | Boxes `v` into an `Any`. Panics if `v`'s concrete type was not registered. |
| `Value()` | `func (d Any) Value() any` | Returns underlying `any` value. |
| `As[T](d)` | `func As[T any](d Any) Option[T]` | Extracts boxed value as `T`. Returns `Some(val)` if type matches, else `None`. |

### Dynamic JSON Polymorphism

Serializes to JSON with an explicit type discriminator: `{"@type": "<name>", "value": <data>}`.

```go
func init() {
    adt.Register[LoginEvent]("auth.login")
}

ev := adt.Dyn(LoginEvent{UserID: "u123"})

// Serialize using the library's JSON wrapper returning Result[[]byte]
bytes := fn.ToJSON(ev).MustGet()
// Wire JSON: {"@type":"auth.login","value":{"UserID":"u123"}}

// Deserialize back to adt.Any via Result wrapper, then extract type-safely
restored := fn.FromJSON[adt.Any](bytes).MustGet()
loginEvent := adt.As[LoginEvent](restored).MustGet()
```

---

## 6. Practical Real-World Scenarios

### Scenario A: Multi-Field Configuration Parser (`Results.Zip3`, `Recover`)

Parsing configuration from external inputs where missing fields or bad formats produce errors, and fallbacks provide defaults:

```go
type DBConfig struct {
    Host string
    Port int
    User string
}

func parsePort(s string) adt.Result[int] {
    return fn.ParseInt(s).MapErrf("invalid port %q", s)
}

func validateHost(host string) adt.Result[string] {
    return adt.OK(host).Filter(fn.Not(fn.IsSpace)).
        ToResult(errors.New("host cannot be blank"))
}

func LoadDatabaseConfig(env map[string]string) adt.Result[DBConfig] {
    hostRes := adt.FromMap(env, "DB_HOST").ToResult(errors.New("DB_HOST missing")).FlatMap(validateHost)
    portRes := adt.FromMap(env, "DB_PORT").ToResult(errors.New("DB_PORT missing")).FlatMap(parsePort)
    userRes := adt.FromMap(env, "DB_USER").ToResult(errors.New("DB_USER missing"))

    // Combine three fallible results into DBConfig:
    return adt.Results.Zip3(hostRes, portRes, userRes, func(h string, p int, u string) DBConfig {
        return DBConfig{Host: h, Port: p, User: u}
    }).Recover(func(err error) adt.Result[DBConfig] {
        // Fall back to safe local developer configuration
        return adt.OK(DBConfig{Host: "localhost", Port: 5432, User: "postgres"})
    })
}
```

### Scenario B: Deeply Nested Optional Navigation (`Option[T]`)

Navigating multi-level hierarchical structures (Organization $\to$ Department $\to$ Manager $\to$ Phone) without nested nil checks:

```go
type Employee struct {
    name  string
    phone *string
}

func (e Employee) Phone() *string { return e.phone }

type Department struct {
    manager *Employee
}

func (d Department) Manager() *Employee { return d.manager }

func FindManagerPhone(orgChart map[string]Department, deptName string) string {
    return adt.FromMap(orgChart, deptName).
        Map(Department.Manager).
        FlatMap(adt.Opt).
        Map(Employee.Phone).
        FlatMap(adt.Opt).
        OrElse("No Direct Line")
}
```

### Scenario C: All-or-Nothing Batch Validation (`Results.Sequence`)

Validating a slice of input commands before executing a database transaction:

```go
type CreateAccountCmd struct {
    Email string
    Age   int
}

func (cmd CreateAccountCmd) Validate() adt.Result[CreateAccountCmd] {
    if !strings.Contains(cmd.Email, "@") {
        return adt.Err[CreateAccountCmd](fmt.Errorf("invalid email: %s", cmd.Email))
    }
    if cmd.Age < 18 {
        return adt.Err[CreateAccountCmd](fmt.Errorf("underage user: %d", cmd.Age))
    }
    return adt.OK(cmd)
}

func ValidateBatch(commands []CreateAccountCmd) adt.Result[[]CreateAccountCmd] {
    // Validate each command, producing []Result[CreateAccountCmd]
    validationResults := stream.Of(commands).
        Map(CreateAccountCmd.Validate).
        Collect()

    // Inverts []Result[T] -> Result[[]T], short-circuiting on the first error
    return adt.Results.Sequence(validationResults)
}
```


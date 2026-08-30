# kleisli-go

Small generic utilities for Go 1.27+. Five packages cover the full surface:

| Package | Contents |
|---|---|
| `adt` | `Result[T]`, `Option[T]`, `Unit`/`Void`, `Lazy[T]`, `Any` |
| `fn` | value transforms, `Fn`/`Fn2` point-free composition, rotated-sequence algorithms, memoization |
| `stream` | `Stream[T]` (eager) and `Seq[T]` (lazy) slice/iterator pipelines |
| `async` | `Pipe[T]` CSP pipelines, `Task[T]`, `Promise`, `Sync[T]`, `Handle[T]`, `Ctx[T]` |
| `ds` | `RingBuffer[T]`, `SyncRingBuffer[T]` |

All types use Go 1.27 generic method type parameters, so transformations that
change type (`Map[U]`, `FlatMap[U]`, `Then[U]`, …) are plain chained method
calls rather than free functions or wrapper types.

The name comes from the [Kleisli category](https://en.wikipedia.org/wiki/Kleisli_category)
— the category of monadic functions — which is exactly what `Result`'s and
`Option`'s `FlatMap`/`Then` compose.

```go
import (
    "github.com/azuiktech/kleisli-go/adt"
    "github.com/azuiktech/kleisli-go/fn"
    "github.com/azuiktech/kleisli-go/stream"
    "github.com/azuiktech/kleisli-go/async"
    "github.com/azuiktech/kleisli-go/ds"
)
```

---

## adt

`adt` consolidates Result, Option, Unit, Lazy, and Any — the algebraic data
types — into one package, eliminating the package-name repetition
(`result.Result`, `option.Option`) and the circular dependency that previously
blocked symmetric Result↔Option conversions.

### Result[T]

`Result[T]` holds either a success value or an error, with combinators for
chaining fallible operations without repeated `if err != nil` checks.

```go
user, err := adt.From(verifier.Verify(ctx, token)).
    MapErrf("verify token %q", token).
    Then(upsertUser).
    FlatMap(ensurePlan).
    Then(buildDTO).
    Unwrap()
```

Full API: `OK`, `Err`, `From`, `FromNonZero`, `Unwrap`, `MustGet`, `MustErr`,
`Expect`, `OrElse`, `OrElseGet`, `Or`, `MapErr`, `MapErrf`, `WrapErr`,
`Recover`, `Tap`, `TapErr`, `Map`, `Map0`, `FlatMap`, `Then`, `Fold`,
`ToOption` — plus `Successes`, `Failures` for slices, and
`adt.Results.{Zip2, Zip3, Flatten, Contains, Sequence}` for the free
functions that share names with their Option equivalents.

### Option[T]

`Option[T]` holds either a present, non-nil value or nothing — the
replacement for a nil-pointer check or a hand-rolled `(T, bool)` pair.
`Some` refuses a nil value (nil and absent are the same concept), so JSON
serialises as the value itself or `null`.

```go
var p *User
name := adt.Opt(p).
    Map(func(u *User) string { return u.Name }).
    OrElse("")

userOpt := adt.FromMap(usersByID, id)
tokenOpt := adt.OptNonZero(cfg.AuthToken)
```

Full API: `Some`, `None`, `Opt`, `FromMap`, `FromOk`, `OptNonZero`,
`FromSlice`, `FromResult`, `ToPtr`, `ToResult`, `ToResultGet`, `ToSlice`,
`IsSome`, `IsNone`, `Unwrap`, `MustGet`, `Expect`, `OrElse`, `OrElseGet`,
`Or`, `Filter`, `Tap`, `Fold`, `Map`, `Map0`, `FlatMap`, `Then` — plus
`Somes` for slices, and
`adt.Options.{Zip2, Zip3, Flatten, Contains, Sequence}`.

**Naming note** — Go forbids a function and a type from sharing the same
identifier, so a few names differ from what the old separate packages used:

| Old | New | Reason |
|---|---|---|
| `option.From(val)` | `adt.Opt(val)` | Collision with `adt.From(val, err)` (Result) |
| `option.FromNonZero(val)` | `adt.OptNonZero(val)` | Same collision |
| `lazy.New(fn)` | `adt.Defer(fn)` | Type `Lazy` and func can't share `New` |
| `dynamic.New(v)` | `adt.Dyn(v)` | Type `Any` and func can't share `New` |
| `result.Void()` / `option.Void()` | `adt.Void` (var) | Single pre-created `Unit{}` value |

### Unit / Void

```go
return adt.OK(adt.Void)   // Result[Unit] carrying no value
return adt.Some(adt.Void) // Option[Unit] carrying no value
```

### Lazy[T]

Thread-safe, memoized zero-argument lazy computation backed by `sync.OnceValue`.

```go
user := adt.DeferErr(func() (*User, error) { return fetchUser(id) })
name := user.Get().Map(func(u *User) string { return u.Name }).OrElse("Guest")
```

Full API: `Defer`, `DeferErr`, `Get`, `Map`, `FlatMap`, `ToOption`, `ToResult`,
`ToResultGet`.

### Any

Type-erased value that remembers its concrete type for safe recovery via `As`.
Types must be `Register`ed once (typically in `init`) before use.

```go
adt.Register[MyEvent]("my-event")
a := adt.Dyn(MyEvent{…})
e := adt.As[MyEvent](a) // Option[MyEvent]
```

### Memoize

`adt.Memoize` and `adt.MemoizeErr` provide thread-safe per-key function
memoization.

---

## fn

`fn` consolidates stateless pure function utilities: value transforms, point-free
composition, rotated-sequence algorithms, and memoization.

```go
port   := fn.Deref(config.Port, 8080)
name   := fn.Cond(user != nil, user.Name, "Guest")
data   := fn.Must(os.ReadFile(path))
secret := fn.Pipe(rawKey, normalise)
```

**Value helpers:** `Must`, `WrapErr`, `MapErr`, `Fallback`, `FallbackGet`,
`Cond`, `CondGet`, `Ptr`, `Deref`, `DerefGet`, `DerefZero`, `Tap`, `Pipe`,
`Zero`, `IsZero`, `Clamp`, `Coalesce`.

**Point-free composition:**

```go
var StdDev = fn.Fn2[float64, float64, float64](subtract).
    Fork(fn.Identity[float64], mean).
    Then(square).Then(mean).Then(sqrt)

StdDev([]float64{2, 4, 4, 4, 5, 5, 7, 9}) // 2
```

`Fn[T,U]` and `Fn2[A,B,U]` are named function types; `Then` composes
left-to-right; `Fork` is the array-language fork train
(`combine.Fork(f, g)(x) = combine(f(x), g(x))`).

**Rotated-sequence algorithms** — designed to align with `ds.RingBuffer.Segments()`:

```go
first, second := ring.Segments()
n := fn.FindRotated(first, second, lastID, compare)
first, second = fn.After(first, second, n)
```

Full API: `FindRotated`, `After`, `Before`, `FindRotatedWithPivot`,
`AfterWithPivot`, `BeforeWithPivot`.

**Memoize:** `Memoize`, `MemoizeErr` (also available on `adt`).

---

## stream

`Stream[T]` wraps a slice for eager, chainable pipeline operations.
`Seq[T]` is its lazy, pull-based counterpart wrapping `iter.Seq[T]` — the
two share one implementation for every operation they both offer, so a `Seq`
pipeline genuinely short-circuits while a `Stream` built from the same call
stays eager.

```go
totals := stream.Of(invoices).
    Filter(func(inv Invoice) bool { return inv.Status == Unpaid }).
    GroupBy(func(inv Invoice) string { return inv.ClientID })

found, ok := stream.FromSeq(gen).Filter(isValid).First(matchesQuery)
```

`Stream` additionally offers operations that need the whole sequence:
`Reverse`, `SortBy` family, `GroupBy`, `ToMap`, `Partition`, `Last`, `Len`.

`Region`/`Parallel` bridge `Stream` to `async.Pipe` for bounded-concurrency
stages without leaving the chain:

```go
fetched := stream.Of(urls).Parallel(8, fetchAndParse).Collect()
```

---

## async

`Pipe[T]` wraps a channel for CSP-style pipeline composition — worker pools,
rate limiting, fan-out/fan-in, batching.

```go
total := async.From(urls).Parallel(8, fetchAndParse).Reduce(0, sum)
```

**`Task[O]` & `Promise`** — generic C++/C#/Rust-style Promise/Task coroutine system with
channel-backed suspension. Suspendable functions take `p *async.Promise` as their first parameter
and return standard Go types `O`. Inside the computation, `async.Receive[T](p)` or `async.Yield[T](p, val)`
suspends until the caller provides input. Callers drive execution via `task.Send(val)` and can
await results with `task.Await()` or register non-blocking callbacks (`task.OnDone`, `task.OnEmit`).

```go
// 1. Suspendable Function taking Promise context
task := async.Launch(ctx, func(p *async.Promise) string {
    // Atomic Emit + Receive (or async.Receive[string](p))
    name := async.Yield[string](p, "What is your name?")
    return "Hello, " + name
})

// 2. Caller fulfills externally (timer, webhook, user input)
task.OnEmit(func(prompt any) { fmt.Println("Prompt:", prompt) })
task.Send("Gopher")

// 3. Await final result (or use OnDone callback)
res := task.Await().MustGet() // "Hello, Gopher"
```

**`Sync[T]`** — concurrent state behind a `sync.RWMutex`, with `Read`,
`Write`, `Map`, and `Mutate`.

**`Handle[D]`** — value type wrapping `*Sync[D]`, enabling the pimpl pattern:
multiple struct types embed `Handle[D]` and expose different method sets over
the same shared state, with no explicit interface declaration needed.

```go
type orgState struct { orgs map[uuid.UUID]Org }

type InMemOrgs    struct{ async.Handle[orgState] }
type InMemMembers struct{ async.Handle[orgState] } // same shared state

h       := async.NewHandle(orgState{orgs: make(map[uuid.UUID]Org)})
orgs    := InMemOrgs{h}
members := InMemMembers{h}
```

**`Ctx[T]`** — couples any value with a `context.Context` for types that lack
a native `WithContext` method (e.g. `*http.Client`). Types that do support
`WithContext` (e.g. `*gorm.DB`) should use that directly.

```go
client := async.InCtx(ctx, httpClient)
```

---

## ds

`RingBuffer[T]` is a fixed-capacity circular buffer with O(1) push and
snapshot access via `Segments()` (two sorted slices that together represent
the logical sequence in order). `SyncRingBuffer[T]` adds a `sync.RWMutex`.

```go
ring := ds.GuardedRing[Event](1000)
ring.Push(event)
first, second := ring.Segments()
```

---

## Installation

```console
go get github.com/azuiktech/kleisli-go@v0.15.0
```

Requires Go 1.27 or later (generic method type parameters).

## License

MIT — see [LICENSE](LICENSE).

# kleisli-go

Small generic utilities for Go 1.27+: a `Result[T]` type for
railway-oriented error handling, an `Option[T]` type for present-or-absent
values, a `Stream[T]` type for functional-style slice pipelines, a
`Pipe[T]` type for CSP-style concurrent pipelines, and `Fn[T,U]`/`Fn2` for
point-free function composition. All of these use Go 1.27's
generic-method type parameters, so transformations that change type
(`Map[U]`, `FlatMap[U]`, `Then[U]`, ...) are plain chained method calls
rather than free functions or wrapper types.

`result`, `option`, and `tacit` are dependency-free. `async` takes
`golang.org/x/time/rate` for `RateLimit`; `stream` pulls that in
transitively via its own `Region`/`Parallel` bridge to `async.Pipe` (see
below) — nothing in `stream`'s own code references it directly.

The name comes from the [Kleisli category](https://en.wikipedia.org/wiki/Kleisli_category)
— the category of monadic functions — which is exactly what `Result`'s (and
`Option`'s) `FlatMap`/`Then` compose.

```go
import (
    "github.com/azuiktech/kleisli-go/async"
    "github.com/azuiktech/kleisli-go/lazy"
    "github.com/azuiktech/kleisli-go/option"
    "github.com/azuiktech/kleisli-go/result"
    "github.com/azuiktech/kleisli-go/stream"
    "github.com/azuiktech/kleisli-go/tacit"
    "github.com/azuiktech/kleisli-go/value"
)
```

## result

`Result[T]` holds either a success value or an error, with combinators for
chaining fallible operations without repeated `if err != nil` checks.

```go
user, err := result.From(verifier.Verify(ctx, token)).
    MapErrf("verify token %q", token).
    Then(upsertUser).
    FlatMap(ensurePlan).
    Then(buildDTO).
    Unwrap()
```

See [`result/result.go`](result/result.go) for the full API: `OK`, `Err`,
`From`, `FromNonZero`, `Unwrap`, `MustGet`, `OrElse`, `OrElseGet`, `Or`, `MapErr`,
`MapErrf`, `WrapErr`, `Tap`, `TapErr`, `Map`, `FlatMap`, `Flatten`, `Then`, `Contains`.


## option

`Option[T]` holds either a present, non-nil value or nothing — the
replacement for a nil-pointer check or a hand-rolled `(T, bool)` "is this
present" pair. `Some` refuses a nil value (nil and absent are the same
concept), so JSON serializes as the value itself or `null` — never an
ambiguous wrapper.

```go
var p *User
name := option.From(p).
    Map(func(u *User) string { return u.Name }).
    OrElse("")

userOpt := option.FromMap(usersByID, 123)
tokenOpt := option.FromNonZero(cfg.AuthToken)
```

See [`option/option.go`](option/option.go) for the full API: `Some`, `None`,
`From`, `FromMap`, `FromOk`, `FromNonZero`, `FromSlice`, `FromResult`, `ToPtr`, `ToResult`, `ToResultGet`, `ToSlice`,
`IsSome`, `IsNone`, `Unwrap`, `MustGet`, `Expect`, `OrElse`, `OrElseGet`, `Or`,
`Filter`, `Tap`, `Fold`, `Map`, `FlatMap`, `Flatten`, `Then`, `Contains`.



## value

`value` provides standalone helpers for pointer manipulation, value pipelines, error fallbacks, formatting, and ternaries.

```go
port := value.Deref(config.Port, 8080)
name := value.Cond(user != nil, user.Name, "Guest")
data := value.Must(os.ReadFile(path))
secret, err := value.MapErr(fetchSecret(key), "get secret %q", key)
```

See [`value/value.go`](value/value.go) for the full API: `Must`, `WrapErr`, `MapErr`,
`Fallback`, `FallbackGet`, `Cond`, `CondGet`, `Ptr`, `Deref`, `DerefGet`,
`DerefZero`, `Tap`, `Pipe`, `Zero`, `IsZero`, `Coalesce`.


## lazy

`lazy` provides thread-safe, memoized zero-argument lazy computation (`Lazy[T]`)
backed by `sync.OnceValue`, along with key-based 1-argument function
memoization (`Memoize` / `MemoizeErr`).

```go
// Deferred execution, evaluated at most once when Get() is called
user := lazy.FromErr(func() (*User, error) { return fetchUser(id) })
name := user.Get().Map(func(u *User) string { return u.Name }).OrElse("Guest")

// Key-based memoization (thread-safe, executed at most once per key)
cachedFetch := lazy.MemoizeErr(fetchUserByID)
```

See [`lazy/lazy.go`](lazy/lazy.go) for the full API: `New`, `FromErr`,
`Get`, `Map`, `FlatMap`, `ToOption`, `Memoize`, `MemoizeErr`.

## stream

`Stream[T]` wraps a slice for eager, chainable pipeline operations.
`Seq[T]` is its lazy, pull-based counterpart, wrapping the standard
library's own `iter.Seq[T]` — the two share one implementation for every
operation they both offer (`Filter`, `Map`, `TakeWhile`, ...), so a `Seq`
pipeline genuinely short-circuits (`Filter().Map().First()` stops pulling
the source the instant a match is found) while a `Stream` built from the
same call stays eager, exactly as before. `Stream` additionally offers
operations that need the whole sequence — `Reverse`, the `SortBy` family,
`GroupBy`, `ToMap`, `Partition`, `Last`, `Len` — sound there specifically
because a slice is always finite and already in memory; absent from
`Seq`'s method set entirely, since an arbitrary `iter.Seq` might not be.

```go
totals := stream.Of(invoices).
    Filter(func(inv Invoice) bool { return inv.Status == Unpaid }).
    GroupBy(func(inv Invoice) string { return inv.ClientID })

found, ok := stream.FromSeq(someGenerator).
    Filter(isValid).
    First(matchesQuery) // stops pulling the moment it finds one
```

See [`stream/stream.go`](stream/stream.go) for `Stream`'s full API: `Of`
(alias `FromSlice`), `Empty`, `OfMap`, `Filter`, `Each`, `Any`, `All`,
`First`, `Last`, `Take`, `Skip`, `Reverse`, `Len`, `Collect`, `Map`,
`FlatMap`, `Reduce`, `Fold`, `Scan`, `WindowFixed`, `WindowSliding`,
`GroupBy`, `ToMap`, `DistinctBy`, `Distinct`, `Enumerate`, `SortBy`
family, `Partition`, `Zip`, `Gather`. See [`stream/seq.go`](stream/seq.go)
for `Seq`'s: `FromSeq`, `Filter`, `Map`, `FlatMap`, `Take`, `Skip`, `Each`,
`Any`, `All`, `First`, `Reduce`, `Collect`, `Gather`, `MapMulti`,
`FilterMap`, `TakeWhile`, `DropWhile`, `DistinctBy`, `DistinctSeq`,
`EnumerateSeq`, `ZipSeq`.

[`stream/parallel.go`](stream/parallel.go) bridges `Stream` to `async.Pipe`
(below) for the one thing this package deliberately doesn't do itself —
run across goroutines. `Region` is the general form, taking a closure that
can call anything `async.Pipe` offers; `Parallel` is sugar over `Region`
for the single most common case.

```go
// Region: any Pipe operation, chained however the region needs.
throttled := stream.Of(urls).
    Region(func(p async.Pipe[string]) async.Pipe[string] {
        return p.RateLimit(ctx, limiter)
    }).
    Collect()

// Parallel: sugar over Region for bounded concurrency alone.
fetched := stream.Of(urls).Parallel(8, fetchAndParse).Collect()
```

`ToPipe`/`FromPipe` are the underlying, non-chained boundary crossing
`Region` is built from, for callers who want to hold the `Pipe` across
more than one statement.

## async

`Pipe[T]` wraps a channel for CSP-style pipeline composition — `Stream`'s
counterpart for work that benefits from goroutines: worker pools, rate
limiting, fan-out/fan-in, batching. Every stage is an explicitly named
call; nothing infers an execution strategy from context.

```go
total := async.From(urls).
    Parallel(8, fetchAndParse).
    Reduce(0, sum)
```

See [`async/pipe.go`](async/pipe.go) for the full API: `From`, `Go`, `Map`,
`Parallel`, `Buffer`, `Fork`, `Merge`, `RateLimit`, `Window`, `Enumerate`,
`Ordered`, `Collect`, `Reduce`, `Each`, `Await`.

## tacit

`Fn[T, U]`/`Fn2[A, B, U]` are named function types for point-free (tacit)
composition — building a new function entirely by combining existing
ones, no named intermediate argument anywhere in the definition. Nothing
here executes until the composed `Fn` is finally called with an
argument, and nothing here is data or a pipeline — it exists purely to
build the `func(T) U` values `stream.Map`/`async.Pipe.Parallel`/etc. take
as arguments (a composed `Fn` is directly assignable wherever a plain
`func(T) U` is expected, no conversion needed).

```go
var StdDev = Subtract.Fork(Identity, Mean).Then(Square).Then(Mean).Then(Sqrt)

StdDev([]float64{2, 4, 4, 4, 5, 5, 7, 9}) // 2
```

`Fork` is the array-language fork train: `combine.Fork(f, g)(x) =
combine(f(x), g(x))` — applying two functions to the same argument and
combining their results with a third (the receiver). `Subtract.Fork(Identity,
Mean)` is `x - mean(x)` without naming `x` or the mean anywhere.

See [`tacit/tacit.go`](tacit/tacit.go) for the full API: `Then`, `Fork`,
`Identity`.


## Installation

```console
go get github.com/azuiktech/kleisli-go
```

Requires Go 1.27 or later (generic method type parameters).

## License

MIT — see [LICENSE](LICENSE).

# kleisli-go

Small generic utilities for Go 1.27+: a `Result[T]` type for
railway-oriented error handling, an `Option[T]` type for present-or-absent
values, a `Stream[T]` type for functional-style slice pipelines, and a
`Pipe[T]` type for CSP-style concurrent pipelines. All of these use Go
1.27's generic-method type parameters, so transformations that change type
(`Map[U]`, `FlatMap[U]`, `Then[U]`, ...) are plain chained method calls
rather than free functions or wrapper types.

`result`, `option`, and `stream` are dependency-free. `async` is the one
exception — it takes `golang.org/x/time/rate` for `RateLimit`.

The name comes from the [Kleisli category](https://en.wikipedia.org/wiki/Kleisli_category)
— the category of monadic functions — which is exactly what `Result`'s (and
`Option`'s) `FlatMap`/`Then` compose.

```go
import (
    "github.com/azuiktech/kleisli-go/async"
    "github.com/azuiktech/kleisli-go/option"
    "github.com/azuiktech/kleisli-go/result"
    "github.com/azuiktech/kleisli-go/stream"
)
```

## result

`Result[T]` holds either a success value or an error, with combinators for
chaining fallible operations without repeated `if err != nil` checks.

```go
user, err := result.From(verifier.Verify(ctx, token)).
    MapErr(wrapUnauthorized).
    Then(upsertUser).
    FlatMap(ensurePlan).
    Then(buildDTO).
    Unwrap()
```

See [`result/result.go`](result/result.go) for the full API: `OK`, `Err`,
`From`, `Val`, `Error`, `Unwrap`, `MustGet`, `OrElse`, `OrElseGet`, `MapErr`,
`Tap`, `TapErr`, `Map`, `FlatMap`, `Then`.

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
```

See [`option/option.go`](option/option.go) for the full API: `Some`, `None`,
`From`, `ToPtr`, `IsSome`, `IsNone`, `Unwrap`, `MustGet`, `Expect`, `OrElse`,
`OrElseGet`, `Filter`, `Tap`, `Fold`, `Map`, `FlatMap`, `Then`.

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

## Installation

```console
go get github.com/azuiktech/kleisli-go
```

Requires Go 1.27 or later (generic method type parameters).

## License

MIT — see [LICENSE](LICENSE).

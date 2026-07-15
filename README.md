# kleisli-go

Small, dependency-free generic utilities for Go 1.27+: a `Result[T]` type for
railway-oriented error handling, and a `Stream[T]` type for functional-style
slice pipelines. Both use Go 1.27's generic-method type parameters, so
transformations that change type (`Map[U]`, `FlatMap[U]`, `Then[U]`, ...) are
plain chained method calls rather than free functions or wrapper types.

The name comes from the [Kleisli category](https://en.wikipedia.org/wiki/Kleisli_category)
— the category of monadic functions — which is exactly what `Result`'s
`FlatMap`/`Then` compose.

```go
import (
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

## stream

`Stream[T]` wraps a slice for eager, chainable pipeline operations.

```go
totals := stream.Of(invoices).
    Filter(func(inv Invoice) bool { return inv.Status == Unpaid }).
    GroupBy(func(inv Invoice) string { return inv.ClientID })
```

See [`stream/stream.go`](stream/stream.go) for the full API: `Of`, `Empty`,
`Filter`, `Each`, `Any`, `All`, `First`, `Last`, `Take`, `Skip`, `Reverse`,
`Len`, `Collect`, `Map`, `FlatMap`, `Reduce`, `GroupBy`, `ToMap`.

## Installation

```console
go get github.com/azuiktech/kleisli-go
```

Requires Go 1.27 or later (generic method type parameters).

## License

MIT — see [LICENSE](LICENSE).

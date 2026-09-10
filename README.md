# kleisli-go

Generic, composable functional utilities for Go 1.27+.

`kleisli-go` provides five cohesive packages covering algebraic data types, pure function transforms, eager and lazy stream pipelines, CSP concurrency, coroutines, and specialized in-memory data structures.

All types leverage Go 1.27 generic method type parameters, allowing transformations that change type (`Map[U]`, `FlatMap[U]`, `Then[U]`) to be written as fluent method chains rather than nested free functions.

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

## Documentation Index

Comprehensive, function-by-function chapterwise API catalogs and real-world recipes are available in the [`docs/`](docs/) directory:

| Chapter | Topic | Highlights & Contents |
|---|---|---|
| [Chapter 1: adt](docs/01_adt.md) | **Algebraic Data Types** | `Result[T]`, `Option[T]`, `Unit`/`Void`, `Lazy[T]`, `Any` dynamic box, JSON serialization |
| [Chapter 2: fn](docs/02_fn.md) | **Functional Utilities** | Value helpers, bound predicates, point-free composition, rotated algorithms, parsers, string transforms |
| [Chapter 3: stream](docs/03_stream.md) | **Data Pipelines** | Eager `Stream[T]`, lazy `Seq[T]`, numeric aggregations (`NumberStream`), async bridging |
| [Chapter 4: async](docs/04_async.md) | **Concurrency & Coroutines** | CSP `Pipe[T]`, `Future`/`Promise`, `Task`/`Co` coroutine engine, `Sync[T]`, `Handle[D]`, `Ctx[T]` |
| [Chapter 5: ds](docs/05_ds.md) | **Data Structures** | Circular `RingBuffer`, thread-safe `SyncRingBuffer`, multi-indexed `Table[V]`, 2D `Grid[R, C, V]`, `Set[T]`, `Map[K, V]`, `Vec[T]` |
| [Chapter 6: examples](docs/06_examples.md) | **Practical Recipes** | 10 production recipes (CORS/Auth, Batch URLs, Weather Coroutine, Graph Audit, etc.) |

---

## Packages Overview

### 1. `adt` — Algebraic Data Types

Consolidates `Result[T]`, `Option[T]`, `Unit`/`Void`, `Lazy[T]`, and `Any` into a unified package.

#### Fallible Pipelines (`Result[T]`)
Compose sequential fallible operations without nested error checks, dispatching at the boundary via `Fold`:

```go
adt.From(verifier.Verify(ctx, token)).
    MapErrf("verify token %q", token).
    Then(upsertUser).
    FlatMap(ensurePlan).
    Then(buildDTO).
    Fold(
        func(dto UserDTO) { renderJSON(w, http.StatusOK, dto) },
        func(err error)   { renderError(w, http.StatusUnauthorized, err) },
    )
```

#### Safe Optionality (`Option[T]`)
Eliminate nil-pointer dereferences and loose boolean flags:

```go
var user *User
displayName := adt.Opt(user).
    Map((*User).Name).
    OrElse("Guest")

// Map and slice lookups
userOpt := adt.FromMap(usersByID, id)
firstArg := adt.FromSlice(os.Args, 1).OrElse("default")
```

#### Unit, Lazy Evaluation & Dynamic Typing
```go
// Void return signal
return adt.OK(adt.Void)

// Thread-safe deferred evaluation
config := adt.Defer(loadConfiguration)
port := config.Get().Port

// Type-safe dynamic boxing
adt.Register[AuditLog]("audit.log")
box := adt.Dyn(AuditLog{Action: "login"})
logOpt := adt.As[AuditLog](box)
```

---

### 2. `fn` — Pure Function Utilities

Stateless value helpers, predicates, point-free function composition, rotated sequence search, and type parsers.

#### Value Helpers & Predicates
```go
port := fn.Deref(cfg.Port, 8080)
label := fn.Cond(isAdmin, "Admin", "User")
timeout := fn.Clamp(reqTimeout, 100*time.Millisecond, 5*time.Second)

// Composable predicates
isValidUser := fn.AllOf(
    fn.GreaterThanOrEqual(18),
    fn.NotIn(bannedIDs...),
)
```

#### Point-Free Composition & Rotated Search
```go
// Array-style fork train: combine.Fork(f, g)(x) = combine(f(x), g(x))
mean := fn.Fn2[float64, float64, float64](divide).Fork(sum, count)
avg := mean([]float64{10, 20, 30}) // 20.0

// Binary search over two partitions of a circular buffer
first, second := ring.Segments()
idx := fn.FindRotated(first, second, targetID, compareFunc)
afterFirst, afterSecond := fn.After(first, second, idx)
```

---

### 3. `stream` — Eager & Lazy Data Pipelines

`Stream[T]` provides an eager, slice-backed pipeline for collections requiring whole-dataset operations (sorting, reversing, grouping). `Seq[T]` provides a lazy, pull-based pipeline wrapping Go's `iter.Seq[T]` that short-circuits.

#### Eager Processing (`Stream[T]`)
```go
// Group unpaid invoices by client using method references
grouped := stream.Of(invoices).
    Filter(Invoice.IsUnpaid).
    SortBy(Invoice.DueDate).
    GroupBy(Invoice.ClientID)
```

#### Lazy Evaluation (`Seq[T]`)
```go
// Short-circuits: stops pulling immediately on first match
foundOpt := stream.FromSeq(generator).
    Filter(isValid).
    FirstOpt(matchesQuery)
```

#### Numeric Streams & Concurrency Bridge
```go
// Numeric aggregations using method reference
avgScore := stream.Of(grades).
    MapToNumber(Grade.Score).
    Mean().
    OrElse(0.0)

// Bridge to concurrent worker pool without leaving the chain
processed := stream.Of(urls).
    Parallel(8, fetchAndParse).
    Collect()
```

---

### 4. `async` — Concurrency, State & Coroutines

Channels, asynchronous handles, suspended coroutines, and thread-safe shared state.

#### CSP Pipelines (`Pipe[T]`)
```go
// Concurrent rate-limited processing with context cancellation
results := async.FromContext(ctx, urls).
    RateLimit(50, 5).              // 50 req/sec token bucket
    Parallel(8, fetchURL).         // 8 concurrent workers
    Batch(100, 200*time.Millisecond).
    Collect()
```

#### Futures & Promises
```go
prom, fut := async.NewPromise[Config](ctx)

go func() {
    // Settle promise using monadic result fold
    adt.From(loadRemoteConfig(path)).
        Fold(prom.Resolve, prom.Reject)
}()

res := fut.Await() // adt.Result[Config]
```

#### Suspendable Coroutines & Tasks
Strongly-typed effect suspension and two-way communication:

```go
var QueryWeather = async.DefineOp[WeatherQuery, string]("query_weather")

cfg := async.Config{
    Context: async.NewGoroutineContext(ctx),
    OnEmit:  func(msg any) { log.Println("Emit:", msg) },
    OnCall: []async.CallHandler{
        QueryWeather.Handle(func(q WeatherQuery) adt.Result[string] {
            return adt.OK("25 deg C")
        }),
    },
}

task := async.Launch(cfg, "bangalore", func(co *async.Co, city string) adt.Result[string] {
    co.Emit("Looking up " + city)
    temp := co.Call(QueryWeather, WeatherQuery{City: city}).Await().MustGet()
    return adt.OK(city + ": " + temp)
})

output := task.Await().MustGet()
```

#### Synchronized State & Pimpl Handles
Multiple structs share identical thread-safe state without interface declarations:

```go
type storeState struct {
    items map[string]Item
}

type CatalogStore struct{ async.Handle[storeState] }
type AdminStore   struct{ async.Handle[storeState] }

h := async.NewHandle(storeState{items: make(map[string]Item)})
catalog := CatalogStore{h}
admin   := AdminStore{h} // Both operate on the same synchronized memory
```

---

### 5. `ds` — High-Performance Data Structures

#### Circular Buffer (`RingBuffer[T]` & `SyncRingBuffer[T]`)
Fixed-capacity buffer with $O(1)$ push (overwriting oldest) and zero-copy partition slices:

```go
ring := ds.Ring[int](1000)
ring.Push(42)

// Zero-copy chronological view
first, second := ring.Segments()

// Thread-safe variant
guarded := ds.GuardedRing[Event](1000)
guarded.Push(newEvent)
```

#### Multi-Index Relational Table (`Table[V]`)
In-memory table supporting primary keys, secondary unique keys, and non-unique index groupings with atomic insertion rollback:

```go
var (
    ByID    = ds.UniqueIndex((*User).ID)
    ByEmail = ds.UniqueIndex((*User).Email)
    ByRole  = ds.NonUniqueIndex((*User).Role)
)

table := ds.NewTable(ByID, ByEmail, ByRole)
table.Insert(&User{id: "u1", email: "u1@acme.com", role: "engineer"})

// Type-safe index views
userOpt := ByEmail.Find(table, "u1@acme.com") // adt.Option[*User]
engineers := ByRole.Find(table, "engineer")    // []*User
```

#### 2D Coordinate Grid (`Grid[R, C, V]`)
2D sparse table with bidirectional indexing for fast row and column slicing:

```go
grid := ds.NewGrid[string, string, Permission]()
grid.Put("engineering", "repo:read", PermAllow)
grid.Put("engineering", "repo:write", PermAllow)

// Zero-allocation row iterator
for action, perm := range grid.Row("engineering") {
    fmt.Printf("%s -> %v\n", action, perm)
}
```

#### Encapsulated Collections (`Set[T]`, `Map[K, V]`, `Vec[T]`)
Functional, encapsulated collections with in-place mutation, iterators, and zero-allocation semantics:

```go
// Set & Map: 8-byte value structs, zero nil-pointer overhead
set := ds.NewSet("read", "write").Add("admin")
m := ds.NewMap(ds.EntryOf("host", "localhost"), ds.EntryOf("port", "8080"))
m.Put("env", "prod")

// Vec: Dynamic array protected by noCopy sentinel
vec := ds.NewVec[int]()
vec.PushAll(10, 20, 30)
```

---

## Installation

```console
go get github.com/azuiktech/kleisli-go@v0.21.0
```

Requires Go 1.27 or later (generic method type parameters).

## License

MIT — see [LICENSE](LICENSE).

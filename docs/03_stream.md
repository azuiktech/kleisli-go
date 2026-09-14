# Chapter 3: Data Pipelines (`stream`)

The `stream` package provides two unified stream processing models:
1. **`Stream[T]`**: An eager, slice-backed pipeline optimized for in-memory collections and operations that require whole-sequence inspection (sorting, reverse, grouping, map materialization).
2. **`Seq[T]`**: A lazy, pull-based pipeline wrapping the Go standard library's `iter.Seq[T]`, capable of processing infinite or deferred streams without allocating intermediate slices.

Both types share the same underlying iteration engine, ensuring behavioral consistency across eager and lazy processing.

```go
import "github.com/azuiktech/kleisli-go/stream"
```

---

## 1. Stream[T] (Eager Slice Pipeline)

### Constructors

| Constructor | Signature | Description |
|---|---|---|
| `Of[T]` | `func Of[T any](items []T) Stream[T]` | Wraps a slice into a `Stream[T]`. Zero-copy (shares backing array). |
| `FromSlice[T]` | `func FromSlice[T any](items []T) Stream[T]` | Counterpart alias of `Of`. |
| `OfOption[T]` | `func OfOption[T any](o adt.Option[T]) Stream[T]` | Lifts `Option[T]` into a 0 or 1 element stream. |
| `Empty[T]` | `func Empty[T any]() Stream[T]` | Constructs an empty `Stream[T]`. |
| `OfMap[K, V]` | `func OfMap[K comparable, V any](m map[K]V) Stream[Pair[K, V]]` | Wraps map key-value pairs into a stream of `Pair[K, V]`. |

### Intermediate Transforms

| Method | Signature | Description |
|---|---|---|
| `Clone()` | `func (s Stream[T]) Clone() Stream[T]` | Clones the underlying slice to isolate mutations. |
| `Filter(fn)` | `func (s Stream[T]) Filter(fn func(T) bool) Stream[T]` | Retains elements satisfying `fn`. |
| `Map[U](fn)` | `func (s Stream[T]) Map[U any](fn func(T) U) Stream[U]` | Transforms elements from type `T` to type `U`. |
| `FlatMap[U](fn)`| `func (s Stream[T]) FlatMap[U any](fn func(T) []U) Stream[U]` | Maps each element to a slice of `U` and concatenates the results. |
| `Take(n)` | `func (s Stream[T]) Take(n int) Stream[T]` | Takes the first `n` elements. |
| `Skip(n)` | `func (s Stream[T]) Skip(n int) Stream[T]` | Skips the first `n` elements. |
| `TakeWhile(fn)` | `func (s Stream[T]) TakeWhile(fn func(T) bool) Stream[T]` | Takes prefix while `fn` is true. |
| `DropWhile(fn)` | `func (s Stream[T]) DropWhile(fn func(T) bool) Stream[T]` | Skips prefix while `fn` is true. |
| `Enumerate[T]` | `func Enumerate[T any](s Stream[T]) Stream[adt.Indexed[T]]` | Indexes items with 0-based positions (`adt.Indexed[T]{Index, Value}`). A free function, not a method, called as `stream.Enumerate(s)`. |
| `Reverse()` | `func (s Stream[T]) Reverse() Stream[T]` | Reverses slice in-place in the stream. |

### Sorting & Grouping (Slice-Only Operations)

These operations are exclusive to `Stream[T]` because they require inspecting the complete dataset.

| Method | Signature | Description |
|---|---|---|
| `SortBy[K]` | `func (s Stream[T]) SortBy[K cmp.Ordered](fn func(T) K) Stream[T]` | Stable sorts elements by an extracted ordered key. |
| `SortByCached[K]` | `func (s Stream[T]) SortByCached[K cmp.Ordered](fn func(T) K) Stream[T]` | Caches sort keys for expensive extractors before sorting. |
| `SortFunc(less)`| `func (s Stream[T]) SortFunc(less func(a, b T) int) Stream[T]` | Sorts elements using a custom comparator (-1, 0, 1) — useful when the ordering isn't expressible as a key projection. Stable, matching `SortBy`. |
| `GroupBy[K](fn)` | `func (s Stream[T]) GroupBy[K comparable](fn func(T) K) map[K][]T` | Buckets elements into a map of slices by key `K`. |
| `ToMap[K, V](fn)`| `func (s Stream[T]) ToMap[K comparable, V any](fn func(T) (K, V)) map[K]V` | Materializes elements into a `map[K]V`, extracting a key and value from each. Later elements overwrite earlier ones on key collision. |
| `ToMapBy[K, V](fn, merge)`| `func (s Stream[T]) ToMapBy[K comparable, V any](fn func(T) (K, V), merge func(existing, incoming V) V) map[K]V` | Like `ToMap`, but `merge` decides the value on key collision instead of last-write-wins. Use `stream.KeepFirst`, `stream.KeepLast`, or a custom combiner. |
| `Partition(fn)` | `func (s Stream[T]) Partition(fn func(T) bool) (Stream[T], Stream[T])` | Splits stream into `(matches, nonMatches)`. |

### Terminals, Search & Aggregations

| Method | Signature | Description |
|---|---|---|
| `Collect()` | `func (s Stream[T]) Collect() []T` | Materializes elements into a standard Go slice. |
| `Len()` | `func (s Stream[T]) Len() int` | Returns element count. |
| `Last(fn)` | `func (s Stream[T]) Last(fn func(T) bool) adt.Option[T]` | Returns `Some(last element satisfying fn)` or `None`. Scans backward — O(1) best case. |
| `Tap(fn)` | `func (s Stream[T]) Tap(fn func(T)) Stream[T]` | Executes side-effect on every item; returns stream for chaining. |
| `ForEach(fn)` | `func (s Stream[T]) ForEach(fn func(T))` | Terminal consumer executing side-effect on every item (returns void). |
| `AnyOf(fn)` | `func (s Stream[T]) AnyOf(fn func(T) bool) bool` | Returns true if at least one item satisfies `fn` (short-circuits). |
| `AllOf(fn)` | `func (s Stream[T]) AllOf(fn func(T) bool) bool` | Returns true if all items satisfy `fn` (short-circuits). |
| `NoneOf(fn)` | `func (s Stream[T]) NoneOf(fn func(T) bool) bool` | Returns true if no items satisfy `fn` (short-circuits). |
| `All()` | `func (s Stream[T]) All() iter.Seq[T]` | Standard range-over-func iterator over stream elements. |
| `First(fn)` | `func (s Stream[T]) First(fn func(T) bool) adt.Option[T]` | Returns `Some(first element satisfying fn)`, or `None`. |
| `Reduce[U](init, fn)`| `func (s Stream[T]) Reduce[U any](initial U, fn func(acc U, item T) U) U` | Left-folds items into an accumulator of type `U`. |
| `MinBy[K](fn)` | `func (s Stream[T]) MinBy[K cmp.Ordered](fn func(T) K) adt.Option[T]` | Returns item with minimal extracted key. |
| `MaxBy[K](fn)` | `func (s Stream[T]) MaxBy[K cmp.Ordered](fn func(T) K) adt.Option[T]` | Returns item with maximal extracted key. |
| `MinByCompare(fn)`| `func (s Stream[T]) MinByCompare(fn func(T, T) int) adt.Option[T]` | Finds minimum using custom comparison. `MaxBy` covers the maximum case — there is no `MaxByCompare`. |

### Free Functions

| Function | Signature | Description |
|---|---|---|
| `Distinct[T]` | `func Distinct[T comparable](s Stream[T]) Stream[T]` | Deduplicates elements preserving first occurrence order. |
| `DistinctBy[T, K]`| `func DistinctBy[T any, K comparable](s Stream[T], keyFn func(T) K) Stream[T]` | Deduplicates by extracted key `K`. |
| `Flatten[T]` | `func Flatten[T any](s Stream[Stream[T]]) Stream[T]` | Flattens a nested stream of streams. |
| `Zip[A, B]` | `func Zip[A, B any](sa Stream[A], sb Stream[B]) Stream[Pair[A, B]]` | Combines two streams into pairs until shorter ends. |

```go
// Direct method references on User:
usersByCountry := stream.Of(users).
    Filter(User.IsActive).
    SortBy(User.Name).
    GroupBy(User.Country)
```

---

## 2. Seq[T] (Lazy Pull Iterator)

`Seq[T]` wraps standard Go `iter.Seq[T]` functions. Every intermediate operation executes lazily on demand.

### Constructors

| Constructor | Signature | Description |
|---|---|---|
| `FromSeq[T]` | `func FromSeq[T any](seq iter.Seq[T]) Seq[T]` | Wraps native Go `iter.Seq[T]` generator. |
| `EmptySeq[T]` | `func EmptySeq[T any]() Seq[T]` | Constructs an empty lazy `Seq[T]`. |
| `SeqOf[T]` | `func SeqOf[T any](items ...T) Seq[T]` | Constructs a lazy `Seq[T]` from variadic items. |
| `SeqOfMap[K, V]`| `func SeqOfMap[K comparable, V any](m map[K]V) Seq[Pair[K, V]]` | Lazy iterator over map entries. |

### Lazy Pipeline Methods

| Method | Signature | Description |
|---|---|---|
| `Filter(fn)` | `func (s Seq[T]) Filter(fn func(T) bool) Seq[T]` | Yields only items satisfying predicate. |
| `Map[U](fn)` | `func (s Seq[T]) Map[U any](fn func(T) U) Seq[U]` | Lazily transforms items from `T` to `U`. |
| `FlatMap[U](fn)`| `func (s Seq[T]) FlatMap[U any](fn func(T) []U) Seq[U]` | Maps each element to a slice of `U` and flattens the results. |
| `Take(n)` | `func (s Seq[T]) Take(n int) Seq[T]` | Pulls at most `n` items then stops iterator. |
| `Skip(n)` | `func (s Seq[T]) Skip(n int) Seq[T]` | Skips first `n` items. |
| `TakeWhile(fn)` | `func (s Seq[T]) TakeWhile(fn func(T) bool) Seq[T]` | Pulls items while `fn` is true; short-circuits. |
| `DropWhile(fn)` | `func (s Seq[T]) DropWhile(fn func(T) bool) Seq[T]` | Drops prefix while `fn` is true. |
| `EnumerateSeq[T]` | `func EnumerateSeq[T any](s Seq[T]) Seq[adt.Indexed[T]]` | Pairs each yielded item with its 0-based index. A free function, called as `stream.EnumerateSeq(s)` — named to avoid shadowing `Stream`'s `Enumerate` in the same package. |
| `Tap(fn)` | `func (s Seq[T]) Tap(fn func(T)) Seq[T]` | Lazily inspects elements as they pass through without draining. |
| `ForEach(fn)` | `func (s Seq[T]) ForEach(fn func(T))` | Terminal consumer running `fn` on all elements (returns void). |
| `AnyOf(fn)` | `func (s Seq[T]) AnyOf(fn func(T) bool) bool` | Short-circuits on first `true`. |
| `AllOf(fn)` | `func (s Seq[T]) AllOf(fn func(T) bool) bool` | Short-circuits on first `false`. |
| `NoneOf(fn)` | `func (s Seq[T]) NoneOf(fn func(T) bool) bool` | Short-circuits on first `true`. |
| `All()` | `func (s Seq[T]) All() iter.Seq[T]` | Standard range-over-func iterator over sequence elements. |
| `First(fn)` | `func (s Seq[T]) First(fn func(T) bool) adt.Option[T]` | Returns `Some(first element satisfying fn)`, or `None` — short-circuits, never pulling past the match. |
| `Reduce[U](init, fn)`| `func (s Seq[T]) Reduce[U any](initial U, fn func(acc U, item T) U) U` | Consumes sequence into accumulator `U`. |
| `Collect()` | `func (s Seq[T]) Collect() []T` | Materializes lazy sequence into slice. |
| `ToMapBy[K, V](fn, merge)`| `func (s Seq[T]) ToMapBy[K comparable, V any](fn func(T) (K, V), merge func(existing, incoming V) V) map[K]V` | Builds a `map[K]V` with explicit collision handling via `merge`. Use `stream.KeepFirst`, `stream.KeepLast`, or a custom combiner. |
| `ToStream()` | `func (s Seq[T]) ToStream() Stream[T]` | Materializes lazy sequence into eager `Stream[T]`. |

### Seq Free Functions

| Function | Signature | Description |
|---|---|---|
| `DistinctSeq[T]` | `func DistinctSeq[T comparable](s Seq[T]) Seq[T]` | Deduplicates items lazily with an internal set. |
| `FlattenSeq[T]` | `func FlattenSeq[T any](s Seq[[]T]) Seq[T]` | Collapses a `Seq[[]T]` into a `Seq[T]`, yielding each inner slice's elements in order. |
| `ZipSeq[A, B]` | `func ZipSeq[A, B any](sa Seq[A], sb Seq[B]) Seq[adt.Pair[A, B]]` | Pairs two lazy sequences positionally, stopping at the shorter one, using `iter.Pull`. |
| `ZipSeq3[A, B, C]` | `func ZipSeq3[A, B, C any](sa Seq[A], sb Seq[B], sc Seq[C]) Seq[adt.Triple[A, B, C]]` | Triples three lazy sequences positionally, stopping at the shortest one. |
| `SeqOf[T]` | `func SeqOf[T any](items ...T) Seq[T]` | Constructs a lazy `Seq` yielding the given items. |
| `SeqOfMap[K, V]` | `func SeqOfMap[K comparable, V any](m map[K]V) Seq[adt.Pair[K, V]]` | Wraps a map's entries as a `Seq` of `adt.Pair` — the lazy counterpart of `OfMap`. |
| `EmptySeq[T]` | `func EmptySeq[T any]() Seq[T]` | Returns an empty lazy `Seq`. |

```go
// Short-circuits: only evaluates until a match is found
found := stream.FromSeq(hugeGenerator).
    Filter(isValid).
    First(matchesQuery) // adt.Option[T]: Some(match) or None
```

---

## 3. Numeric Pipelines

When items are mapped to numeric types, `MapToNumber` exposes dedicated arithmetic aggregators.

```go
type NumberStream[N Number] struct { /* unexported fields */ }
type NumberSeq[N Number] struct { /* unexported fields */ }
```

| Method | Return Type | Description |
|---|---|---|
| `s.MapToNumber(fn)` | `NumberStream[N]` or `NumberSeq[N]` | Converts elements to numbers via projection `fn`. |
| `.Sum()` | `N` | Computes arithmetic sum. |
| `.Product()` | `N` | Computes product of elements. |
| `.Mean()` | `adt.Option[float64]` | Computes average (returns `None` if empty). |
| `.Min()` | `adt.Option[N]` | Finds minimum value. |
| `.Max()` | `adt.Option[N]` | Finds maximum value. |

```go
avgSalary := stream.Of(employees).
    MapToNumber(Employee.Salary).
    Mean().
    OrElse(0.0)
```

---

## 4. Concurrency Bridge (`async.Pipe`)

`stream` provides seamless bridges to `async.Pipe` for concurrent execution across goroutines.

| Method / Function | Signature | Description |
|---|---|---|
| `s.ToPipe()` | `func (s Stream[T]) ToPipe() async.Pipe[T]` | Hands slice items into channel-backed `async.Pipe[T]`. |
| `seq.ToPipe()` | `func (s Seq[T]) ToPipe() async.Pipe[T]` | Lazily feeds sequence items into `async.Pipe[T]`. |
| `FromPipe(p)` | `func FromPipe[T any](p async.Pipe[T]) Stream[T]` | Collects drained `async.Pipe[T]` back into a `Stream[T]`. |
| `s.Parallel(n, fn)`| `func (s Stream[T]) Parallel[U any](n int, fn func(T) U) Stream[U]` | Sugar: runs `fn` across `n` worker goroutines via `Pipe`. Output order is not preserved. |
| `s.Region(fn)` | `func (s Stream[T]) Region[U any](fn func(async.Pipe[T]) async.Pipe[U]) Stream[U]` | Escape hatch: enters `async.Pipe`, chains arbitrary async stages (rate limit, buffer, worker pools), then returns to `Stream[U]`. |

```go
// Concurrent HTTP processing pipeline within a synchronous stream chain
results := stream.Of(imageURLs).
    Parallel(8, downloadAndCompress).
    Filter(isValid).
    Collect()
```

---

## 5. Practical Real-World Scenarios

### Scenario A: Word Frequency Histogram & Top-K Ranking (`FlatMap`, `GroupBy`, `SortBy`, `Take`)

Tokenizing text documents, filtering stop words, computing frequencies, and extracting the top $K$ most frequent words:

```go
type WordCount struct {
    Word  string
    Count int
}

func (wc WordCount) Freq() int { return wc.Count }

func TopKFrequentWords(documents []string, k int) []WordCount {
    stopWords := []string{"the", "a", "an", "and", "or", "in", "to", "of", "is"}

    // Step 1: Flatten sentences to lowercase words, dropping stop words
    words := stream.Of(documents).
        FlatMap(func(doc string) stream.Stream[string] {
            return stream.Of(strings.Fields(doc))
        }).
        Map(strings.ToLower).
        Map(fn.Trim(",.!?\"'")).
        Filter(fn.NotIn(stopWords...)).
        Collect()

    // Step 2: Group words and construct count aggregates
    grouped := stream.Of(words).GroupBy(fn.Identity[string])
    
    counts := stream.OfMap(grouped).
        Map(func(p adt.Pair[string, []string]) WordCount {
            return WordCount{Word: p.First(), Count: len(p.Second())}
        }).
        SortBy(WordCount.Freq).
        Reverse().
        Take(k).
        Collect()

    return counts
}
```

### Scenario B: Time-Series Pairwise Price Deltas (`Zip`, `Drop`)

Computing adjacent element rate-of-change by zipping a stream with its own tail:

```go
type PriceTick struct {
    Timestamp time.Time
    Price     float64
}

type PriceChange struct {
    From  float64
    To    float64
    Delta float64
}

func CalculatePriceDeltas(ticks []PriceTick) []PriceChange {
    if len(ticks) < 2 {
        return nil
    }

    currents := stream.Of(ticks)
    nexts := stream.Of(ticks).Skip(1)

    // Zip tick[i] with tick[i+1]
    return stream.Zip(currents, nexts).
        Map(func(p adt.Pair[PriceTick, PriceTick]) PriceChange {
            return PriceChange{
                From:  p.First().Price,
                To:    p.Second().Price,
                Delta: p.Second().Price - p.First().Price,
            }
        }).
        Collect()
}
```

### Scenario C: Order Queue Triage & Partitioning (`Partition`)

Splitting an incoming queue of orders into express fulfillment vs standard batching in a single pass:

```go
type Order struct {
    ID        string
    IsExpress bool
    TotalUSD  float64
}

func (o Order) ExpressPriority() bool { return o.IsExpress || o.TotalUSD >= 500.0 }

func TriageOrders(orders []Order) (express []Order, standard []Order) {
    expressStream, standardStream := stream.Of(orders).
        Partition(Order.ExpressPriority)

    return expressStream.Collect(), standardStream.Collect()
}
```

### Scenario D: Lazy Pull-Based Infinite Stream Processing (`Seq[T]`)

Pulling items from an infinite generator, filtering lazily, and stopping with zero buffer allocation:

```go
// Infinite Fibonacci iterator using standard Go iter.Seq[int64]
func InfiniteFibonacci() iter.Seq[int64] {
    return func(yield func(int64) bool) {
        var a, b int64 = 0, 1
        for {
            if !yield(a) {
                return
            }
            a, b = b, a+b
        }
    }
}

func GetEvenFibonacciBelow(limit int64) []int64 {
    return stream.FromSeq(InfiniteFibonacci()).
        TakeWhile(func(n int64) bool { return n < limit }).
        Filter(func(n int64) bool { return n%2 == 0 }).
        Collect()
}
```


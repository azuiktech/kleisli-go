# Chapter 4: Concurrency & Coroutines (`async`)

The `async` package provides concurrency primitives for Go:
1. **`Pipe[T]`**: CSP channel pipelines for bounded worker pools, rate limiting, buffering, windowing, and fan-out/fan-in.
2. **`Future[T]` & `Promise[T]`**: Single-assignment producer/consumer asynchronous handles with context cause propagation.
3. **Coroutines & Task Engine**: Strongly-typed effect suspension, two-way communication (`co.Emit`, `co.Call`), and replayable step journals.
4. **`Sync[T]`**: Thread-safe concurrent state encapsulation.
5. **`Ctx[T]`**: Context-value bundling.

```go
import "github.com/azuiktech/kleisli-go/async"
```

---

## 1. Pipe[T] (CSP Pipelines)

`Pipe[T]` wraps Go channels with context-aware composition. Cancelling the underlying `context.Context` terminates and drains all pipeline stages.

### Constructors

| Constructor | Signature | Description |
|---|---|---|
| `From[T]` | `func From[T any](items []T) Pipe[T]` | Lifts a slice into a `Pipe[T]` with `context.Background()`. |
| `FromContext[T]` | `func FromContext[T any](ctx context.Context, items []T) Pipe[T]` | Lifts a slice into a `Pipe[T]` bound to `ctx`. |
| `FromIter[T]` | `func FromIter[T any](seq iter.Seq[T]) Pipe[T]` | Streams an `iter.Seq[T]` into a `Pipe[T]`. |
| `FromIterContext[T]`| `func FromIterContext[T any](ctx context.Context, seq iter.Seq[T]) Pipe[T]`| Streams an `iter.Seq[T]` into a `Pipe[T]` bound to `ctx`. |
| `Go[T]` | `func Go[T any](fn func() T) Pipe[T]` | Runs `fn` in a background goroutine; yields the result on a 1-capacity Pipe. |
| `p.WithContext(ctx)`| `func (p Pipe[T]) WithContext(ctx context.Context) Pipe[T]` | Attaches a cancellation context to an existing pipe. |

### Intermediate Pipeline Stages

| Method / Function | Signature | Description |
|---|---|---|
| `p.Map[U](fn)` | `func (p Pipe[T]) Map[U any](fn func(T) U) Pipe[U]` | Streams 1-to-1 transforms through a dedicated worker goroutine. |
| `p.Parallel[U](n, fn)`| `func (p Pipe[T]) Parallel[U any](n int, fn func(T) U) Pipe[U]` | Spawns a pool of `n` worker goroutines. Output order is non-deterministic. |
| `p.Buffer(size)` | `func (p Pipe[T]) Buffer(size int) Pipe[T]` | Inserts an asynchronous ring buffer of capacity `size`. |
| `p.RateLimit(ctx, lim)`| `func (p Pipe[T]) RateLimit(ctx context.Context, lim *rate.Limiter) Pipe[T]` | Paces items no faster than `lim` allows, via `golang.org/x/time/rate`'s own `*rate.Limiter`. `ctx` cancelling stops the pipe early. |
| `p.Fork(n)` | `func (p Pipe[T]) Fork(n int) []Pipe[T]` | Splits pipe into `n` identical broadcast copies (unbuffered). Every branch must be read at roughly the same pace — a slow branch stalls the shared pump for all siblings. |
| `Merge(pipes...)` | `func Merge[T any](pipes ...Pipe[T]) Pipe[T]` | Fan-in: merges multiple pipes into one unified output pipe. Only `pipes[0]`'s context governs cancellation of the merged output. |
| `Window(p, n)` | `func Window[T any](p Pipe[T], n int) Pipe[[]T]` | Batches items into fixed-size chunks of `n`, emitting each chunk as soon as it fills (trailing chunk may be shorter). Free function, not a method — same instantiation-cycle reason as `Enumerate`. Panics if `n < 1`. |
| `Enumerate(p)` | `func Enumerate[T any](p Pipe[T]) Pipe[adt.Indexed[T]]` | Tags each item with its position as it's produced. Call this right after the source (typically right before `Parallel`) if a later stage may scramble arrival order. |
| `Ordered(p)` | `func Ordered[T any](p Pipe[adt.Indexed[T]]) Pipe[T]` | Re-orders out-of-order `adt.Indexed[T]` elements — `Enumerate`'s pair, restoring sequential order after `Parallel`. Unbounded reorder buffer. |
| `OrderedN(p, max)` | `func OrderedN[T any](p Pipe[adt.Indexed[T]], maxPending int) Pipe[T]` | Same as `Ordered`, but stops once more than `maxPending` items are waiting for a missing predecessor, instead of buffering without limit. |

### Terminals

| Method | Signature | Description |
|---|---|---|
| `Collect()` | `func (p Pipe[T]) Collect() []T` | Drains pipe and materializes all elements into a slice. |
| `Each(fn)` | `func (p Pipe[T]) Each(fn func(T))` | Drains pipe, invoking `fn` on each item. |
| `Reduce[U](init, fn)`| `func (p Pipe[T]) Reduce[U any](initial U, fn func(acc U, item T) U) U` | Left-folds pipe items into an accumulated value. |
| `Await()` | `func (p Pipe[T]) Await() adt.Option[T]` | Blocks for the single item a `Go`-built pipe produces. Returns `Some(value)`, or `None` if the pipe is already drained or empty. |

```go
// Concurrent rate-limited URL processor
limiter := rate.NewLimiter(rate.Limit(10), 1) // Max 10 req/sec

fetched := async.FromContext(ctx, urls).
    RateLimit(ctx, limiter).
    Parallel(4, fetchHTTPContent) // 4 concurrent workers

results := async.Window(fetched, 50).Collect()
```

---

## 2. Future[T] & Promise[T]

A single-assignment producer/consumer primitive for asynchronous operations.

### Types & Creation

```go
prom, fut := async.NewPromise[string](ctx)
```

| Type | Role | Key Methods |
|---|---|---|
| `Promise[T]` | Producer handle (writable once) | `Resolve(val T) bool`, `Reject(err error) bool`, `Cancel(cause error) bool` |
| `Future[T]` | Consumer handle (read-only) | `Await() adt.Result[T]`, `AwaitCtx(ctx) adt.Result[T]`, `Context() context.Context`, `Cancel(cause error) bool` |

```go
prom, fut := async.NewPromise[Config](ctx)

go func() {
    // Settle promise cleanly using monadic primitives
    adt.From(loadRemoteConfig(path)).
        Fold(prom.Resolve, prom.Reject)
}()

// Blocking wait returning adt.Result[Config]
res := fut.Await()
```

### Receiver[T]

`Receiver[T]` is the common handle every combinator below accepts as a source — anything that can hand back a `*Future[T]`. Implemented by `*Future[T]`, `*Promise[T]`, and `*Task[I, O]`.

```go
type Receiver[T any] interface {
    Future() *Future[T]
}
```

### Combinators

Multi-source coordination built on `Future[T]`. Each combinator takes a `context.Context` plus one or more `Receiver[T]` sources and returns a single `*Future[...]`.

| Function | Signature | Description |
|---|---|---|
| `AllOf[T](ctx, sources...)` | `func AllOf[T any](ctx context.Context, sources ...Receiver[T]) *Future[[]T]` | Resolves with all values, in input order, once every source succeeds. Rejects immediately with the first failure and cancels the rest. Empty `sources` resolves with `[]T{}`. |
| `AllSettled[T](ctx, sources...)` | `func AllSettled[T any](ctx context.Context, sources ...Receiver[T]) *Future[[]adt.Result[T]]` | Resolves once every source has settled (OK or Err), preserving input order. Never itself rejects. Empty `sources` resolves with `[]adt.Result[T]{}`. |
| `Race[T](ctx, sources...)` | `func Race[T any](ctx context.Context, sources ...Receiver[T]) *Future[T]` | Settles with the outcome (OK or Err) of whichever source completes first; cancels the rest. Rejects with `ErrEmptySources` if `sources` is empty. |
| `AnyOf[T](ctx, sources...)` | `func AnyOf[T any](ctx context.Context, sources ...Receiver[T]) *Future[T]` | Resolves with the first source to succeed; other sources keep running on failure. Rejects with `ErrAllFailed` only once every source has failed, or `ErrEmptySources` if `sources` is empty. |
| `Zip2[A, B](ctx, sa, sb)` | `func Zip2[A, B any](ctx context.Context, sa Receiver[A], sb Receiver[B]) *Future[adt.Pair[A, B]]` | Combines two heterogeneous sources into a `Future` of `adt.Pair`. Either failing fails the combined future fast and cancels the other. |
| `Zip3[A, B, C](ctx, sa, sb, sc)` | `func Zip3[A, B, C any](ctx context.Context, sa Receiver[A], sb Receiver[B], sc Receiver[C]) *Future[adt.Triple[A, B, C]]` | Same as `Zip2` for three heterogeneous sources, yielding `adt.Triple`. |

```go
// Wait for every fetch to succeed, or bail out on the first error.
allFut := async.AllOf(ctx, fut1, fut2, fut3)
docs := allFut.Await().MustGet()

// Combine two differently-typed futures into a pair.
pairFut := async.Zip2(ctx, userFut, profileFut)
user, profile := pairFut.Await().MustGet().Unpack()
```

`ErrEmptySources` and `ErrAllFailed` are the sentinel errors these combinators reject with, exported so callers can match them with `errors.Is`.

### TaskGroup

`TaskGroup` is the lower-level coordination primitive the combinators above are built on: it runs a batch of `Receiver[T]` sources under a shared cancellable context and lets a `policy` decide, result by result, when to stop early.

| Method / Function | Signature | Description |
|---|---|---|
| `NewTaskGroup()` | `func NewTaskGroup() *TaskGroup` | Creates a `TaskGroup` derived from `context.Background()`. |
| `TaskGroupWithContext(ctx)` | `func TaskGroupWithContext(ctx context.Context) *TaskGroup` | Creates a `TaskGroup` derived from `ctx`. |
| `tg.Context()` | `func (tg *TaskGroup) Context() context.Context` | Returns the group's cancellation context. |
| `tg.Run[T](sources, policy)` | `func (tg *TaskGroup) Run[T any](sources []Receiver[T], policy func(idx int, res adt.Result[T]) bool)` | Awaits every source under `tg`, calling `policy` with each result as it settles. Once `policy` returns `true`, the group cancels itself and every remaining pending source. Blocks until all sources are accounted for. |

---

## 3. Coroutines & Effect Engine

`kleisli-go` provides a coroutine and task system supporting cooperative suspension, typed effects, and caller-driven resumption.

### Types

- **`Op[S, R]`**: A strongly typed suspension operation effect. Suspends with payload `S` and resumes with response `R`.
- **`CallHandler`**: A handler that binds to an `Op[S, R]`.
- **`Co`**: Coroutine execution scope passed to the task function. Embeds `context.Context`.
- **`Task[I, O]`**: Driver handle for running, awaiting, or cancelling a packaged coroutine. Implements `Receiver[O]`.

### Defining Operations & Handlers

```go
// Define an operation: suspends with WeatherQuery, expects string response
var QueryWeather = async.DefineOp[WeatherQuery, string]("query_weather")

// Define a handler
handler := QueryWeather.Handle(func(q WeatherQuery) adt.Result[string] {
    return adt.OK("25 deg C")
})
```

| Method / Function | Signature | Description |
|---|---|---|
| `DefineOp[S, R](name)` | `func DefineOp[S, R any](name string) Op[S, R]` | Creates an operation mapping suspend payload `S` to resume type `R`. |
| `op.Name()` | `func (o Op[S, R]) Name() string` | Returns the operation's identifier. |
| `op.Handle(fn)` | `func (o Op[S, R]) Handle(fn func(s S) adt.Result[R]) CallHandler` | Creates a strongly-typed `CallHandler` for `op`. |

A `CallHandler` dispatches on a `CallInvocation{OpName string; S any}` — the untyped `(op name, payload)` pair a `Context` passes through `Call`.

### Coroutine Scope (`Co`) Methods

| Method | Signature | Description |
|---|---|---|
| `co.Emit(val)` | `func (c *Co) Emit(val any)` | Sends a fire-and-forget notification to the caller's `OnEmit` listener. |
| `co.Call[S, R](op, s)`| `func (c *Co) Call[S, R any](op Op[S, R], s S) *Future[R]` | Suspends the coroutine, sends `s` to the configured handler for `op`, and returns a `*Future[R]`. |
| `co.Async[R](fn)` | `func (c *Co) Async[R any](fn func(ctx context.Context) adt.Result[R]) *Future[R]` | Dispatches a concurrent branch within the coroutine engine, returning `*Future[R]`. |

### Launching Tasks

| Function / Method | Signature | Description |
|---|---|---|
| `Launch[I, O](cfg, in, fn)` | `func Launch[I, O any](cfg Config, in I, fn func(co *Co, in I) adt.Result[O]) *Task[I, O]` | Packages the task and immediately runs it on a new goroutine. |
| `PackagedTask[I, O](cfg, fn)`| `func PackagedTask[I, O any](cfg Config, fn func(co *Co, in I) adt.Result[O]) *Task[I, O]`| Packages the coroutine function without executing it. Call `t.Run(in)` to start. |
| `t.Run(in)` | `func (t *Task[I, O]) Run(in I) adt.Result[O]` | Starts the task with input `in` and blocks for its result — safe to call synchronously, from a worker pool, or concurrently from multiple callers; the task runs once and everyone gets the settled result. Fails fast if the task's context was already cancelled before start. |
| `t.Await()` | `func (t *Task[I, O]) Await() adt.Result[O]` | Blocks until the coroutine completes, returning `Result[O]`. |
| `t.AwaitCtx(ctx)` | `func (t *Task[I, O]) AwaitCtx(ctx context.Context) adt.Result[O]` | Blocks until the coroutine completes or `ctx` expires. |
| `t.Cancel(cause)` | `func (t *Task[I, O]) Cancel(cause error) bool` | Cancels the task's context. Returns whether this call performed the cancellation. |
| `t.Context()` | `func (t *Task[I, O]) Context() context.Context` | Returns the coroutine's context. |
| `t.Future()` | `func (t *Task[I, O]) Future() *Future[O]` | Returns the read-only `Future[O]` handle (satisfies `Receiver[O]`). |

### Execution Engines & Journaling

`Config.Context` plugs in the engine that actually runs `co.Call` and `co.Async` — every coroutine has one, defaulting to `GoroutineContext` when `Config.Context` is left nil.

- **`Context`**: The engine interface itself — embeds `context.Context` and adds `Call(op string, s any) *Future[any]` / `Async(fn func(ctx context.Context) adt.Result[any]) *Future[any]`.
- **`HandlerContext`**: Implemented by engines that support registering call handlers after construction: `SetHandlers(handlers []CallHandler)`.
- **`GoroutineContext`**: In-memory `Context` running coroutines and handlers on plain goroutines — no replay, no persistence.
- **`DurableContext`**: `Context` with deterministic step replay across workflow turns, backed by a `Journal`. Re-running the same coroutine replays already-completed steps from the journal instead of re-executing them, and can cap how many *new* steps run per turn via `SetMaxSteps`.
- **`ErrSuspended`**: sentinel a `DurableContext` step rejects with when its per-turn step budget (`SetMaxSteps`) is exhausted before the step has a recorded result.
- **`Journal`**: In-memory or persisted step history for deterministic replayable workflows.
  - `NewJournal() *Journal`
  - `(j *Journal) Append(val any)`
  - `(j *Journal) Get(idx int) (any, bool)`
  - `(j *Journal) Len() int`

| Method / Function | Signature | Description |
|---|---|---|
| `NewGoroutineContext(ctx)` | `func NewGoroutineContext(ctx context.Context) *GoroutineContext` | Creates an in-memory coroutine `Context` bound to `ctx`. |
| `(g *GoroutineContext) SetHandlers(handlers)` | `func (g *GoroutineContext) SetHandlers(handlers []CallHandler)` | Appends call handlers. |
| `(g *GoroutineContext) Call(op, s)` | `func (g *GoroutineContext) Call(op string, s any) *Future[any]` | Dispatches `op`/`s` to a registered handler on a new goroutine. |
| `(g *GoroutineContext) Async(fn)` | `func (g *GoroutineContext) Async(fn func(ctx context.Context) adt.Result[any]) *Future[any]` | Runs `fn` on a new goroutine. |
| `NewDurableContext(ctx, journal)` | `func NewDurableContext(ctx context.Context, journal *Journal) *DurableContext` | Constructs a `DurableContext` bound to `ctx`, backed by `journal`. |
| `(d *DurableContext) SetMaxSteps(n)` | `func (d *DurableContext) SetMaxSteps(n int)` | Caps the number of newly executed (non-replayed) steps this turn before suspending with `ErrSuspended`. |
| `(d *DurableContext) SetHandlers(handlers)` | `func (d *DurableContext) SetHandlers(handlers []CallHandler)` | Appends call handlers. |
| `(d *DurableContext) Call(op, s)` | `func (d *DurableContext) Call(op string, s any) *Future[any]` | Executes `op`/`s` against a handler, or replays its recorded result from the journal. |
| `(d *DurableContext) Async(fn)` | `func (d *DurableContext) Async(fn func(ctx context.Context) adt.Result[any]) *Future[any]` | Executes `fn`, or replays its recorded result from the journal. |

```go
cfg := async.Config{
    Context: async.NewGoroutineContext(ctx),
    OnEmit: func(val any) {
        fmt.Println("Emitted:", val)
    },
    OnCall: []async.CallHandler{
        QueryWeather.Handle(func(q WeatherQuery) adt.Result[string] {
            return adt.OK("22 deg")
        }),
    },
}

task := async.Launch(cfg, "bangalore", func(co *async.Co, city string) adt.Result[string] {
    co.Emit("Fetching temperature for " + city)
    temp := co.Call(QueryWeather, WeatherQuery{City: city}).Await().MustGet()
    return adt.OK(city + " is " + temp)
})

output := task.Await().MustGet()
```

---

## 4. Concurrent State (`Sync[T]`)

Encapsulates mutable state behind `sync.RWMutex` without exposing raw locks.

### `Sync[T]`

```go
type Sync[C any] struct { /* unexported fields */ }
```

| Method / Function | Signature | Description |
|---|---|---|
| `Of[C](inner)` | `func Of[C any](inner C) Sync[C]` | Wraps `inner` in a `Sync[C]`. |
| `s.Read(fn)` | `func (s *Sync[C]) Read(f func(C))` | Acquires RLock; passes shallow snapshot of `C` to `f`. |
| `s.Write(fn)` | `func (s *Sync[C]) Write(f func(*C))` | Acquires Lock; passes pointer to `C` for mutation. |
| `s.Fetch[R](fn)` | `func (s *Sync[C]) Fetch[R any](f func(C) R) R` | Acquires RLock; extracts computed value from snapshot into `R`. |
| `s.Mutate[R](fn)` | `func (s *Sync[C]) Mutate[R any](f func(*C) R) R` | Acquires Lock; mutates `C` and returns `R`. |

---

## 5. Ctx[T] (Context Pairing)

A light value type bundling any arbitrary value with a `context.Context`. Useful for dependencies lacking a native `WithContext` method.

```go
type Ctx[T any] struct {
    Context context.Context
    Val     T
}
```

```go
clientWithCtx := async.InCtx(ctx, httpClient)
```

---

## 6. Practical Real-World Scenarios

### Scenario A: Resilient Web Ingestion Pipeline (`Pipe[T]`, `RateLimit`, `Parallel`, `Window`)

Building a crawler pipeline that fetches web resources with token-bucket rate limiting, parallel workers, and batch database writing:

```go
type Document struct {
    URL  string
    Body string
}

func CrawlPipeline(ctx context.Context, targetURLs []string, dbSaver func([]Document)) {
    limiter := rate.NewLimiter(rate.Limit(100), 10) // 100 req/sec, burst 10

    fetched := async.FromContext(ctx, targetURLs).
        RateLimit(ctx, limiter).
        Parallel(16, func(url string) Document {
            // Concurrent worker fetch
            content := fetchHTML(url)
            return Document{URL: url, Body: content}
        }).
        Buffer(256) // Decouple network from DB

    async.Window(fetched, 50).Each(dbSaver) // Persist in batches of up to 50 docs
}
```

### Scenario B: Multi-Channel Event Broadcast & Fan-In (`Fork`, `Merge`)

Broadcasting incoming security events simultaneously to multiple sinks (Slack, PagerDuty, Elastic), and collecting acknowledgments into a single monitoring pipe:

```go
type SecurityEvent struct {
    Severity string
    Message  string
}

type DeliveryReceipt struct {
    Sink   string
    Status string
}

func FanOutFanInAlerts(ctx context.Context, events []SecurityEvent) []DeliveryReceipt {
    pipeline := async.FromContext(ctx, events)

    // Fork into 3 independent broadcast streams — each must be read at
    // roughly the same pace, or a slow sink stalls the shared pump for
    // the other two.
    sinks := pipeline.Fork(3)

    slackPipe := sinks[0].Map(func(ev SecurityEvent) DeliveryReceipt {
        sendSlackAlert(ev)
        return DeliveryReceipt{Sink: "slack", Status: "delivered"}
    })

    pagerDutyPipe := sinks[1].Map(func(ev SecurityEvent) DeliveryReceipt {
        sendPagerDuty(ev)
        return DeliveryReceipt{Sink: "pagerduty", Status: "delivered"}
    })

    auditPipe := sinks[2].Map(func(ev SecurityEvent) DeliveryReceipt {
        writeAuditLog(ev)
        return DeliveryReceipt{Sink: "audit_db", Status: "recorded"}
    })

    // Fan-in: merge receipts into a single stream and collect
    return async.Merge(slackPipe, pagerDutyPipe, auditPipe).Collect()
}
```

### Scenario C: Two-Factor Authentication Challenge Flow (`Co`, `Op[S, R]`, `Call`)

Modeling interactive multi-stage human/machine interactions using suspended coroutines:

```go
type OTPChallenge struct {
    UserID string
    Prompt string
}

var RequestOTP = async.DefineOp[OTPChallenge, string]("request_otp")

func Run2FAWorkflow(ctx context.Context, userID string, otpProvider func(string) string) adt.Result[string] {
    cfg := async.Config{
        Context: async.NewGoroutineContext(ctx),
        OnCall: []async.CallHandler{
            RequestOTP.Handle(func(c OTPChallenge) adt.Result[string] {
                // Prompt user or external device for 6-digit OTP
                code := otpProvider(c.Prompt)
                return adt.OK(code)
            }),
        },
    }

    task := async.Launch(cfg, userID, func(co *async.Co, uid string) adt.Result[string] {
        co.Emit("Initiating 2FA verification")

        // Suspends until caller supplies the OTP code
        code := co.Call(RequestOTP, OTPChallenge{
            UserID: uid,
            Prompt: "Enter the code sent to your phone:",
        }).Await().MustGet()

        if code != "123456" {
            return adt.Err[string](errors.New("invalid OTP code"))
        }

        return adt.OK("jwt_token_auth_approved")
    })

    return task.Await()
}
```


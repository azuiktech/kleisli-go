# Chapter 4: Concurrency & Coroutines (`async`)

The `async` package provides concurrency primitives for Go:
1. **`Pipe[T]`**: CSP channel pipelines for bounded worker pools, rate limiting, buffering, windowing, and fan-out/fan-in.
2. **`Future[T]` & `Promise[T]`**: Single-assignment producer/consumer asynchronous handles with context cause propagation.
3. **Coroutines & Task Engine**: Strongly-typed effect suspension, two-way communication (`co.Emit`, `co.Call`), and replayable step journals.
4. **`Sync[T]` & `Handle[D]`**: Thread-safe concurrent state encapsulation and the pointer-to-implementation (pimpl) pattern.
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
| `p.RateLimit(r, b)`| `func (p Pipe[T]) RateLimit(limit rate.Limit, burst int) Pipe[T]` | Token-bucket rate limiter via `golang.org/x/time/rate`. |
| `p.Fork(n)` | `func (p Pipe[T]) Fork(n int) []Pipe[T]` | Splits pipe into `n` identical broadcast copies (unbuffered). |
| `ForkBuffered(p, n, buf)`| `func ForkBuffered[T any](p Pipe[T], n, buf int) []Pipe[T]` | Splits pipe into `n` broadcast copies with buffered channels. |
| `Merge(pipes...)` | `func Merge[T any](pipes ...Pipe[T]) Pipe[T]` | Fan-in: merges multiple pipes into one unified output pipe. |
| `p.Window(size)` | `func (p Pipe[T]) Window(size int) Pipe[[]T]` | Groups stream elements into fixed-size chunks of `size`. |
| `p.Batch(size, dur)`| `func (p Pipe[T]) Batch(size int, timeout time.Duration) Pipe[[]T]` | Emits batches when `size` elements accumulate or `timeout` elapses. |
| `Ordered(p)` | `func Ordered[T any](p Pipe[Indexed[T]]) Pipe[T]` | Re-orders out-of-order `Indexed[T]` elements (restores sequential order after `Parallel`). |

### Terminals

| Method | Signature | Description |
|---|---|---|
| `Collect()` | `func (p Pipe[T]) Collect() []T` | Drains pipe and materializes all elements into a slice. |
| `Each(fn)` | `func (p Pipe[T]) Each(fn func(T))` | Drains pipe, invoking `fn` on each item. |
| `Reduce[U](init, fn)`| `func (p Pipe[T]) Reduce[U any](initial U, fn func(acc U, item T) U) U` | Left-folds pipe items into an accumulated value. |
| `Await()` | `func (p Pipe[T]) Await() T` | Reads the single (or first) item from pipe and closes. |

```go
// Concurrent rate-limited URL processor
results := async.FromContext(ctx, urls).
    RateLimit(10, 1).              // Max 10 req/sec
    Parallel(4, fetchHTTPContent). // 4 concurrent workers
    Batch(50, 500*time.Millisecond).
    Collect()
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

---

## 3. Coroutines & Effect Engine

`kleisli-go` provides a coroutine and task system supporting cooperative suspension, typed effects, and caller-driven resumption.

### Types

- **`Op[S, R]`**: A strongly typed suspension operation effect. Suspends with payload `S` and resumes with response `R`.
- **`CallHandler`**: A handler that binds to an `Op[S, R]`.
- **`Co`**: Coroutine execution scope passed to the task function. Embeds `context.Context`.
- **`Task[I, O]`**: Driver handle for running, sending data, or awaiting task completion.

### Defining Operations & Handlers

```go
// Define an operation: suspends with WeatherQuery, expects string response
var QueryWeather = async.DefineOp[WeatherQuery, string]("query_weather")

// Define a handler
handler := QueryWeather.Handle(func(q WeatherQuery) adt.Result[string] {
    return adt.OK("25 deg C")
})
```

### Coroutine Scope (`Co`) Methods

| Method | Signature | Description |
|---|---|---|
| `co.Emit(val)` | `func (c *Co) Emit(val any)` | Sends a fire-and-forget notification to the caller's `OnEmit` listener. |
| `co.Call[S, R](op, s)`| `func (c *Co) Call[S, R any](op Op[S, R], s S) *Future[R]` | Suspends the coroutine, sends `s` to the configured handler for `op`, and returns a `*Future[R]`. |
| `co.Async[R](fn)` | `func (c *Co) Async[R any](fn func(ctx context.Context) adt.Result[R]) *Future[R]` | Dispatches a concurrent branch within the coroutine engine, returning `*Future[R]`. |

### Launching Tasks

| Function / Method | Signature | Description |
|---|---|---|
| `Launch[I, O](cfg, input, fn)` | `func Launch[I, O any](cfg Config, input I, fn func(co *Co, in I) adt.Result[O]) *Task[I, O]` | Creates and immediately starts a coroutine task in the background. |
| `PackagedTask[I, O](cfg, input, fn)`| `func PackagedTask[I, O any](cfg Config, input I, fn func(co *Co, in I) adt.Result[O]) *Task[I, O]`| Creates an unstarted task. Must call `t.Run()` to start. |
| `t.Run()` | `func (t *Task[I, O]) Run()` | Starts an unstarted task (thread-safe, idempotent). |
| `t.Send(val)` | `func (t *Task[I, O]) Send(val any)` | Sends input value to the suspended task. |
| `t.Await()` | `func (t *Task[I, O]) Await() adt.Result[O]` | Blocks until task completes, returning `Result[O]`. |
| `t.Cancel(cause)` | `func (t *Task[I, O]) Cancel(cause error) bool` | Cancels the task and its underlying context. |
| `t.OnEmit(fn)` | `func (t *Task[I, O]) OnEmit(fn func(val any))` | Registers a callback for values emitted via `co.Emit`. |
| `t.OnDone(fn)` | `func (t *Task[I, O]) OnDone(fn func(res adt.Result[O]))`| Registers a non-blocking callback upon task completion. |

### Execution Engines & Journaling

- **`GoroutineContext`**: Standard in-memory engine running coroutines and handlers on goroutines.
- **`Journal`**: In-memory or persisted step history for deterministic replayable workflows.
  - `NewJournal() *Journal`
  - `(j *Journal) Append(val any)`
  - `(j *Journal) Get(idx int) (any, bool)`
  - `(j *Journal) Len() int`

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

## 4. Concurrent State (`Sync[T]` & `Handle[D]`)

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
| `s.Map[R](fn)` | `func (s *Sync[C]) Map[R any](f func(C) R) R` | Acquires RLock; transforms snapshot into `R`. |
| `s.Mutate[R](fn)` | `func (s *Sync[C]) Mutate[R any](f func(*C) R) R` | Acquires Lock; mutates `C` and returns `R`. |

### `Handle[D]` (Pimpl Pattern)

`Handle[D]` wraps `*Sync[D]` in a shallow value type. All copies share the exact same underlying synchronized state. Multiple structs can embed `Handle[D]` to present different API facades without interface overhead.

```go
type orgState struct {
    orgs map[string]string
}

type InMemOrgs struct { async.Handle[orgState] }
type InMemAdmin struct { async.Handle[orgState] }

h := async.NewHandle(orgState{orgs: make(map[string]string)})
orgService := InMemOrgs{h}
adminService := InMemAdmin{h} // Shares identical synchronized memory
```

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

### Scenario A: Resilient Web Ingestion Pipeline (`Pipe[T]`, `RateLimit`, `Parallel`, `Batch`)

Building a crawler pipeline that fetches web resources with token-bucket rate limiting, parallel workers, and batch database writing:

```go
type Document struct {
    URL  string
    Body string
}

func CrawlPipeline(ctx context.Context, targetURLs []string, dbSaver func([]Document)) {
    async.FromContext(ctx, targetURLs).
        RateLimit(100, 10).                    // Rate limit: 100 req/sec, burst 10
        Parallel(16, func(url string) Document {
            // Concurrent worker fetch
            content := fetchHTML(url)
            return Document{URL: url, Body: content}
        }).
        Buffer(256).                            // Decouple network from DB
        Batch(50, 500*time.Millisecond).       // Batch up to 50 docs or flush every 500ms
        Each(dbSaver)                          // Persist batch to DB
}
```

### Scenario B: Multi-Channel Event Broadcast & Fan-In (`ForkBuffered`, `Merge`)

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

    // Fork into 3 independent buffered streams
    sinks := async.ForkBuffered(pipeline, 3, 64)

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


# Ain

A Go library containing reusable utilities and data structures. Ain is named after
Epsilon Tauri.

## Motivation

Ain collects helpers shared by multiple projects so fixes and improvements can be
maintained in one place.

## Installation

Requires Go 1.23.0 or later. The library has no third-party Go dependencies.

```bash
go get github.com/abaxoth0/Ain
```

## Packages

- **`structs`**: Ring buffer, synchronized FIFO queue, stacks, worker pool, and metadata helpers.
- **`logger`**: Console and asynchronous file logging, log levels, serializers, and forwarding.
- **`errs`**: Error types and predefined errors with HTTP status codes.
- **`common`**: Retry, ternary, and map helpers.

The Go examples below are standalone programs. Lifecycle and concurrency limits
are described alongside each example; see also [SECURITY.md](SECURITY.md).

### `structs`: Disruptor

Disruptor uses a preallocated ring buffer, atomic reservation cursors, and per-slot
readiness sequences. Publication is lock-free and allocation-free, supporting
multiple concurrent publishers and one consumer. Pass `nil` to
`NewDisruptor[T](conf)` for the default 65,536-slot buffer and 10 ms idle check
interval. `NewDisruptorWithSize[T](size, conf)` returns a buffer and an error;
`size` must be a power of two and at least 64. The fullness check leaves one slot
unused. A custom `DisruptorConfig` sets `IdleCheckInterval` directly.

This example publishes an entry and closes the buffer from its consumer:

```go
package main

import (
    "fmt"

    "github.com/abaxoth0/Ain/structs"
)

func main() {
    d := structs.NewDisruptor[string](nil)
    if !d.Publish("Hello, World!") {
        panic("buffer full or closed")
    }
    if err := d.Consume(func(msg string) {
        fmt.Println("Received:", msg)
        d.Close()
    }); err != nil {
        panic(err)
    }
}
```

`Publish` returns `false` when full or closed; it does not wait for capacity.
`Consume` blocks and invokes the handler synchronously. Use only one consumer:
there is no coordination between multiple `Consume` calls. `Close` signals
shutdown without waiting, is not idempotent, and prevents subsequent `Consume`
calls. An already running consumer drains all accepted entries before returning;
wait for `Consume` to return yourself. A publication racing `Close` either reserves
its slot before shutdown or is rejected. Close does not wait for in-flight
publishers; the consumer waits for their payloads before completing its drain.
Slots become readable only after their payloads are written, and are reused only
after the handler returns. A paused publisher can delay ordered consumption and
shutdown draining, while other publishers can fill remaining capacity. Keep
referenced payload data immutable while it may be in use. Race instrumentation
is enabled for both publication and consumption.
`ConsumeWithReady(handler, ready)` invokes an optional readiness callback after
entering the consumer and before processing records. It does not call the
callback if the buffer was already closed; the callback must return promptly.

### `structs`: SyncQueue

`SyncQueue[T]` requires a comparable element type. Individual queue operations
use a mutex. A size limit of zero or less disables the limit; `Push` returns an
error immediately when a positive limit is reached. `Pop` returns `false` when
empty.

```go
package main

import (
    "fmt"

    "github.com/abaxoth0/Ain/structs"
)

func main() {
    queue := structs.NewSyncQueue[string](0)
    if err := queue.Push("item1"); err != nil {
        panic(err)
    }
    item, ok := queue.Pop()
    fmt.Println(item, ok)
}
```

`WaitTillEmpty` and `WaitTillNotEmpty` wait for a condition, with nonpositive
timeouts meaning no timeout and expiration returning `errs.StatusTimeout`.
`WaitTillEmptyContext(ctx)` and `WaitTillNotEmptyContext(ctx)` also observe context
cancellation and deadlines, returning `context.Canceled` or
`context.DeadlineExceeded`. If the requested queue condition holds when checked,
the wait returns nil even if the context has ended. Waits recheck the condition
under the queue mutex after every wakeup, including cancellation notifications.
They do not reserve elements or close the queue, so a later Pop can still find
the queue empty after a successful WaitTillNotEmpty.

`PreserveAndPop` calls two separately locked methods, so it is not an atomic
transaction. The queue holds one preserved value, including zero, false, an empty
string, or a nil pointer. Preserve on a nonempty queue replaces that value, and
RollBack restores it to the front at most once. Coordinate preserve/pop/rollback
sequences with other callers when transaction semantics are needed. RollBack wakes
nonempty waiters when it restores an empty queue; UnwrapAndFlush wakes empty
waiters when it removes the remaining elements.

### `structs`: Stacks

`StaticStack` has a fixed positive capacity; `DynamicStack` grows as needed and
uses an initial capacity of 16 when given a nonpositive capacity. Neither is
synchronized. Wrap a stack with `NewSyncStack` for mutex protection of individual
method calls, and access the underlying stack only through that wrapper when
sharing it. `NewSyncStack` accepts pointer and value implementations of `Stack`
and panics for nil implementations, including a nil interface or typed nil.

`PopBatch(n)` returns up to n elements, top first, in the same order as repeated
`Pop` calls. For example, after pushing `[1, 2, 3]`, `PopBatch(2)` returns `[3, 2]`.
This changes DynamicStack's previous insertion-order behavior. Returned batches
have independent slice storage, so later pushes and batch mutations cannot
change one another, including through SyncStack. Element values are shallow
copies; referenced objects remain shared. Custom Stack implementations must
provide the same batch ownership and ordering. For n <= 0, PopBatch removes
nothing and returns an empty slice with true; a positive n on an empty stack
returns nil with false.

```go
package main

import (
    "fmt"

    "github.com/abaxoth0/Ain/structs"
)

func main() {
    stack := structs.NewStaticStack[string](10)
    if !stack.Push("example") {
        panic("stack full")
    }
    item, ok := stack.Pop()
    fmt.Println(item, ok)

    shared := structs.NewSyncStack[string](structs.NewDynamicStack[string](0))
    shared.Push("shared item")
}
```

### `structs`: WorkerPool

Tasks implement `Process()`. `NewWorkerPool(ctx, nil)` defaults to a batch size
of 1 and a cancellation wait timeout of 1 second. Call `Start` once with a positive
worker count; subsequent calls do nothing, even if the first count was zero.
The queue is unbounded and task execution order is not guaranteed across workers.

Successful cancellation waits for accepted work:

```go
package main

import (
    "context"
    "fmt"

    "github.com/abaxoth0/Ain/structs"
)

type task struct{}

func (t *task) Process() {
    fmt.Println("task processed")
}

func main() {
    pool := structs.NewWorkerPool(context.Background(), nil)
    pool.Start(4)
    if err := pool.Push(&task{}); err != nil {
        panic(err)
    }
    if err := pool.Cancel(); err != nil {
        panic(err)
    }
}
```

Explicit `Cancel` and parent-context cancellation both stop admission, wake idle
workers, and drain accepted tasks. `IsCanceled` reflects either form of
cancellation. Push can race cancellation: it either returns an error or accepts a
task covered by shutdown, including a Push already in progress when the parent
ends. Worker lifetimes include both normal processing and shutdown draining.
Successful Cancel means every accepted task finished and all workers exited.

`StopTimeout` bounds how long the first Cancel waits; expiration returns an error
wrapping `errs.StatusTimeout`. It cannot interrupt Process, and accepted queued
tasks continue draining in the background after a timeout. Keep resources used by
tasks valid until `Done()` closes or `Wait(ctx)` returns nil. Wait only observes
shutdown; its context limits that caller's wait and does not cancel the pool.
It returns `ctx.Err()` if shutdown is still incomplete when that context ends.
Completed shutdown takes precedence over a canceled wait context.

The first explicit Cancel can join shutdown initiated by the parent; repeated
Cancel calls return an error. Push may queue work before Start. If cancellation
finds queued work without any workers, the pool creates one worker to finish that
work. Start cannot restart a stopping or stopped pool.

### `logger`: Logging

Console loggers write synchronously. Use a source wrapper for convenient level
methods (`Trace`, `Debug`, `Info`, `Warning`, `Error`, `Fatal`, and `Panic`):

```go
package main

import "github.com/abaxoth0/Ain/logger"

func main() {
    output := logger.NewStdOutLogger("")
    source := logger.NewSource("example", output)
    source.Info("application started", nil)
}
```

`NewStdErrLogger(prefix)` writes to stderr. Console loggers use the default
configuration, which filters Debug and Trace. `Fatal` calls `os.Exit(1)`;
`Panic` panics. These actions do not flush asynchronous file logging.

For file logging, `NewFileLogger` requires a non-nil `*FileLoggerConfig`. Set
`Path` to a directory whose parent exists. Call `Init()`, then `Start()`, then
`Stop(true)` and check each error. Files use the name
`<ApplicationName>_<AppInstance>[<timestamp>].log`; `TimeLayout` defaults to
`2006-01-02_15-04-05`. `SerializerProducer` defaults to a new `JSONSerializer`;
provide a fresh serializer per factory call. Default output is one JSON object
per line. `FilePerm` defaults to `0644` for regular files. `DirPerm` defaults to
`0755` for new directories; existing directory permissions are unchanged. Both
modes are subject to the process umask on POSIX systems. For private logs, set
`FilePerm: 0600` and, if desired, `DirPerm: 0700`.

`NewSource` uses the wrapped FileLogger's application name and instance when
constructing entries, including entries queued before Start. Other loggers use
the default identity. `NewLogEntry` keeps the default identity;
`NewLogEntryWithConfig` uses the supplied identity. Logging and forwarding retain
an entry's existing service and instance, even when the destination has different
settings or the entry's identity fields are empty.

Treat a `FileLogger` as a single-use instance. Start establishes consumer
readiness before returning, so immediate Start/log/Stop is supported. Entries may
be queued before Start; processing requires Init followed by Start. Stop closes
admission, drains both the ring consumer and fallback workers, then syncs and
closes their output. Calls arriving after shutdown starts are rejected.

`StopTimeout` bounds each Stop caller's wait, including in non-strict mode. A
timeout wraps `errs.StatusTimeout`; cleanup continues while the file remains open
for active writers. A later Stop can wait again, and `Done()` closes only after
writers exit and cleanup finishes. Completed Stop calls return the retained
cleanup result without repeating sync or close. Strict Stop additionally reports
processing and sync errors; sync failure does not skip closing the output. Start cannot restart
a stopping or stopped instance.

`Err()` reports a snapshot of failure counts and the first cause for each stage:
admission, serialization, writing, consumer execution, sync, and close. This state
uses bounded memory. `Errors()` provides up to 64 buffered notifications without
blocking processing; notifications can be dropped when full, while Err retains
the counts and first causes. The channel closes when cleanup completes. Log calls
rejected after completion still update Err. Strict Stop includes these failures;
non-strict Stop reports incomplete shutdown, consumer errors, and close errors.
A successful strict Stop means admitted records were processed, written in full,
and output sync/close succeeded. Stop cannot protect against later disk failure.

Failed records are not retried; a partial write may leave an incomplete record.
Log returns no delivery status, so observe Errors/Err and check strict Stop.
Overflow uses a worker pool and can reorder entries. Configure forwarding before logging; forwarding changes
are not synchronized. Forwarded entries bypass the target's filtering and
critical-level side effects. Entries and their metadata are shared by reference;
keep them immutable while logging is in progress.

`FileLogger.Stop(strict bool)` differs from `ConcurrentLogger.Stop()`, so
`*FileLogger` does not implement `ConcurrentLogger`.

### `errs`: Status errors

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/abaxoth0/Ain/errs"
)

func main() {
    err := errs.NewStatusError("Invalid request", http.StatusBadRequest)
    fmt.Println(err.Error(), err.Status())
}
```

### `common`: Retry and helpers

`Retry` blocks until success or an attempt limit. A nil config defaults to
3 attempts, a 5-second maximum backoff, and a 1 ms scale. The initial call counts
as an attempt; `MaxAttempts <= 0` retries indefinitely and `MaxAttempts == 1`
executes only the initial call. There is no context cancellation or error filter.

```go
package main

import (
    "errors"
    "fmt"
    "time"

    "github.com/abaxoth0/Ain/common"
)

func main() {
    attempts := 0
    err := common.Retry(func() error {
        attempts++
        if attempts < 2 {
            return errors.New("temporary failure")
        }
        return nil
    }, &common.RetryConfig{
        MaxAttempts: 3,
        MaxBackoff: 5 * time.Second,
        BackoffScale: 100 * time.Millisecond,
    })
    if err != nil {
        panic(err)
    }
    fmt.Println("operation succeeded")
}
```

All attempts run synchronously in the caller, and no callback runs after
`Retry` returns. Callback panics propagate to the caller. Backoff is
`2^attempts * BackoffScale` plus 0–9 ms of jitter, capped by `MaxBackoff`.
Currently, a zero `MaxBackoff` clamps the delay to zero. The implementation may
mutate a zero `BackoffScale`, so use a separate configuration per concurrent call.

Other helpers include `Ternary` (both value arguments are evaluated), `MapKeys`,
`MapValues`, and `MapPop` (returns a key's value and deletes the key, or returns the zero value
when absent).
Map helper results follow unspecified Go map iteration order where applicable;
these helpers do not synchronize concurrent map access. `structs.Meta` is also a
plain map, and its global well-known property registry is unsynchronized.

## Performance

The current benchmarks have measurement and comparability limitations and do not
establish throughput rankings, latency bounds, or suitability for a production
workload. See [BENCHMARKS.md](BENCHMARKS.md) for the method review and bounded
smoke commands.

## Contributing and license

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and validation commands.
Ain is licensed under the [MIT License](LICENSE).

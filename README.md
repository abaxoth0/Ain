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

Disruptor uses a preallocated ring buffer and atomic cursors. Pass `nil` to
`NewDisruptor[T](conf)` for the default 65,536-slot buffer and 10 ms idle check
interval. `NewDisruptorWithSize[T](size, conf)` returns a buffer and an error;
`size` must be a power of two and at least 64. The fullness check leaves one slot
unused. A custom `DisruptorConfig` sets `IdleCheckInterval` directly.

This example publishes before consuming to avoid overlapping publication and
consumption:

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
calls. An already running consumer attempts to drain entries before returning;
coordinate producers and wait for `Consume` to return yourself.

Concurrent publication and consumption currently have a correctness limitation:
`Publish` advances the writer cursor **before** assigning the buffer slot, so a
consumer can observe a slot before its value is written. Multiple publishers can
also expose unfinished earlier slots. The `//go:norace` directives on `Publish`
and `Consume` suppress instrumentation. Passing race tests does not establish
that these accesses are safe. Do not rely on concurrent delivery guarantees
until this publication ordering is fixed.

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
timeouts meaning no timeout. They do not reserve elements or close the queue.
`PreserveAndPop` calls two separately locked methods, so it is not an atomic
transaction. `RollBack` uses the zero value as the absence marker and cannot
restore a preserved zero value. `UnwrapAndFlush` and `RollBack` do not notify
condition waiters; avoid relying on them to wake waiting goroutines.

### `structs`: Stacks

`StaticStack` has a fixed positive capacity; `DynamicStack` grows as needed and
uses an initial capacity of 16 when given a nonpositive capacity. Neither is
synchronized. Wrap a stack with `NewSyncStack` for mutex protection of individual
method calls, and access the underlying stack only through that wrapper when
sharing it.

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
of 1 and a shutdown drain timeout of 1 second. Call `Start` once with a positive
worker count; subsequent calls do nothing, even if the first count was zero.
The queue is unbounded and task execution order is not guaranteed across workers.

Track task completion explicitly when it matters:

```go
package main

import (
    "context"
    "fmt"

    "github.com/abaxoth0/Ain/structs"
)

type task struct{ done chan struct{} }

func (t *task) Process() {
    defer close(t.done)
    fmt.Println("task processed")
}

func main() {
    pool := structs.NewWorkerPool(context.Background(), nil)
    pool.Start(4)
    done := make(chan struct{})
    if err := pool.Push(&task{done: done}); err != nil {
        panic(err)
    }
    <-done
    if err := pool.Cancel(); err != nil {
        panic(err)
    }
}
```

`Cancel` marks the pool canceled and signals its context; later `Push` calls
return errors, and repeated `Cancel` calls return an error. Serialize cancellation
with producers. Cancellation does not interrupt `Process`, wake workers blocked
on an empty queue, or join all worker goroutines. Its wait group covers active
normal batches, not all queued tasks or shutdown draining. `StopTimeout` is
checked between drain batches and cannot interrupt a blocked task; drain errors
are discarded. A successful `Cancel` does not guarantee that all tasks finished.
Canceling the parent context alone does not set `IsCanceled` or reject `Push`.

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
per line. `FilePerm` defaults to `0644` and is also passed to directory creation;
configure directory permissions appropriate to your deployment.

Treat a `FileLogger` as a single-use instance: `Stop` closes its ring buffer and
file, and `Start` does not recreate them. Stop producers before shutdown and
serialize lifecycle calls. `Stop(true)` attempts to drain, sync, and close the
file, but inherits the Disruptor and WorkerPool limitations above. Serialization
failures discard entries and write errors are ignored; neither `Log` nor a
successful `Stop` guarantees durable delivery. Overflow uses a worker pool and
can reorder entries. Configure forwarding before logging; forwarding changes
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

After a failure, retries run in a goroutine while the caller waits. Backoff is
`2^attempts * BackoffScale` plus 0–9 ms of jitter, capped by `MaxBackoff`.
Currently, a zero `MaxBackoff` clamps the delay to zero. The implementation may
mutate a zero `BackoffScale`, so use a separate configuration per concurrent call.
There is also an attempt-limit bug: after returning the last error, the retry
goroutine continues, can call the operation again, and can block forever sending
a result. Account for this before using retries for operations with side effects.

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

# Ain

A Go library containing reusable utilities and data structures.

## Overview

Ain (formal name of Epsilon Tauri star) is a collection of common Go packages and utilities. This library provides battle-tested, production-ready implementations of commonly needed functionality.

## Motivation

This library was created to solve a common problem: fixing the same bugs and implementing the same features across multiple projects. Instead of copying code between repositories, this centralized library ensures that bug fixes and improvements benefit all projects simultaneously, reducing maintenance overhead and improving code quality across the board.

## License

This library is licensed under the MIT License.
Please see the [LICENSE](LICENSE) file for the full license text.

## Features

- **High-Performance Data Structures**: Lock-free ring buffer (Disruptor), thread-safe queues, worker pools
- **Logging System**: Flexible, multi-level logging with forwarding capabilities
- **Error Handling**: Structured error types with HTTP status codes
- **Common Utilities**: Reusable helper functions

## Installation

```bash
go get github.com/abaxoth0/Ain
```

## Requirements

- Go 1.23.0 or later

## Packages

### `structs` - Concurrent Data Structures

#### Disruptor
A high-performance, lock-free ring buffer implementation based on the LMAX Disruptor pattern.

```go
disruptor := structs.NewDisruptor[string]()
go disruptor.Consume(func(msg string) {
    fmt.Println("Received:", msg)
})

disruptor.Publish("Hello, World!")
```

#### SyncQueue
Thread-safe FIFO queue with optional size limits and blocking operations.

```go
queue := structs.NewSyncQueue[string](0) // 0 = no size limit
queue.Push("item1")
queue.Push("item2")

item, ok := queue.Pop()
```

#### WorkerPool
Concurrent task processing with configurable worker count and graceful shutdown.

```go
type MyTask struct{ data string }
func (t *MyTask) Process() { /* process task */ }

pool := structs.NewWorkerPool(context.Background(), nil)
pool.Start(4) // 4 workers
pool.Push(&MyTask{data: "test"})
```

### `logger` - Flexible Logging System

Multi-level logging with support for forwarding, configuration, and different output targets.

```go
logger.NewLoggerEntry(logger.InfoLogLevel, "Hello", "optional error")
logger.Stdout.Log(entry) // Built-in stdout logger
```

Features:
- Multiple log levels (Trace, Debug, Info, Warn, Error, Fatal, Panic)
- Forwarding capabilities to chain loggers
- Configurable output formatting
- Concurrent logger support with background processing

### `errs` - Error Handling

Structured error types with HTTP status code integration.

```go
err := errs.NewStatusError("Invalid request", http.StatusBadRequest)
// err.Error() returns "Invalid request"
// err.Status() returns 400
```

### `common` - Utility Functions

Common helper functions for everyday Go programming.

```go
// Ternary operator
result := common.Ternary(condition, "yes", "no")
```

## Performance

The Disruptor implementation provides high-throughput, low-latency messaging suitable for:
- Event processing systems
- Message queues
- Producer-consumer patterns
- High-frequency trading systems

Benchmark results and performance characteristics are documented in the [BENCHMARKS.md](BENCHMARKS.md).

## Contributing

**Current Performance Summary (100 iterations, statistical analysis):**
- **Single Operations**: Disruptor 69.3B ops/s (0.01±0.00ns), Channels 42.5M ops/s (23.55±0.21ns), SyncQueue 14.2M ops/s (70.43±11.0ns)
- **Throughput**: Channels 876K±42K items/sec (most stable), Disruptor 834K±145K (competitive, higher variance), SyncQueue 832K±81K (surprisingly competitive)
- **Memory Efficiency**: Disruptor & Channels: 0 allocations, SyncQueue: 218K±839 B/op (47 allocs)
- **Best Use Cases**: Channels for consistency, Disruptor for high-concurrency scenarios, SyncQueue when features outweigh performance

All benchmarks use automated script (`scripts/bench.py`) for 100-iteration statistical analysis.

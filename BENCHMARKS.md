# Benchmark methods and limitations

The suite in [structs/benchmark_test.go](structs/benchmark_test.go) compares
Disruptor, SyncQueue, and buffered Go channels using `TestData` (an `int64` ID and
32 bytes of padding). It contains three `SingleOperation` benchmarks and three
`Throughput` benchmarks. There is no `benchmark_simple_test.go`, concurrent
benchmark group, latency histogram, or multi-producer stress benchmark in the
current suite.

## What the suite measures

| Benchmark | Timed work | Limitations |
|-----------|------------|-------------|
| `Disruptor_SingleOperation` | Publishes to a ring with `1<<20` slots, without a consumer | Loop stops at `min(b.N, (1<<20)-2)`, but Go divides elapsed time by `b.N`. Invalid normalization once the cap is reached. |
| `SyncQueue_SingleOperation` | `b.N` pushes to an unbounded queue | The queue grows throughout the run; timed slice growth is part of the result. No consumer or compaction is exercised. |
| `Channel_SingleOperation` | `b.N` sends to a channel with capacity `b.N` | Buffer allocation is excluded by `ResetTimer`. Calibration can request a very large buffer. No receiving is measured. |
| `Disruptor_Throughput` | One producer publishes 1,000 items per iteration to one persistent consumer | The ring and consumer are reused; shutdown is timed. The default consumer idle interval is 10 ms. |
| `SyncQueue_Throughput` | One producer pushes 1,000 items per iteration to one consumer polling `Pop` | A new queue and consumer are created outside timing per iteration. Slice growth and compaction can allocate during timing. |
| `Channel_Throughput` | One producer sends 1,000 items per iteration to one consumer through a 1,024-slot channel | A new channel and consumer are created outside timing per iteration. |

All throughput benchmarks count processed entries with an atomic counter and
poll for completion using 1 ms sleeps. Those sleeps, consumer wait strategies,
and scheduling affect the measurement; the benchmark is not a per-message
latency test. Consumers count entries without checking every payload's identity
or order, so these runs do not establish delivery correctness. See the
[Disruptor concurrency limitation](README.md#structs-disruptor).

Additional interpretation limits:

- In throughput benchmarks, `ns/op`, `B/op`, and `allocs/op` describe one batch of
  **1,000 items**, rather than one item. Single-operation metrics have different
  units and must be analyzed separately.
- Each iteration overwrites the custom `items/sec` metric using `ReportMetric`;
  the printed value is the **last iteration's** rate, not a rate computed from the
  entire run. Go's `ns/op` uses its accumulated benchmark timer, with different
  timing boundaries from that custom metric.
- `SetBytes(40)` reports 40 bytes per batch although a batch transfers 1,000
  40-byte values. The printed `MB/s` therefore does not represent payload
  throughput.
- Setup and shutdown timing differs among implementations. Zero timed
  allocations do not mean zero memory use, zero setup cost, or no GC pauses.
  A buffered channel also allocates its storage up front.
- These tests do not measure multiple producers or consumers, contention scaling,
  tail latency (P50/P99/P99.9), real-time bounds, or application performance.
  They provide no basis for claiming that any implementation wins in production.

## Running a bounded smoke check

Run from the repository root. A fixed small iteration count avoids the
single-operation calibration cap and limits queue/channel memory consumption.
These commands exercise the harness; they do not repair its comparison issues.

```bash
# Compile and run every benchmark with 100 iterations per sample.
go test -run='^$' -bench=. -benchmem -benchtime=100x -count=1 ./structs

# Repeated throughput batches; save raw output outside the repository.
go test -run='^$' -bench='_Throughput$' -benchmem -benchtime=100x -count=5 ./structs > /tmp/ain-throughput.txt
```

Record the commit, `go version`, OS, CPU, `GOMAXPROCS`, buffer capacities, payload,
producer/consumer counts, and exact command with any reported measurement.
The numeric suffix on a Go benchmark name describes `GOMAXPROCS`, not necessarily
the machine's physical core count. A successful smoke run does not validate
concurrent safety: Disruptor's `Publish` and `Consume` disable race instrumentation.

## Existing Python tools

[scripts/bench.py](scripts/bench.py) is a legacy collector requiring Python 3.10+
(it uses structural pattern matching). For example:

```bash
python3 scripts/bench.py -c 5 --path ./structs
```

It runs all tests and benchmarks with Go's default benchmark calibration and
`-timeout 0`; it can use substantial memory and has no test timeout. Use the
bounded Go commands above for smoke checks. `-c` controls repeated benchmark
samples, not the number of operations inside each sample.

The collector writes a timestamped CSV into the **current working directory**,
deletes the intermediate `.result` output, and performs no statistical analysis.
Its `op/s` column stores Go's custom `items/sec` value for throughput rows and
`N\A` for single-operation rows. The parser assumes particular output shapes and
integer throughput values; Go can print fractional custom metrics, so conversion
can fail. Preserve raw Go output when collecting measurements.

[scripts/analyze_simple.py](scripts/analyze_simple.py) computes arithmetic means,
population standard deviations, minima, and maxima for the checked-in CSV. It
hardcodes an absolute path and a heading claiming 100 iterations, rather than
accepting a report path and deriving the heading from the data. Its sample counts
at the end reflect the loaded rows. Its derived Disruptor operations-per-second
value inherits the invalid single-operation normalization. Neither script fixes
the measurement problems above.

## Before publishing new comparisons

A revised harness should execute exactly `b.N` operations, use comparable setup
and cleanup boundaries, verify payload delivery, and report batch and item units
consistently. It should aggregate throughput over all timed work, set byte counts
for the full batch, and separate completion polling from any latency measurement.
Bound buffer growth and include workloads with explicit capacities, wait
strategies, and producer/consumer counts. Resolve concurrent correctness issues
before drawing performance conclusions, then retain raw samples and environment
metadata with any summary.

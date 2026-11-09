# Review remediation validation

Validated on 2026-11-09 against implementation commit `8215f9f` and the phase 6
test, CI, and documentation changes in this commit. The original review covered
`59cddd9` on 2026-10-29; its R1–R15 identifiers are retained below.

All 15 findings have implemented fixes and passing permanent regressions in the
tested environments. This record covers correctness and resource ownership;
benchmark smoke results do not establish performance rankings. The CI follow-up
below records native runner results and corrections to the initial workflow.

## Finding checklist

Every row was revalidated with fresh test executions and inspection of the
repaired implementation. The tests recreate the original failure scenarios;
they assert payloads, completion, resource ownership, or errors. The original
external `/tmp/ain-review` probe files are no longer available. Their reproductions
now live in the tracked tests linked here.

| Finding | Implemented contract and fix commit | Permanent regressions |
| --- | --- | --- |
| R1 · Revalidated | Per-slot readiness publishes payloads after assignment; CAS reservation and Close preserve accepted work. Publication remains lock-free and allocation-free, with race instrumentation. `b123d35` | [disruptor_delivery_test.go](structs/disruptor_delivery_test.go): `TestDisruptorPendingPublication`, `TestDisruptorPayloadsThroughWraparound`, `TestDisruptorCloseDuringPublication`, `TestDisruptorSequenceRollover`, `TestDisruptorPublishDoesNotAllocate` |
| R2 · Revalidated | Retry runs synchronously and stops at its attempt limit, with no later callback or retry goroutine. `f8b362e` | [retry_termination_test.go](common/retry_termination_test.go): `TestRetryStopsAfterExhaustion`, `TestRetryAttemptLimits`, `TestRetryCallbackPanicPropagates` |
| R3 · Revalidated | Queue predicates, cancellation registration, and notifications use the same mutex; short or expired waits return without an unrelated mutation. `6ca17ce` | [sync-queue_wait_test.go](structs/sync-queue_wait_test.go): `TestSyncQueueShortTimeout`, `TestSyncQueueContextAlreadyDone`, `TestSyncQueueCancellationBeforeRegistration`, `TestSyncQueueWaitAndMutationRaces` |
| R4 · Revalidated | Worker lifetimes include normal batches and shutdown draining. Successful Cancel joins all accepted tasks; timeout reports incomplete shutdown while work continues. `dcdc413` | [worker-pool_lifecycle_test.go](structs/worker-pool_lifecycle_test.go): `TestWorkerPoolImmediateCancelCannotReportBlockedDrainComplete`, `TestWorkerPoolImmediateShutdownCompletesAcceptedTasks`, `TestWorkerPoolTimeoutRetainsWorkUntilCompletion` |
| R5 · Revalidated | Start establishes consumer readiness before returning; Stop joins ring and fallback writers. `721c1f0` | [file-logger_lifecycle_test.go](logger/file-logger_lifecycle_test.go): `TestFileLoggerImmediateStartLogStop`, `TestFileLoggerStartDoesNotWaitForQueuedSerialization`; [file-logger_delivery_test.go](logger/file-logger_delivery_test.go): `TestFileLoggerShutdownWaitsForFallbackDrain` |
| R6 · Revalidated | New directories use DirPerm (default 0755), independently of FilePerm; existing modes stay intact. `8b7929b` | [file-logger_permissions_test.go](logger/file-logger_permissions_test.go): `TestFileLoggerInitPermissions`, including new/default/private and existing directories, run as UID 1000 on Linux |
| R7 · Revalidated | Cancellation wakes idle queue waiters and worker completion is joined. `dcdc413` | [worker-pool_lifecycle_test.go](structs/worker-pool_lifecycle_test.go): `TestWorkerPoolIdleShutdownJoinsWorkers`; [sync-queue_wait_test.go](structs/sync-queue_wait_test.go): `TestSyncQueueContextCancellation` |
| R8 · Revalidated | A timed-out Stop permits retry and keeps active output valid. Cleanup attempts close even after sync failure and retains errors. `721c1f0`, `2aed328` | [file-logger_lifecycle_test.go](logger/file-logger_lifecycle_test.go): `TestFileLoggerStopTimeoutCanFinishCleanup`, `TestFileLoggerCleanupRetainsSyncAndCloseErrors`, `TestFileLoggerTimeoutWhileOutputBlocked`, `TestFileLoggerTimeoutKeepsFallbackOutputOpen`, `TestFileLoggerConsumerStartFailureCleansUp` |
| R9 · Revalidated | Parent cancellation updates status and closes admission. Accepted in-flight pushes remain covered by shutdown. `dcdc413` | [worker-pool_lifecycle_test.go](structs/worker-pool_lifecycle_test.go): `TestWorkerPoolParentCancellationRejectsWork`, `TestWorkerPoolAlreadyCanceledParent`, `TestWorkerPoolPushRacingCancellation`, `TestWorkerPoolInFlightPushIsCoveredByParentShutdown` |
| R10 · Revalidated | Stack batches own independent slice storage and use repeated-Pop order across static, dynamic, and synchronized implementations. `8215f9f` | [stack_contract_test.go](structs/stack_contract_test.go): `TestStackPopBatchContract`, `TestStackPopBatchOwnership`, `TestSyncStackReturnedBatchConcurrentUse` |
| R11 · Revalidated | Preservation has an explicit presence flag; zero, false, empty string, and nil pointer survive rollback exactly once. `6ca17ce` | [sync-queue_rollback_test.go](structs/sync-queue_rollback_test.go): `TestSyncQueueZeroValueRollback` |
| R12 · Revalidated | RollBack and UnwrapAndFlush notify waiters when they satisfy their predicates. `6ca17ce` | [sync-queue_wait_test.go](structs/sync-queue_wait_test.go): `TestSyncQueueRollbackAndFlushWakeWaiters`, `TestSyncQueueWaitersRecheckCondition` |
| R13 · Revalidated | Source obtains FileLogger identity; explicit, empty, default, and forwarded entry identities remain intact. `ef07690` | [source_identity_test.go](logger/source_identity_test.go): `TestSourceFileLoggerIdentityAndForwarding`, decoding both origin and destination output |
| R14 · Revalidated | Serialization, writes/short writes, fallback rejection, and cleanup errors are observable through Err/Errors and strict Stop. Retained error state and notification buffering are bounded. `2aed328` | [file-logger_errors_test.go](logger/file-logger_errors_test.go): `TestFileLoggerStrictStopReportsRecordFailures`, `TestFileLoggerReportsFallbackRejection`, `TestFileLoggerFailureObservationIsBounded`, `TestFileLoggerErrorNotificationWhileRunning` |
| R15 · Revalidated | NewSyncStack accepts valid value implementations and checks nil only for supported reflected kinds, including nil interfaces. `8215f9f` | [stack_contract_test.go](structs/stack_contract_test.go): `TestNewSyncStackValueImplementation`, `TestNewSyncStackRejectsNil` |

## Integrated and stress checks

The complete suite and a focused run of the regressions above passed with the
race detector, five repetitions at each of GOMAXPROCS 1 and 4, and shuffled test
order. The complete shuffled suite also passed using the official stable
Go 1.27.3 toolchain.

- Ring delivery uses a 64-slot buffer, one or four publishers, and 10,000 payloads
  per publisher. Every ID and all 128 payload fields are checked through wraparound.
- `TestSyncQueueContextConsumerDelivery` joins four consumers after checking 4,000
  unique values from four producers.
- WorkerPool regressions cover zero, one, three, and eight workers; several batch
  sizes; parent/explicit cancellation; and admission racing cancellation. Per-task
  counts establish exactly-once completion for accepted work. Blocked callbacks
  make Cancel return a timeout while Done remains open; release permits completion.
- `TestFileLoggerConcurrentPayloadDelivery` decodes 4,000 unique records from four
  publishers. `TestFileLoggerShutdownWaitsForFallbackDrain` forces overflow with
  a 64-slot ring and a single fallback worker, then checks all 100 records.
- `TestFileLoggerConcurrentAdmissionAndStop` accounts for attempted records as
  persisted once or observably rejected. Shutdown tests hold a serializer,
  fallback task, Write, Sync, or Close across the timeout and release it afterward.
  They verify output remains usable while needed and is closed after cleanup.
- `TestFileLoggerIdleShutdownAndRestartRejection` checks 25 create/start/stop cycles
  per invocation, joining the consumer and fallback workers through their owned
  completion channels. No global goroutine-count heuristic is used for these joins.

## Toolchains and platforms

Host: Linux 7.2.14-arch1-1, amd64, UID 1000 (non-root), AMD Ryzen 9 9900X.
Go caches, downloaded toolchains, coverage output, and cross-platform executables
were kept under `/tmp`. The installed compiler identifies itself as
`go1.27.3-X:nodwarf5`; official downloaded toolchains were exercised separately.
[Go's release metadata](https://go.dev/dl/?mode=json) identified Go 1.27.3 as stable
on the validation date.

| Environment | Result and scope |
| --- | --- |
| Linux/amd64, official Go 1.23.0 | Full race suite, vet, and build passed at the exact minimum declared in go.mod |
| Linux/amd64, official Go 1.23.12 | Full race suite, vet, and build passed; matches CI's 1.23.x series |
| Linux/amd64, official Go 1.27.3 | Full atomic-coverage race suite, shuffled race stress suite, vet, and build passed |
| Linux/amd64, installed Go 1.27.3-X:nodwarf5 | Full ordinary/coverage suite, race suite, focused shuffled stress checks, vet, build, and benchmark smoke passed |
| Linux/386, installed Go toolchain, CGO disabled | Full suite passed natively; execution needed to run outside the sandbox's syscall filter |
| Windows/amd64, Wine 11.21 | Full suite passed with CGO disabled, then with race/atomic coverage enabled using MinGW and official Go 1.27.3; native Windows results are recorded in the CI row below |
| macOS/amd64 and macOS/arm64, CGO disabled | All package test binaries cross-compiled; execution and race checks were not run locally |
| Native GitHub Actions runners: Ubuntu, Windows, macOS | All six race/atomic-coverage jobs passed on Go 1.23.x and stable in the corrected PR #14 run linked below |

POSIX directory/file permission assertions ran without root privileges on Linux.
The Windows test intentionally omits POSIX mode assertions. After completed
shutdown, closure checks use a zero-length Write expecting os.ErrClosed; Stat on
a closed Windows handle may instead return an invalid-handle error. Three older
fixtures now close initialized files before TempDir cleanup. Read-only test
handles explicitly discard their deferred Close results to satisfy lint.

The Wine adapter captured each executable's output in a temporary file and
returned its exit status. This prevents Wine helper processes from retaining the
Go runner's output pipes after the executable exits. These adapter/fixture issues
were observed and corrected during validation; no production behavior changed in
phase 6.

## Commands and quality checks

Run from the repository root. Set a writable cache when the default Go cache is
unavailable. The following are the core native commands used; the stress command
is bounded and includes every regression in the checklist.

```sh
export GOCACHE=/tmp/ain-revalidation-cache

go test -count=1 -timeout=60s -cover ./...
go test -race -count=1 -timeout=60s -coverprofile=/tmp/ain-phase6-coverage.out -covermode=atomic ./...
go test -race -count=5 -cpu=1,4 -shuffle=on -timeout=60s ./...
go vet ./...
go build ./...
go test -run='^$' -bench=. -benchmem -benchtime=100x -count=1 -timeout=30s ./structs
python3 -m unittest discover -s scripts -p 'test_*.py'
git ls-files -z '*.go' | xargs -0 gofmt -l
```

Use `GOTOOLCHAIN=go1.23.0`, `go1.23.12`, or `go1.27.3` with
`GOMODCACHE=/tmp/ain-phase6-modcache` to select downloaded toolchains. Both the
minimum-version and stable-version checks use the same module without changing
go.mod. Format checks produced no paths. All nine workflow regression tests
passed, including platform/version coverage, a shared test shell, and the tested
minimum linter version.

Local golangci-lint 2.15.0 reported zero issues for the tracked Go packages:

```sh
GOLANGCI_LINT_CACHE=/tmp/ain-phase6-lint-cache golangci-lint run --timeout=2m ./common ./errs ./logger ./structs
```

The ignored local `cmd/` example is included by Go's `./...` expansion but is absent
from a clean Git checkout. It was included in full test/build runs; its existing
unchecked calls and unused function were excluded from the tracked-package lint
result. CI now pins golangci-lint 2.15.0; its official release binary was also
tested locally against the official Go 1.27.3 toolchain. The archive SHA-256
matched the GitHub release asset digest. CI-pinned gosec 2.30.0 reproduced G115
before the capacity comparison cleanup and reported zero findings afterward.

Full coverage with the race detector was common 55.9%, logger 93.4%, structs 98.3%,
and errs 0.0% (no tests). These percentages describe this run and are not the
criterion for closing findings. All six benchmarks completed at 100 iterations;
the normalization, timing, and parsing limitations in [BENCHMARKS.md](BENCHMARKS.md)
remain unchanged.

Cross-compilation used `GOOS=darwin GOARCH=amd64` and `GOARCH=arm64` with
`CGO_ENABLED=0 go test -exec=/usr/bin/true -count=1 ./...`. The true adapter only
confirms compilation; its printed package results do not mean tests executed.
Windows used `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test` with a Wine execution
adapter and `-count=1 -timeout=60s`. Linux/386 used
`GOARCH=386 CGO_ENABLED=0 go test -count=1 -timeout=60s ./...`.
The CI follow-up repeated Windows tests with `CGO_ENABLED=1`,
`CC=x86_64-w64-mingw32-gcc`, `-race`, `-covermode=atomic`, and a coverage profile;
all four tracked packages passed under Wine.

## Open verification and remaining limits

Native Windows and macOS execution is unavailable locally; remote runner results
are recorded below. The CI matrix runs Ubuntu, Windows, and macOS at Go 1.23.x and
stable, with coverage uploaded only from Ubuntu stable. Other architectures and
operating systems are outside this validation record. CodeQL, actionlint, and
GoReleaser were not run locally. No release was performed.

The repaired APIs retain their documented contracts: one Disruptor consumer and
immutable referenced payloads; separately coordinated queue preservation
sequences, logger configuration, forwarding, and metadata; shallow stack element
copies; and no forced interruption of running callbacks. Shutdown deadlines limit
the caller's wait, while accepted work and safe resource cleanup continue. Failed
log records are observable but are not retried, and partial writes can leave an
incomplete record. Fatal logging does not flush asynchronous output. See
[README.md](README.md) and [SECURITY.md](SECURITY.md) for these usage requirements.


## CI follow-up

The [initial PR #14 run](https://github.com/abaxoth0/Ain/actions/runs/42685003820)
passed native Ubuntu and macOS race/coverage suites on both Go 1.23.x and stable,
plus Build and Workflows. Four jobs failed:

- Both Windows jobs failed before executing tests: PowerShell split the coverage
  argument and Go attempted to load `.out` as a package. The test step now selects
  Bash explicitly on every platform; a workflow regression guards that choice.
- golangci-lint 2.13.2 could not decode Go 1.27.3 export data (version 5). The
  workflow now pins the locally verified official 2.15.0 binary; another regression
  prevents restoring an older, unverified pin.
- gosec 2.30.0 flagged the signed capacity conversion in Disruptor.Publish. The
  comparison now uses the existing unsigned bufferMask, computed as size minus one
  after constructor validation. Capacity and wraparound behavior stay covered by
  the existing regressions, with no new suppression or publication allocation.

Local verification after these corrections passed the full race suite at Go
1.23.0 and 1.27.3, vet, build, five Disruptor race repetitions at GOMAXPROCS 1 and 4,
all nine workflow tests, the official pinned linter, and gosec with zero findings.
The Windows race/coverage suite also passed under Wine with official Go 1.27.3.
The [corrected PR #14 run](https://github.com/abaxoth0/Ain/actions/runs/42686445195)
for code commit `73ee81a` passed all 10 jobs: the six native Ubuntu/Windows/macOS
race-and-coverage combinations at Go 1.23.x and stable, Lint, Security, Build,
and Workflows. Native Windows and macOS verification is complete for these
runner/toolchain combinations.

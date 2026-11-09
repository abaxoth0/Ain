# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Run the CI race suite on Linux, Windows, and macOS with Go 1.23.x and stable.
- Record integrated verification of all 15 review findings, their regression tests, and platform coverage in VALIDATION.md.

### Fixed

- Release file handles in logger test fixtures before temporary-directory cleanup on Windows.
- Return independent DynamicStack batches in top-first order, matching StaticStack and repeated Pop calls, including through SyncStack.
- Accept valid value implementations in NewSyncStack and consistently reject nil implementations without reflection panics.
- Use FileLogger application identity for Source entries while preserving explicit and forwarded entry identities.
- Expose FileLogger admission, serialization, write, and cleanup failures through Err/Errors and strict Stop, including short writes.
- Synchronize FileLogger consumer startup, join every writer before closing output, and allow shutdown cleanup to finish after timeouts or sync errors.
- Coordinate WorkerPool admission and parent cancellation, wake idle workers, join all accepted work on successful shutdown, and expose completion after cancellation timeouts.
- Make queue timeouts reliable, add context-aware waits, notify waiters after rollback and flush, and preserve zero-valued rollback elements.
- Create usable log directories with a separate DirPerm setting (default 0755), preserving FilePerm restrictions and existing directory modes.
- Stop Retry at its attempt limit without further callbacks or a leaked goroutine; execute retries synchronously.
- Restore lock-free, allocation-free Disruptor publication with per-slot readiness, coordinate Close with reservations, and restore race instrumentation.
- Streamline benchmark guidance and README links.
- Correct README constructors and lifecycle examples, document concurrency and shutdown limits, and remove unsupported benchmark claims.
- Align contributor guidance and security notes with the implementation and repository conventions.
- Run CI on dev and main, validate workflows, and test Go 1.23 and the current stable version.
- Update lint and security tooling and resolve existing findings.
- Gate library releases on CI, fetch release history, and grant release publishing permissions.
- Remove binary packaging and the obsolete SLSA binary release workflow.

## [v1.2.1] - 2026-04-17

### Added

- TimeLayout option in FileLoggerConfig. This layout applied only to the name of log files.
- MapValues() and MapKeys() helpers.

### Fixed

- Fix file name template of log files. Previously it contained ":", which made it invalid on Windows.

## [v1.2.0] - 2026-02-15

### Added

- Stack - classic LIFO stack in several implementations:
    - StaticStack - stack with fixed capacity
    - DynamicStack - stack with unlimited capacity that grows dynamicaly
    - SyncStack - provides thread-safe wrappers for the methods of Stack interface

### Changed

- Logger:
    - Add DI for FileLogger (FileLoggerConfig.SerializerProducer property): Now FileLogger relies on Serializer interface to encode logs.
    This change removes hard-lock on JSON-lines log format.

### Removed

- All third-party dependencies

## [1.1.0] - 2026-02-08

### Added

- Retry mechanism - useful feature for handling resilient operations:
    - Exponential component increases delay with each failure
    - Jitter prevents synchronized retry attempts across multiple clients
    - MaxBackoff caps the delay to prevent excessive waits

### Changed

- Logger:
    - Rework and improve FileLogger initialization
    - Remove global states
    - Fix naming inconsistencies
    - Refactor some identifiers names
    - Other minor changes

## [1.0.0] - 2026-02-04

### Added

- All

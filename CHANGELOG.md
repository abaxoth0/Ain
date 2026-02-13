# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [UNRELEASED]

### Added

- Stack - classic LIFO stack in several implementations:
    - StaticStack - stack with fixed capacity
    - DynamicStack - stack with unlimited capacity that grows dynamicaly
    - SyncStack - provides thread-safe wrappers for the methods of Stack interface

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


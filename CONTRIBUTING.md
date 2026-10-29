# Contributing to Ain

Thank you for your interest in contributing to Ain! This guide will help you get started.

## Prerequisites

- **Go 1.23.0 or later**
- **Python 3.10+** (optional, for benchmark scripts)
- **Git**

## Getting Started

1. **Clone the repository:**
   ```bash
   git clone https://github.com/abaxoth0/Ain.git
   cd Ain
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Run tests to ensure everything works:**
   ```bash
   go test ./...
   ```

## Development Workflow

### Running Tests

- **Basic test suite:**
  ```bash
  go test ./...
  ```

- **Tests with race detector** (recommended for concurrent code):
  ```bash
  go test -race -timeout=10m ./...
  ```

- **Test coverage:**
  ```bash
  go test -cover ./...
  ```

### Running Benchmarks

- **Bounded benchmark smoke check:**
  ```bash
  go test -run='^$' -bench=. -benchmem -benchtime=100x -count=1 ./structs
  ```

- **Repeated throughput samples:**
  ```bash
  go test -run='^$' -bench='_Throughput$' -benchmem -benchtime=100x -count=5 ./structs
  ```

Read [BENCHMARKS.md](BENCHMARKS.md) before interpreting results. The existing
suite has normalization, timing, and comparability limitations. The legacy
`scripts/bench.py` collector writes CSV files in the current working directory;
it does not perform statistical analysis.

Disruptor's `Publish` and `Consume` use `//go:norace`, so the race detector does
not instrument their buffer accesses. A passing race run is not proof of safe
concurrent delivery. Review the lifecycle and concurrency limits in
[README.md](README.md) when adding concurrent code.

### Code Quality

Before committing, ensure your code follows the project standards:

```bash
# Format all Go files
gofmt -w .

# Run static analysis
go vet ./...

# Run tests with race detector
go test -race -timeout=10m ./...
```

For workflow regression tests:

```bash
python3 -m pip install -r scripts/requirements-ci.txt
python3 -m unittest discover -s scripts -p 'test_*.py'
```

## Code Style Guidelines

- Follow Go conventions and standard formatting
- Use `gofmt -w .` before committing changes
- Add comprehensive tests for new features
- Comment complex logic, especially in concurrent code
- Use generics appropriately (Go 1.23+)
- Follow existing patterns in each package

## Project Structure

```
Ain/
├── structs/     # Data structures and concurrency helpers
├── logger/      # Flexible logging system
├── errs/        # Error handling utilities
├── common/      # Common helper functions
└── scripts/     # Benchmark, analysis, and workflow validation tools
```

### Package Overview

- **`structs`**: Disruptor, SyncQueue, stacks, WorkerPool, and metadata helpers
- **`logger`**: Multi-level logging with forwarding capabilities
- **`errs`**: Structured error types with HTTP status codes
- **`common`**: Utility functions and helpers

## Pull Request Process

1. **Fork the repository** and create a branch using git-flow naming
   (`feature/...` or `hotfix/...`). Do not edit or commit directly on `dev` or
   `main` unless explicitly authorized:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** and ensure:
   - All tests pass (`go test ./...`)
   - Tests pass with race detector (`go test -race -timeout=10m ./...`)
   - Code is formatted (`gofmt -w .`)
   - New features include tests
   - Documentation is updated if needed

3. **Run benchmarks** if your changes affect performance:
   ```bash
   go test -run='^$' -bench=. -benchmem -benchtime=100x -count=1 ./structs
   ```

4. **Update `CHANGELOG.md`** under `Unreleased`, then commit all task changes
   with a concise `[scope] message`:
   ```bash
   git commit -m "[structs] add task processing option"
   ```
   Include a regression test for bug fixes and run smoke, unit, and integration
   checks for affected modules before committing. Leave the working tree clean.

5. **Push to your fork** and create a pull request.

### Pull Request Guidelines

- **Title**: Use the `[scope] message` format (for example, `[docs] clarify queue lifecycle`)
- **Description**: Explain what you changed and why
- **Testing**: List the checks run and their results, including any failures
- **Performance**: Include benchmark results if applicable
- **Documentation**: Update README if adding new features

## Testing Guidelines

### Test Coverage

- Measure current coverage with `go test -cover ./...`; do not assume historical percentages still apply
- Test both happy paths and error cases
- Test concurrent code with race detector
- Include benchmark tests for performance-critical code

### Test Organization

- Follow existing test file patterns (`*_test.go`)
- Use table-driven tests for multiple scenarios
- Test edge cases and boundary conditions
- Mock external dependencies when appropriate

## Performance Considerations

When changing performance-sensitive code:

- **Benchmark your changes**: Use the existing benchmark suite
- **Consider memory allocations**: Measure setup and steady-state costs separately
- **Test under contention**: Simulate real-world high-load scenarios
- **Profile**: Use `pprof` for performance optimization

## Reporting Issues

### Bug Reports

Include the following in bug reports:

1. **Go version**: `go version`
2. **Operating System**:
3. **Steps to reproduce**: Minimal reproduction code
4. **Expected behavior**: What should happen
5. **Actual behavior**: What actually happens
6. **Error messages**: Full stack traces if applicable

### Feature Requests

1. **Use case**: Explain the problem you're trying to solve
2. **Proposed solution**: How you envision the feature
3. **Alternatives considered**: Other approaches you've thought of
4. **Additional context**: Any relevant information

## Questions?

If you have questions about contributing:

1. Check existing issues for similar discussions
2. Create an issue with the "question" label
3. Start your issue title with "Question:"

## License

By contributing to Ain, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Ain! Your contributions help make this library better for everyone.

# Contributing to Ain

Thank you for your interest in contributing to Ain! This guide will help you get started.

## Prerequisites

- **Go 1.23.0 or later**
- **Python 3.8+** (optional, for benchmark scripts)
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
  go test -race ./...
  ```

- **Test coverage:**
  ```bash
  go test -cover ./...
  ```

### Running Benchmarks

- **Quick benchmarks:**
  ```bash
  go test -bench=. -benchmem ./structs
  ```

- **Comprehensive benchmark suite (100 iterations):**
  ```bash
  python3 scripts/bench.py -c 100 --path ./structs
  ```

The automated benchmark script generates statistical analysis and CSV reports in the `scripts/` directory.

### Code Quality

Before committing, ensure your code follows the project standards:

```bash
# Format all Go files
gofmt -w .

# Run static analysis
go vet ./...

# Run tests with race detector
go test -race ./...
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
├── structs/     # Concurrent data structures
├── logger/      # Flexible logging system
├── errs/        # Error handling utilities
├── common/      # Common helper functions
├── cmd/         # Example/demo code
└── scripts/     # Benchmark and analysis tools
```

### Package Overview

- **`structs`**: Data structures (Disruptor, SyncQueue, WorkerPool)
- **`logger`**: Multi-level logging with forwarding capabilities
- **`errs`**: Structured error types with HTTP status codes
- **`common`**: Utility functions and helpers

## Pull Request Process

1. **Fork the repository** and create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** and ensure:
   - All tests pass (`go test ./...`)
   - Tests pass with race detector (`go test -race ./...`)
   - Code is formatted (`gofmt -w .`)
   - New features include tests
   - Documentation is updated if needed

3. **Run benchmarks** if your changes affect performance:
   ```bash
   go test -bench=. -benchmem ./structs
   ```

4. **Commit your changes** with clear, descriptive messages:
   ```bash
   git commit -m "feat: add new feature description"
   ```

5. **Push to your fork** and create a pull request.

### Pull Request Guidelines

- **Title**: Use conventional commits format (`feat:`, `fix:`, `docs:`, etc.)
- **Description**: Explain what you changed and why
- **Testing**: Mention that all tests pass
- **Performance**: Include benchmark results if applicable
- **Documentation**: Update README if adding new features

## Testing Guidelines

### Test Coverage

- Aim for high test coverage (current: logger 83.7%, structs 97.5%)
- Test both happy paths and error cases
- Test concurrent code with race detector
- Include benchmark tests for performance-critical code

### Test Organization

- Follow existing test file patterns (`*_test.go`)
- Use table-driven tests for multiple scenarios
- Test edge cases and boundary conditions
- Mock external dependencies when appropriate

## Performance Considerations

This library focuses on high-performance concurrent data structures. When contributing:

- **Benchmark your changes**: Use the existing benchmark suite
- **Consider memory allocations**: Zero-allocation paths are preferred
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

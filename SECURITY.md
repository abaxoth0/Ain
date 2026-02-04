# Security Policy

## Supported Versions

| Version | Supported          |
|---------|-------------------|
| Latest  | ✅                |
| Latest - 1 | ✅            |
| Latest - 2 | ✅            |

**Support Policy:** Security patches are provided for the latest 3 minor releases. Older versions do not receive security updates.

## Reporting Vulnerabilities

Since this is a personal utility library (not a professional-oriented library), security vulnerabilities should be reported by creating an issue in the repository.

### How to Report

1. **Create a new issue** in the GitHub repository
2. **Use the "Security" label** (if available) or "Bug"
3. **Mark as sensitive** in the issue description if GitHub provides that option

### What to Include

- **Type of vulnerability** (e.g., buffer overflow, race condition, etc.)
- **Affected versions** of Ain
- **Steps to reproduce** the vulnerability
- **Impact assessment** (what could happen if exploited)
- **Any potential mitigations** you're aware of

### Response Process

- **Initial response**: We'll acknowledge the issue within 7 days
- **Assessment**: We'll investigate and validate the vulnerability
- **Fix**: We'll develop and test a patch
- **Release**: We'll release a new version with the fix
- **Disclosure**: We'll update the issue once fixed

## Security Considerations

### Concurrent Code

This library contains high-performance concurrent data structures. Be aware of:

- **Race detector**: Some code may trigger false positives with `-race` flag due to lock-free implementations
- **Atomic operations**: The Disruptor uses atomic operations with specific memory ordering
- **Memory safety**: Zero-allocation paths are used to prevent GC pressure

### Dependencies

Current dependencies are minimal and regularly updated:

```
github.com/json-iterator/go v1.1.12
github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421
github.com/modern-go/reflect2 v1.0.2
```

### No User Data Collection

Ain is a library only and does not:
- Collect user data
- Make network calls (except for testing/benchmarking)
- Access filesystem (except for optional file logging)
- Handle sensitive information directly

## Known Security Notes

### Race Detector False Positives

The Disruptor implementation may trigger race detector warnings even though the code is thread-safe. This is documented in `structs/disruptor.go:120`:

```go
/*
    IMPORTANT
    Race detector notes: When running with -race, Go's race detector flags publisher-consumer buffer access as data races.
    These are false positives because:
    
      1. CAS on writer cursor ensures only one publisher can claim a sequence number
      2. Writer is advanced only after CAS succeeds (establishes happens-before relationship)
      3. Consumer reads after seeing updated writer cursor (synchronized through atomic operations)
      4. No mutexes used - fully lock-free implementation
    
    The race detector doesn't understand happens-before relationships established through
    atomic operations on different variables.
*/
```

### Memory Safety

- **Disruptor**: Pre-allocated ring buffer prevents dynamic allocations during operation
- **SyncQueue**: Uses proper synchronization with mutexes
- **WorkerPool**: Context-based cancellation for safe shutdown

## Best Practices for Users

When using Ain in your applications:

1. **Keep dependencies updated**: Regularly update Ain to latest version
2. **Review your usage**: Understand the concurrent patterns you're using
3. **Test thoroughly**: Test your application with race detector enabled
4. **Monitor performance**: Watch for unusual behavior in production

## Security Updates

- **Patch releases**: Security fixes are released as patch versions (e.g., v1.2.3)
- **Announcements**: Security updates are mentioned in release notes
- **Compatibility**: Security patches maintain backward compatibility

## Contact

For security-related questions or concerns:

1. **Create an issue** with the "Security" label
2. **Check existing issues** for similar reports
3. **Provide detailed information** about your security concern

## Disclaimer

Ain is provided "as is" without warranty. While we take security seriously and will address reported vulnerabilities, users should:

- Review the code before using in security-sensitive applications
- Test thoroughly in their specific use cases
- Consider their threat model when using concurrent data structures

---

Thank you for helping keep Ain secure!
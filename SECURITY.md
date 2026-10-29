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
3. **Keep public reports free of secrets**: repository issues are public; avoid including secrets or private data

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

### Concurrency and lifecycle limits

Atomic cursors alone do not establish that the Disruptor buffer is safe. In
`Publish`, the writer cursor advances before the slot is assigned. A consumer
can therefore read an unwritten slot, including when multiple publishers finish
out of order. There is no coordination between multiple consumers.

`Publish` and `Consume` are marked `//go:norace`. Their buffer accesses are not
instrumented, so a passing race-detector run cannot validate these paths. The
previous claim that warnings were harmless false positives was unsupported.
Treat race reports as findings to investigate, not warnings to dismiss.

Other implementation limits relevant to callers:

- Disruptor `Close` is not idempotent and does not join the consumer.
- WorkerPool cancellation does not wake idle queue waiters or guarantee that
  queued tasks and drain work have finished. It cannot interrupt a running task.
- FileLogger inherits the Disruptor and WorkerPool limits. Serialization failures
  discard entries, write errors are ignored, and shutdown does not establish
  durable delivery. Fatal logging exits without waiting for file output.
- Logger configuration, forwarding changes, shared metadata, and the global
  well-known metadata property registry require caller coordination.
- Retry can continue its operation after returning an attempt-limit error and
  leave a goroutine blocked sending a result. It has no context cancellation.

See [README.md](README.md) for API-specific usage and lifecycle details. These
limitations require implementation fixes; documentation and passing tests do not
remove them.

### Dependencies and data handling

The library has no third-party Go dependencies. Optional development and CI tools
have their own dependencies.

Ain does not provide telemetry or built-in network clients. FileLogger writes log
entries to the configured filesystem path; console loggers write to stdout or
stderr. Log contents, error strings, and metadata are caller-supplied and may
contain sensitive data. Choose file permissions and log destinations accordingly.
Callbacks supplied to Retry and WorkerPool execute application code and may have
network, filesystem, or other side effects.

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

# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | Yes       |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in crate, please report it responsibly.

### How to Report

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email: **security@agentcrate.ai**

Include as much of the following information as possible:

- Type of vulnerability (e.g., injection, privilege escalation, supply chain)
- Full path to the affected source file(s)
- Step-by-step instructions to reproduce
- Impact assessment
- Suggested fix, if any

### Response Timeline

- **Acknowledgment**: Within 48 hours
- **Assessment**: Within 5 business days
- **Fix timeline**: Depends on severity
  - Critical: Patch within 72 hours
  - High: Patch within 1 week
  - Medium/Low: Next scheduled release

### Scope

The following are in scope:

- The `crate` CLI binary
- Agentfile parsing and validation
- OCI artifact building and signing
- Local Docker runtime orchestration
- Configuration and secrets handling
- The `pkg/agentfile` public Go library

The following are out of scope for this repository:

- CrateHub (report to CrateHub's security process)
- Third-party MCP servers
- Docker Engine vulnerabilities
- LLM provider APIs

## Security Design Principles

- **Zero secrets in artifacts**: Agentfiles must never contain plaintext secrets
- **Signed artifacts**: All OCI artifacts are signed with Cosign/Sigstore
- **Minimal privileges**: Docker containers run with least-privilege policies
- **Input validation**: All Agentfile inputs are validated against the JSON Schema
- **Dependency scanning**: Dependencies are regularly audited for vulnerabilities

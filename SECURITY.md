# Security Policy

## Supported versions

requestCore ships two independent modules. Security fixes are applied to the following lines:

| Module | Import path | Status | Receives security fixes |
|---|---|---|---|
| Root (v1) | `github.com/hmmftg/requestCore` | Stable | Yes — latest `v1.x` release |
| v2 | `github.com/hmmftg/requestCore/v2` | Alpha prerelease | Best-effort — update to the latest `v2/v2.0.0-alpha.N` tag |

Unsupported, unmaintained, or EOL lines do not receive security fixes.

## Reporting a vulnerability

**Do not open a public GitHub issue for a security vulnerability.**

Please report security issues privately using one of these channels:

1. **GitHub Security Advisories** (preferred): go to the
   [Security tab](https://github.com/hmmftg/requestCore/security/advisories/new)
   and use "Report a vulnerability". This keeps the report private to the
   maintainers until a fix is coordinated.
2. **Email**: if you cannot use GitHub Advisories, contact the maintainer via
   the email listed on the GitHub profile.

Please include:

- A description of the vulnerability and its impact
- Steps to reproduce, or a proof-of-concept
- Affected versions/tags
- Any suggested mitigation or fix

You will receive an acknowledgment within a reasonable timeframe. Please do not
disclose the issue publicly until a fix has been released.

## Disclosure

Once a fix is ready, a new release is cut and a GitHub Security Advisory is
published with the details and credits. Public disclosure happens alongside
the patched release.

# Runtime Security

## Build Baseline

Build tools and installed application runtimes are separate. Use Go 1.27.3 and
patched Node.js 22.23.4+, 24.22.1+ or 26.12.1+ on those supported release lines.
The npm engine guards and build scripts reject older build runtimes. Rebuild
Windows amd64 and macOS amd64/arm64 gateways and inspect embedded Go metadata;
upgrading the compiler alone does not replace a deployed binary.

Desktop packages use Electron 44.9.3, which embeds Node 24.22.0. Its blocked-key
certificate-chain patch and Chromium backports do not establish that every fix
in Node 24.22.1 is included. Keep the upstream runtime gap under review; do not
claim that upgrading system Node or an application dependency fixes Electron.
Never bypass TLS validation or replace Electron's embedded runtime manually.

- [Node security release](https://nodejs.org/en/blog/vulnerability/october-2026-security-releases)
- [Go release history](https://go.dev/doc/devel/release)
- [Electron 44.9.3](https://github.com/electron/electron/releases/tag/v44.9.3)
- [Electron blocked-key patch](https://github.com/electron/electron/blob/v44.9.3/patches/node/tls_reject_chains_containing_blocked_public_keys.patch)

## Deployment Boundaries

Browser, OS, CA bundle, reverse proxy and gateway updates must each be checked on
the machine actually running them. A local build is not evidence of server patch
state. If using Caddy, inspect that binary's embedded Go version separately.
Source pulls and package downloads do not install system updates or replace
running processes. Preserve authorization, origin/host checks and loopback-only
listeners when deploying; firewall and trust-store changes require authorization.

Check complete package bytes, not just an executable version string. Update
transports accept only the fixed repository paths, bounded repository-specific
GitHub CDN redirects, declared sizes and SHA-256 values. Android also verifies
package identity and signer. No credentials or login cookies accompany updates.
Source resets do not authorize bypassing downgrade or signature protections.

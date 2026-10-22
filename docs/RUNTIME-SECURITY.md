# Runtime Security Checks

## Build57 (2026-10-22)

The build uses Go 1.27.3 and Electron 44.9.3. The development computer's Node.js
22.x installation was separately upgraded to 22.23.4, with the official SHA-256
and OpenJS Authenticode signer checked. Upgrading system Node does not replace
the Node or Chromium embedded in a desktop installer, nor any running process.

Go 1.27.3 and 1.26.10 include the announced crypto/x509 security fixes. All
Windows amd64 and macOS amd64/arm64 gateway artifacts must be rebuilt, and their
embedded Go build metadata checked; updating a compiler alone is insufficient.

Electron 44.9.3 is an official stable release on the existing major line. Its
source contains `patches/node/tls_reject_chains_containing_blocked_public_keys.patch`,
which rejects certificate chains containing the blocked public keys, in
addition to Chromium security backports. It still reports Node 24.22.0.
**This is not a claim that it incorporates every fix in Node 24.22.1.** The
remaining upstream Node security-release delta must be rechecked when Electron
publishes its corresponding embedded-runtime update. Do not downgrade TLS
validation, add certificates, or replace Electron's embedded runtime manually.

Official evidence:

- [Node October security release](https://nodejs.org/en/blog/vulnerability/october-2026-security-releases)
- [Go release history](https://go.dev/doc/devel/release)
- [Electron 44.9.3](https://github.com/electron/electron/releases/tag/v44.9.3)
- [Electron blocked-key verification patch](https://github.com/electron/electron/blob/v44.9.3/patches/node/tls_reject_chains_containing_blocked_public_keys.patch)

## Machine And Deployment Audit Boundary

Local browser and OS updates are independent from application builds. During
this audit Chrome 155.0.8059.80 was behind the official stable 156.0.8078.48;
Edge 155.0.4283.62 matched its official stable feed. Windows Update reported
KB5127648 pending. Root-certificate automatic synchronization was observed on
2026-10-22 with no disabling policy observed. These observations do not prove
that every installed certificate, browser process or OS component is current.
No browser, OS or trust-store update is installed as part of this read-only audit.

The connected Web deployment host was checked separately: Nginx, not Caddy,
was active; no Caddy executable, active unit, process or running container was
found. Its CA bundle, OpenSSL and Nginx matched that host's configured package
index candidates, whose security/update indexes were dated 2026-10-21. No fresh
package-index refresh, server upgrade or proxy/firewall change was performed.
This does not establish the state of other hosts, stopped containers or future
updates. If Caddy is used elsewhere, inspect that actual binary's build info
and Go version; local Go or gateway metadata is not evidence about Caddy.

Raw machine identifiers, service inventories, update logs and configuration
stay in ignored local evidence, not the public repository.

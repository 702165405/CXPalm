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
Other system packages had available upgrades; matching CA/OpenSSL/Nginx
candidates does not mean the entire deployment host is fully patched.
This does not establish the state of other hosts, stopped containers or future
updates. If Caddy is used elsewhere, inspect that actual binary's build info
and Go version; local Go or gateway metadata is not evidence about Caddy.

Raw machine identifiers, service inventories, update logs and configuration
stay in ignored local evidence, not the public repository.

## Build Guards And Deployment (2026-10-22)

The build57 machine and server observations above describe a separate Windows
build/deployment audit, not every checkout or Mac installation. Pulling source
does not update installed applications, system Node, existing node_modules or
running processes. A locally installed build55 remains old until it is replaced.

Both npm projects now require Node 22.23.4+, 24.22.1+ or 26.12.1+ on those
supported release lines, with strict engine checks before dependency install.
PowerShell build scripts and npm packaging/vendor entry points fail early on
older runtimes. `.node-version` records the default build version; `go.mod`
already requires Go 1.27.3. These guards concern build tools, not Electron's
embedded Node, and are not a substitute for checking packaged build metadata.

At this recheck the official Electron stable feed still reports Node 24.22.0
for 44.9.3 and 45.0.0. The 44.9.3 Node patch list includes the blocked-public-key
patch, but does not establish that CVE-2026-111497 and CVE-2026-105481 are fixed.
Keep these embedded-runtime items open pending upstream confirmation. Do not
claim an Electron major upgrade, an Accept-Encoding header or an application
source update alone fixes them.

For deployment, rebuild with the patched tools and replace the complete desktop
bundle and bundled gateway, then restart with user approval and verify actual
runtime versions. Updating only Web assets, app.asar or the compiler leaves old
native runtime files intact. Preserve private configuration and state; publish
changed installer bytes under a new build/version, never overwrite an existing
release artifact or change a manifest to hashes of unpublished local output.

The subsequent Mac Intel installation check replaces the complete build55
application with the verified build57 bundle and confirms Go 1.27.3, Electron
44.9.3 and embedded Node 24.22.0 in the installed files. Local health and
read-only official IPC checks pass, and protected configuration/authorization
state is preserved. This deployment does not resolve the unconfirmed upstream
Node fixes or update system Node. See VERIFICATION.md for the acceptance scope.

## System Node Follow-Up (2026-10-22)

The Mac system executable at `/usr/local/bin/node` is subsequently updated
from 22.13.1 to the official 22.23.4 binary. The official archive SHA-256,
extracted binary hash and macOS code signature are checked before replacement.
The old executable is backed up and replacement is atomic. npm and existing
global packages are not replaced. The system command now reports 22.23.4 and
passes the build guard and all 70 Node regression tests.

Nine Node processes were already running before replacement and still existed
at verification. Their ancestry includes the current Codex/ChatGPT client;
they are not terminated automatically. Restart those owning clients/processes
before considering their in-memory runtimes updated. Replacing a file does
not patch an already running process, and this is not a host-wide runtime audit.

The standalone Node HTTPS update fallback now uses `agent: false` and
`Connection: close` to avoid pooled response confusion. Its existing identity
encoding request and compressed-response rejection remain enforced; it does
not invoke a Brotli decoder. This source-level defense is not a native Node
patch. The installed Electron build uses its existing Chromium network path;
this fallback change has not been repackaged into a new application release.

A fresh official Electron feed check still finds no stable release reporting
Node 24.22.1 or later on the 24.x line. No complete upstream Node security-fix
backport is confirmed for the installed 44.9.3. The embedded-runtime items
remain open; the system update and one-shot transport do not prove them fixed.

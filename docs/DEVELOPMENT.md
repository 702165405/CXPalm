# Development

## Layout

| Path | Responsibility |
| --- | --- |
| `cmd/cxpalm` | Agent CLI and local stdio host entry point |
| `internal/desktop` | Official-owner IPC, state projection, history and controls |
| `internal/server` | HTTPS/WSS API, authorization, uploads and notification feed |
| `internal/store` | Local SQLite authorizations, receipts, queues and preferences |
| `internal/attachments` | Bounded, rooted upload storage and native input conversion |
| `web` | Browser client, safe Markdown and protocol/UI regressions |
| `android` | Native Kotlin/Compose client |
| `desktop` | Local Electron settings, lifecycle and updater |
| `deploy` | Reverse-proxy and release-page templates; no automatic publication |

## Build and Test

Use Go 1.27.3+, patched Node.js (22.23.4+, 24.22.1+ or 26.12.1+ on those
release lines), JDK 17+ and an Android SDK for API 35. Android uses
Gradle 8.11.1. Local tool caches may be placed in ignored `.tools/`.
The root `.node-version` selects the 22.x baseline for compatible version
managers; it does not install Node or change PATH by itself. Both npm projects
reject unsupported build runtimes with `engine-strict`. The build scripts and
npm packaging/vendor entry points also check the runtime before proceeding.

```powershell
node scripts/check-runtime.cjs
node --test scripts/check-runtime.test.cjs
go test ./...
go vet ./...
node --test web/protocol.test.mjs
node --test desktop/test/*.test.cjs
node --test scripts/check-repository.test.cjs
node scripts/check-repository.cjs
gradle -p android :app:testDebugUnitTest :app:lintDebug :app:assembleDebug
```

`scripts/build.ps1 -Android` runs Go checks and creates local Agent/APK output.
`scripts/build-desktop.ps1 -InstallDependencies -Installer` packages Windows
and stages a checksum manifest locally, without uploading it. It also builds
the Agent needed by desktop process tests. Use `npm ci` in `desktop` before
running Electron locally.

`node web/browser.test.cjs` requires Playwright to be installed separately.
Set `CXPALM_TEST_BROWSER` to an existing Chromium-based browser executable if
the Playwright browser cache is absent. The suite uses isolated network fixtures,
not the real gateway. Screenshots and diagnostics remain in ignored `.local/`.

For Android debug signing, `scripts/build.ps1 -Android` creates a project-local
development key if absent. Direct Gradle builds can set `CXPALM_DEBUG_KEYSTORE`
to an existing key. Never commit it. Release signing requires your own separate
configuration; a debug build is not a production-signed release.

Also cross-build `./cmd/cxpalm` with `CGO_ENABLED=0` for Windows amd64 and macOS
amd64/arm64. macOS compilation is not macOS runtime verification. Native/live
tests are opt-in and must not send real model requests without authorization.

## Safety and Release

- Preserve official main-chat execution ownership. Never resume it elsewhere.
- Unknown writes are not retried. Query or inspect their original receipts.
- Keep native patch bases immutable; queue and confirmed-steer presentation
  overlays are not execution state.
- Defaults are quiet: notifications require explicit per-chat opt-in.
- Attachments are local runtime data, not repository assets or public downloads.
- The initial shared version is `0.0.1`, Android versionCode 1 and desktop
  release build 1. Increase the shared version and both build numbers together;
  never reuse an existing version/build for different package bytes.
- Desktop manifests use `releaseLine: fresh-001`. Native download redirects are
  restricted to this repository's immutable asset identity. Do not bypass
  downgrade, certificate, package or checksum checks for an older installation.
- Never upload secrets, databases, logs, backups, SDKs, `node_modules` or build
  output. Do not publish or change server/firewall configuration without approval.

See [protocol](PROTOCOL.md), [notifications](NOTIFICATIONS.md),
[attachments](ATTACHMENTS.md) and [verification](../VERIFICATION.md).

Windows installer names include the build, for example
`CXPalm-Setup-0.0.1-build1-x64.exe`. Android uses
`CXPalm-0.0.1-build1-debug.apk`. Publish and verify immutable assets before
promoting the corresponding manifests in `updates/`.

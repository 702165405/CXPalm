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

Use Go 1.24+, Node.js, JDK 17+ and an Android SDK for API 35. Android uses
Gradle 8.11.1. Local tool caches may be placed in ignored `.tools/`.

```powershell
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
- The source version line restarted at `0.0.1`; the current source is `0.0.8`,
  Android installation code 44; desktop release build 44. Build numbers remain monotonic and retain the
  original signing identity for in-place upgrades.
- Desktop manifests use `releaseLine: initial-001`. Previous feeds cannot
  silently reinstall an older feature set after the semantic version reset.
  Existing desktops with higher semantic versions need an explicit manual
  transition; do not bypass downgrade checks or replace immutable old packages.
- Never upload secrets, databases, logs, backups, SDKs, `node_modules` or build
  output. Do not publish or change server/firewall configuration without approval.

See [protocol](PROTOCOL.md), [notifications](NOTIFICATIONS.md),
[attachments](ATTACHMENTS.md) and [verification](../VERIFICATION.md).

Windows immutable installer names include the release build, for example
`CXPalm-Setup-0.0.1-build36-x64.exe`. The strict manifest validator checks that
exact build-qualified URL under the existing trusted HTTPS origin. Old
`CXPalm-Setup-0.0.1-x64.exe` bytes are retained unchanged.

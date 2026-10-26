# Verification

Public verification records exclude real conversation content, device identifiers,
private deployment paths, credentials and raw logs. Tests use synthetic data unless
explicitly described as deployment connectivity or public-byte checks.

## 0.0.2 / build2

- Web and Android remember the last confirmed newly created chat's notification,
  model, reasoning effort and permission mode, separately per computer/client.
  Only connected desktop state updates defaults; approval grants, messages,
  instructions and execution ownership are not copied. Full access requires
  fresh confirmation and unsupported model/effort selections are rejected.
- Empty-thread startup overrides were not durable in an isolated official CLI
  check. The implementation instead applies settings once through the original
  desktop owner's conditional settings API, after connection and while the new
  chat remains empty. Unknown/rejected results never replay the settings write
  or creation. Synthetic original-owner, rejection, disconnect and nonempty-chat
  tests pass; this is not a real conversation or physical-device acceptance.
- Full Go tests and vet pass. Windows amd64 and macOS amd64/arm64 artifacts are
  rebuilt with Go 1.27.3, trimmed paths and no Git VCS build metadata.
- All 78 Node checks and isolated Chromium UI checks pass, including first-use
  defaults, parameter reuse, explicit full-access confirmation and opting out.
- Android reports 84 unit tests: 83 pass and one publication-only updater test
  is skipped before release. Lint and normal assembly pass. The APK reports
  version 0.0.2/code 2, retains its package/signing identity and excludes rendering
  probes and checked private runtime files. No phone installation is performed.
- All 12 packaged desktop source/manifest files pass the selected privacy scan;
  executable source files match tested inputs and generated package identity
  matches source metadata. Published-byte and deployment checks follow release.

## 0.0.1 / build1

The release starts a new repository/version line. Android, Windows desktop and
Web/gateway use version 0.0.1; Android versionCode and desktop release build are 1.
Native updates are bound to this repository identity and desktop release line
`fresh-001`. Hash, signature, package identity and downgrade checks remain enabled.

Existing installations from another version line are not silently downgraded.
Android installation over a higher versionCode is refused by the platform; users
must handle any manual transition and local data preservation themselves. No phone
is uninstalled, no account data is migrated and no model request is simulated as
real acceptance.

## Build And Package Checks

- Full Go tests and vet pass. Windows amd64 and macOS amd64/arm64 gateway
  archives are rebuilt with Go 1.27.3 and without previous Git VCS metadata.
- All 77 Node checks pass, including shared-version/build consistency,
  strict update transport and credential/private-file detection regressions.
- Isolated Chromium and Electron UI checks pass. These use synthetic execution
  data and do not claim a real model run or physical-device acceptance.
- Android reports 83 unit tests: 82 pass and one opt-in public-update check is
  skipped before publication. Lint and normal APK assembly pass. The APK reports
  version 0.0.1/code 1, retains its signing identity, includes appearance and
  computer-switching features and excludes rendering-probe classes.
- Packaged desktop sources match tested inputs and pass the selected privacy
  scan. Current source and new Git history are checked separately before upload.

## Published Release And Deployment

- The new repository has a parentless initial root, no inherited source history,
  only the main branch and one v0.0.1-build1 release/tag. Source author metadata
  uses the maintainer's GitHub noreply address. Private backups are not uploaded.
- All ten published assets are downloaded and match staged sizes and SHA-256
  values. Both main-branch update manifests report 0.0.1/build1. GitHub asset
  redirects are verified against the new repository's numeric resource identity.
- Actual Electron check/download/pre-install verification passes without running
  an installer. A higher installed version is not offered a downgrade, and the
  previous release line is rejected. Android's public download/hash test passes
  with one test and no skips; no phone install or uninstall is performed.
- The Web entry and three versioned assets match their public hashes. All 76
  installed Windows runtime files match the new package. The gateway reports
  0.0.1, official desktop IPC connects, the desktop owns its Agent and established
  tunnel, reverse health succeeds and unauthenticated public API returns 401.
- Configuration, authorizations, IP policy, preferences, shortcuts and startup
  registration remain unchanged. Local diagnostics are active. Windows/macOS
  cross-compilation is not physical Android or macOS runtime acceptance.

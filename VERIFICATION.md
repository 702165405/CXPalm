# Verification

Public verification records exclude real conversation content, device identifiers,
private deployment paths, credentials and raw logs. Tests use synthetic data unless
explicitly described as deployment connectivity or public-byte checks.

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

Public-byte, updater and deployment results are recorded after publication.
Windows/macOS cross-compilation is not physical Android or macOS runtime acceptance.

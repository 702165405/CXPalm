# Verification Status

This public record intentionally excludes real conversation titles, IDs,
content, device identifiers, account counts, private paths and deployment logs.
Private test evidence is not part of the source repository.

## Conversation-Scoped Receipts (0.0.12 / build48)

- Go tests/vet and Windows amd64 / macOS amd64+arm64 cross-builds pass.
- All 62 Node tests and isolated Chromium UI fixtures pass. Regressions cover
  unknown archives, independent conversation mutations, cross-tab journals,
  legacy Web recovery, contextual warnings and operation-ID-specific inspection.
- Android: 78 tests, 77 passed and one opt-in public-network test skipped;
  Lint and normal APK assembly pass. The existing package ID and signer remain
  unchanged. Rendering probes are absent from the normal APK.
- Old Android journals containing only an ID remain protected until inspected;
  no conversation is guessed and no uncertain request is replayed.
- No Android device was attached for this fix. Unit/build checks do not claim
  physical-device acceptance. No live chat mutation is part of these tests.
- Web and Android are updated; desktop/gateway remain 0.0.10/build46.

## Android Question Navigation (0.0.11 / build47)

- A private local snapshot replay on Android 35 reproduced a main-thread
  `IllegalArgumentException: Index should be non-negative (-1)` in question-list
  automatic scrolling. The derived index could change during a frame wait,
  before its effect was cancelled. This is a reproduced crash, not an inference
  from a successful build.
- The fix retains a validated local target across that suspension and checks
  the list bounds after layout. Slider releases also reject stale indices.
- Normal APKs exclude the opt-in rendering activity and instrumentation runner.
  Replay inputs stay local and untracked; tests do not execute real chat actions.
- Fresh Go tests/vet and Windows amd64 / macOS amd64+arm64 cross-builds pass;
  all 60 Node tests pass. Android has 76 tests: 75 pass and one public-network
  opt-in test is skipped in the default run. Lint and APK assembly pass.
- Package identity and the existing Android signer are retained. Bytecode
  inspection confirms all rendering-probe classes are absent from the normal APK.
- Android 35 emulator replay and repeated openings of previously crashing
  inputs pass after the fix. The normal, probe-free APK also installs and stays
  alive through a cold-start check; its manifest has no probe instrumentation.
- All three Android release assets pass fresh public byte/hash checks. The
  actual Android updater's opt-in JVM feed/download test passes for build47.
  The desktop feed is unchanged and the existing gateway remains running.
- This is an Android-only patch. Web and Windows/gateway remain 0.0.10/build46;
  no gateway restart, authorization change or data migration is required.
- Physical-device acceptance remains separate from emulator checks.

## Pending Send Recovery (0.0.10 / build46)

- Fresh Go tests and vet pass; Windows amd64 and macOS amd64/arm64 builds pass.
- All 60 Node tests pass. Isolated Chromium and Electron UI fixtures pass.
- Android: 75 unit tests, 74 passed and one opt-in public download check skipped;
  Lint and APK assembly pass. Package ID and existing signer are retained.
- Regression checks cover missing-receipt recovery, permanent delayed-request
  fences, device isolation, dispatch/lookup races, draft/attachment preservation,
  mismatched receipts and continued blocking of uncertain operations.
- The local gateway and original-owner snapshots were available during
  diagnosis. A stranded client pending lock is reproducible from the code path;
  this is not proof of the reporting phone's persisted state.
- No USB phone was available. Android installation, startup and real-message
  sending on the reporting device remain unverified; no real message was sent
  by the tests. No server/firewall or IP-policy changes are part of this fix.
- All six GitHub release assets pass fresh public size/SHA-256 checks. The
  actual Electron updater detects and downloads the upgrade and rechecks it
  before installation; the installer was not executed by that test. Android's
  opt-in JVM updater check also passes against the public feed and APK.
- Local desktop/gateway resources are deployed, health reports 0.0.10 and a
  passive original-owner snapshot succeeds. Configuration, preferences,
  authorizations, IP-policy settings and startup configuration remain unchanged.
- Web uses two new versioned modules; public entry/module bytes match staged
  hashes. Previous modules remain unchanged.

## Source Baseline

- Web / Android source: 0.0.12 / versionCode 48; desktop/gateway: 0.0.10.
- Desktop package: releaseBuild 46.
- Previous local records report passing Go tests/vet, Windows amd64 and macOS
  amd64/arm64 cross-builds, Web and desktop tests, Android unit tests, Lint and
  APK assembly for this baseline. These are historical reports, not a fresh
  run or a guarantee that another environment will pass.
- Browser/Electron fixture tests do not constitute real-account or model tests.
- A cross-build is not runtime acceptance on the target operating system.

## Repository Preparation Checks (2026-10-20)

Fresh checks during privacy/documentation preparation:

| Check | Result |
| --- | --- |
| Go default tests and vet | Pass; live opt-in variables disabled |
| Windows amd64 build | Pass |
| macOS amd64 and arm64 cross-builds | Pass; not executed on macOS |
| Desktop and Web Node tests | 52 pass, including isolated stdio host tests |
| Repository checker regression tests | 5 pass |
| Working-tree candidate privacy scan | 248 files; no selected-pattern findings |
| Markdown local links / npm resolved URLs | No broken links or credential-bearing dependency URLs found |
| Android / physical device | Not run in this preparation task; startup crash remains unresolved |

No public repository, source commit, push, binary release, installation or
server change is performed as part of this preparation. Local private records
are preserved in ignored storage. Raw test logs are not publication artifacts.

## GitHub Update Migration Checks (0.0.8 / build44)

- Go default tests/vet and Windows amd64/macOS amd64+arm64 cross-builds pass.
- Node regression suite: 58 pass. Android: 74 unit tests, 73 pass and one
  opt-in network test skipped; Lint and APK assembly pass.
- Windows installer builds; hidden Electron check/download/ready fixture passes
  without running an installer. APK package/version and the original signing
  certificate are verified. This does not establish physical-device acceptance.
- Regression cases reject other repositories, other CDN repository IDs,
  non-HTTPS destinations, credential-bearing redirects, redirect loops and
  manifest redirects. Size/hash, signer and downgrade checks remain covered.
- Public release download and legacy-feed migration acceptance are recorded
  separately after publication; build output alone is not publication evidence.

## Native Redirect Correction (0.0.9 / build45)

Real Electron download acceptance found that its Fetch manual mode cancels
redirects instead of returning a readable 3xx response. Build44 is superseded,
not replaced in place. The corrected transport uses credential-free native
HEAD probes, checks each hop with the shared repository/CDN allowlist, then
fetches the body without implicit redirects. A regression specifically covers
this Electron path. A real GitHub download/hash/pre-install check passes with
the corrected source, without executing the installer.

Final build45 Go tests/vet and all three cross-builds pass. Node reports 59
passing tests. Android unit/Lint/APK build and original-certificate checks pass;
hidden Electron update UI check/download/ready/install-refusal passes with an
observed exit code. Public build45 and legacy bridge results follow after their
actual network verification. No Android crash fix is claimed by this migration.

## Published Update Acceptance (0.0.9 / build45)

- The public stable GitHub Release contains the new Windows installer, APK,
  both manifests and checksum files. All six downloads match exact local sizes
  and SHA-256 values. Their redirects use the allowed CDN repository path.
- Both stable raw-GitHub manifest endpoints match committed bytes. The actual
  Electron updater recognizes current/upgrade states, downloads the installer,
  verifies it and rechecks immediately before installation. No installer is run
  by this test. The actual Android UpdateTransport opt-in JVM test downloads and
  hashes the public APK successfully; this is not physical-device installation.
- The old source offers one compatible migration release with the identical
  GitHub package bytes. The original desktop updater validates its manifest,
  fully downloads it and passes pre-install recheck. Legacy Android manifest
  fields and complete APK bytes match the new repository package and original
  signing identity. After upgrading, clients use only the repository feeds.
- Build44 is explicitly marked superseded/prerelease; its files are retained
  unchanged. Build45 is the current stable release; no immutable file is replaced.
- Installed local resources match build45; health reports 0.0.9 and official
  desktop IPC connects. Configuration, authorizations, preferences, shortcuts
  and startup registration remain unchanged. The desktop window owns the gateway.
- The Web public entry matches current source. Unaffected historical public
  assets and all captured server configuration hashes remain unchanged; no
  server/firewall/TLS/proxy configuration change or service installation occurs.

Raw device/account/deployment evidence remains in ignored local storage.
GitHub availability depends on the client's network. This update-source
migration does not establish resolution of the separately reported Android
startup crash or OEM background-notification acceptance.

## Known Issues And Acceptance Gaps

- An Android startup crash has been reported and reproduced by the user.
  Its cause and a fix have not been confirmed in this repository-preparation
  task. Do not describe Android startup as accepted or the crash as resolved.
- Physical-device image insertion/zoom, OEM background behavior and notification
  delivery require explicit device testing. A successful APK build is not proof.
- Official desktop internal protocols can change and require adaptation.
- Windows packages are unsigned; Android development signing is not production
  signing. No signing key or installation authorization is included here.

## Reproducing Checks

See [development](docs/DEVELOPMENT.md) for dependencies and commands. The standard
Go, Node and Android unit tests use fixtures. Live tests require explicit opt-in
variables and can access private local data or mutate test chats; read the
individual test before enabling it. Never upload their raw output.

Before sharing a source snapshot run `node scripts/check-repository.cjs` and
review the exact candidate file list. The checker detects selected patterns;
it cannot prove the absence of all secrets or establish asset provenance.

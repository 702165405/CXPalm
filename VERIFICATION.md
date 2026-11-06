# Verification

Public verification records exclude real conversation content, device identifiers,
private deployment paths, credentials and raw logs. Tests use synthetic data unless
explicitly described as deployment connectivity or public-byte checks.

## 0.0.8 / build8

- Web and Android expose document cards for assistant file citations and local
  Markdown links, including existing history. Web saves with the browser download
  manager; Android uses the system document picker. HTML and office documents
  remain downloads, with no script execution or embedded office editor.
- Tests cover opaque message-bound IDs, citation/link deduplication, private and
  remote path rejection, outside-root and oversized reads, missing files,
  attachment response headers, Origin/authentication checks and revocation.
  Rooted opens retain traversal protection. Symlink fixtures run only when the
  host grants symlink creation; this Windows account does not grant that privilege.
- Two existing locally generated documents (XLSX and HTML) are
  located through read-only original history and downloaded through an isolated
  local HTTP server with a temporary authentication store. Both SHA-256 values
  match the original local files. No chat resumes, model calls, production device
  authorization changes or edits to those files occur.
- Chromium checks pass under the existing strict CSP, including desktop/mobile
  file cards, Unicode filenames, exact saved bytes, HTML downloads, missing-file
  errors and dropped late results after message removal. Clicking message
  controls no longer arms automatic history loading. Screenshots were reviewed.
- Full Go tests and vet pass; Windows amd64 and macOS amd64/arm64 cross-builds
  pass. All 99 Node checks pass. Android reports 92 tests: 91 pass and one
  publication-only updater check skips. Lint has zero errors and ten warnings.
  APK identity is 0.0.8/code8 and retains the existing development signer.
  Physical Android saving and macOS runtime acceptance have not been performed.
- All 12 desktop source/UI files and extra resources match their tested inputs;
  selected privacy checks report no findings. All 76 installed runtime files
  match the package. After restart the gateway reports 0.0.8 and completes the
  official desktop IPC handshake. Configuration, authorizations, account
  settings, preferences, shortcuts and login startup are unchanged.
- The initial Web entry and three immutable build8 resources are deployed and fresh
  HTTPS bytes match staged sizes/hashes. The public file endpoint returns 401
  without authorization. Native packages/manifests are staged locally; both
  public native feeds reported 0.0.3/build3 at this initial deployment.
  Native publication is recorded separately after public-byte verification.
  No phone installation or server/firewall change occurred.
- After explicit publication authorization, release `v0.0.8-build8` and its
  corresponding source commit are public. Both fixed native feeds now return
  0.0.8/build8 and match the staged manifests exactly. All ten assets were fetched
  through their public release URLs and allowed repository CDN redirects; sizes
  and SHA-256 values match the local artifacts. The downloaded APK retains its
  original package identity, code8 and development signer. Gateway archives
  contain only the tested binary, build metadata and README.
- The previously skipped Android publication check passes separately using the
  production update transport and fixed public feed, without account credentials.
  An isolated Electron desktop updater verifies discovery from an older version,
  the real installer download and the installation preflight hash. No installer
  is executed and no phone installation or physical-device acceptance is claimed.

## 0.0.7 / build7

- Guidance captures its insertion anchor before dispatch, persists it with the
  accepted receipt and retains a stable presentation identity on the native echo.
  Synthetic regressions cover later output, repeated identical prompts, multiple
  guidance receipts, delayed/out-of-order echoes, idempotent projection and
  unchanged native patch bases. Legacy receipts without anchors are preserved;
  their original positions cannot be reconstructed.
- Guidance requires the client's displayed task turn and rechecks it immediately
  before original-owner dispatch. Missing/stale bindings are rejected, not
  replayed. Visible paginated turn order takes priority over unordered entities.
  Android actions retain their captured computer/chat, and late Web receipts
  retain their originating chat. Guidance requires updated build7 clients/gateway.
- Question indexes identify individual user items, including multiple guidance
  messages within one turn. Web and Android use this identity for navigation and
  refresh indexes for new same-turn user messages. Chromium desktop and 390px
  mobile fixtures verify exact second-guidance positioning, active markers and
  no accidental send; screenshots were visually reviewed.
- Full Go tests and vet pass. Windows amd64 and macOS amd64/arm64 cross-builds
  pass. All 98 Node checks pass. Android reports 91 tests: 90 pass and one
  publication-only updater check skips; lint has zero errors and ten warnings.
  APK identity is 0.0.7/code7, retaining the existing development signer.
- All 12 packaged desktop source/UI files and extra resources match tested
  inputs; selected privacy checks report no findings. All 76 installed runtime
  files match the package. The restarted gateway reports 0.0.7 and completes the
  official desktop IPC handshake. Configuration, device authorizations, account
  settings, preferences, shortcuts and login startup remain unchanged.
- The Web entry and three immutable build7 resources are deployed; fresh HTTPS
  downloads match staged sizes and SHA-256 values. The public API still requires
  authorization. Android/desktop packages and manifests are staged locally; both
  public native feeds still report 0.0.3/build3. No source commit/push, native feed
  promotion, phone installation or service/firewall configuration change occurred.
  No real guidance was sent to a private chat, and no physical Android or macOS
  runtime acceptance is claimed.

## 0.0.6 / build6

- Web and Android add opt-in input suggestions, disabled by default with a Codex
  quota notice. Tab/tap fills a draft; Escape/dismiss hides the current suggestion.
  A suggestion never sends a message. Existing drafts, caret selections, IME
  composition, stale reply identities and offline state gate acceptance.
- Full Go tests and vet pass. Windows amd64 and macOS amd64/arm64 cross-builds
  pass. All 97 Node checks pass. Isolated Chromium covers desktop/mobile
  suggestions, prefix completion, IME and Shift-Tab preservation, caret selection,
  Escape dismissal, late results and acceptance without sending. Screenshots
  were visually reviewed.
- The installed official CLI and the CLI selected by the gateway both pass a
  localhost synthetic-provider integration. Each prediction uses a new ephemeral
  identity with a verified read-only/never-approve policy, no persistent rollout
  and no main-chat resume. The test checks that executable tools are absent,
  configured MCP commands do not run and an interactive request aborts without
  another model request. No external model or real chat is used in these tests.
- Suggestions contain only bounded recent user/assistant text. Execution
  features, plugins, MCP, project instructions and environment access are
  disabled. Interactive requests are never answered. Selection changes,
  authorization revocation and deadlines cancel the bounded background lane.
  Child process trees are stopped when generation ends. Independent predictions
  can differ from the official desktop; real-model quality was not evaluated.
- Android reports 90 tests: 89 pass and one publication-only updater check skips.
  Lint has zero errors and ten warnings. APK identity is 0.0.6/code6, with the
  existing development signer. No physical Android or macOS runtime acceptance
  is claimed.
- All 12 packaged desktop source/UI files and extra resources match their tested
  inputs; privacy candidate checks report no findings. All 76 installed runtime
  files match the package. After restart, the local gateway reports 0.0.6 and
  completes the official desktop IPC handshake. Configuration, authorizations,
  account settings, preferences, shortcuts and login startup remain unchanged.
- The Web entry and three immutable build6 resources are deployed; fresh HTTPS
  downloads match staged sizes and SHA-256 values. The public API requires
  authorization. Android/desktop packages and manifests are staged locally;
  both public native update feeds still report 0.0.3/build3. No source commit,
  push, native feed promotion, phone installation or service/firewall
  configuration change was performed.

## 0.0.5 / build5

- First-batch UX changes cover connection/sync/read-only presentation, compact
  mobile controls with visible permission warnings, cancellable attachment
  uploads, readable pending-operation cards and confirmed desktop gateway stop.
  Unknown writes and interrupted uploads are not automatically replayed.
- Native sync time comes only from validated original-owner snapshots/patches.
  Projection/read overlays and transport heartbeats do not renew it; old gateways
  remain compatible and show an unknown sync time. Focused Go tests cover this.
- Full uncached Go tests and vet pass; Windows amd64 and macOS amd64/arm64 builds
  use Go 1.27.3. All 96 Node checks pass. Isolated Chromium covers 320/390px and
  desktop layouts, upload cancellation/late replies, stable cancel-button focus,
  pending details and native sync evidence. Screenshots were visually reviewed.
- Isolated Electron tests cover both stop entry points, default cancellation,
  ownership loss after confirmation, duplicate requests, diagnostic staleness
  and English layouts. These tests do not stop a real official desktop task.
- Android reports 89 tests: 88 pass and one publication-only updater check skips.
  Lint has zero errors and ten warnings; two new warnings are existing icon
  resources no longer used by the compact controls. The normal APK builds with
  version 0.0.5/code5, retains the existing development signer and excludes
  rendering-probe code. No physical Android or macOS runtime acceptance is claimed.
- All 12 packaged desktop source/UI files and extra resources match the tested
  inputs; the selected privacy checks report zero findings. All 76 installed
  runtime files match the package. Configuration, device authorizations, account
  settings, preferences, shortcuts and login startup remain unchanged.
- The updated local gateway reports 0.0.5, writes fresh lifecycle diagnostics and
  passes the official desktop IPC handshake. It starts hidden according to the
  existing preference. Public API connectivity returns 401 without authorization.
  No real message, approval or model execution was used to verify the deployment.
- The Web entry and three immutable build5 assets are deployed; fresh HTTPS
  downloads match staged sizes and SHA-256 values. The Android package and native
  manifests are staged, but both public native feeds still report 0.0.3/build3.
  No Git commit/push, phone installation, service installation or server/firewall
  configuration change was performed. Native feed promotion still requires a
  separately authorized commit; a staged manifest is not a published update.

## 0.0.4 / build4

- Web mutation tests reproduce the old cross-connection dispatch race. Writes
  and manual unknown-result acknowledgements now require the original socket
  and authorization; late receipts settle only their original pending journal.
- Desktop pipe error/EOF tests confirm pending calls fail without an uncaught
  stream error or replay. Shutdown clears concurrent pending calls. Partial
  native IPC header/payload write tests fail against the old implementation and
  pass after closing the damaged connection.
- History opens use an `os.Root` handle after normalized containment checks.
  Boundary and Windows long/extended-path tests pass. The concurrent symlink
  replacement test skips because this Windows account lacks symlink privilege;
  no permission or system setting was changed to enable it.
- Diagnostics report unavailable log/marker writes without stopping the gateway,
  retry failed markers and distinguish Windows session query/end notifications.
  Unit tests and isolated Electron clean/abrupt/unwritable-path checks pass.
  These fixtures do not reproduce or prove a fix for the earlier update shutdown.
- Full uncached Go tests and vet pass. Windows amd64 and macOS amd64/arm64 builds
  use Go 1.27.3. All 88 Node checks pass, including version consistency. Isolated
  Chromium UI checks pass. Android has 84 passing tests and one publication-only
  updater check skipped; lint has zero errors and eight warnings, and APK build
  succeeds with version 0.0.4/code4 and the same development signer as build3.
- Packaged desktop identity, all 11 packaged source files and extra resources
  match the tested inputs; the selected privacy checks report zero findings.
  This is not a complete binary-secret audit or physical Android/macOS acceptance.
- The Web entry and three immutable build4 assets are deployed and freshly
  downloaded HTTPS bytes match staged sizes/SHA-256 values. Native installers and
  local update manifests are staged only. The desktop launch approval timed out,
  so the running installation was not stopped/replaced and its health remains
  0.0.3. Both public native feeds still report 0.0.3/build3. No Git commit/push,
  service installation, authorization or server/firewall configuration change
  was performed. Native feed promotion requires a separately authorized commit.
- An online npm advisory scan was not authorized because it would send dependency
  metadata to a third party. The source review is not a claim of zero remaining
  vulnerabilities or current upstream dependency/advisory coverage.

## 0.0.3 / build3

- Turn error messages, tool error messages and rollout error events are projected
  as conversation-local `sessionError` items. Failed/interrupted turns without a
  detailed reason receive an explicit fallback. Empty failed turns are visible;
  normal completed turns do not acquire error cards. Native patch bases remain
  unchanged and lean/history reprojection does not duplicate notices.
- Error cards are expanded plain text in Web and Android. They do not execute
  HTML/Markdown or introduce retries, global blocking or execution-owner changes.
  Messages are limited to 4,000 characters plus a truncation notice; only display
  messages are selected, with common credentials and URLs redacted. Raw error
  objects, additional details and stacks are not forwarded.
- Full Go tests and vet pass. Windows amd64 and macOS amd64/arm64 gateway builds
  use Go 1.27.3. All 79 Node checks pass. Isolated Chromium verifies error-card
  text, turn scoping, removal on subsequent state, XSS resistance and layouts.
- Android reports 85 unit tests: 84 pass and one publication-only updater test is
  skipped before release. Lint and normal assembly pass; version 0.0.3/code 3,
  original package/signing identity and absence of rendering probes are verified.
  All 12 packaged desktop source/manifest files pass the selected privacy scan.
  Public-byte checks are recorded after release. No physical Android or macOS
  runtime acceptance or real model-error reproduction is claimed.
- Publication checks pass for all ten public assets and both main-branch feeds.
  Actual Electron check/download/hash/pre-install verification passes without
  running an installer; Android's public updater test passes with one test and
  no skips. The Web entry and three versioned assets match their public hashes.
- All 76 installed Windows runtime files match the package. The app is reopened
  through its independent application launcher and restores its gateway using
  existing preferences. Health reports 0.0.3 and official desktop IPC connects.
  Desktop/host/listener/tunnel ownership, remote reverse health and public API
  authorization (401 without a session) are verified. Windows TCP enumeration
  did not expose this SSH process's sockets; connectivity is established by the
  actual reverse HTTP check, not inferred from an empty TCP query.
- Configuration, device authorization, IP policy, desktop preferences, shortcuts
  and login-startup registration are unchanged across deployment. All 296 staged
  source blobs pass the selected privacy scan; unrelated local tooling remains
  uncommitted. No runtime data, diagnostic logs or packages are committed.

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
  matches source metadata. All 294 staged source blobs pass the selected privacy
  scan; unrelated local development tooling is not committed.
- All ten published assets are freshly downloaded and match staged sizes and
  SHA-256 values. Both main-branch update feeds report 0.0.2/build2. Actual
  Electron check/download/pre-install verification passes without executing an
  installer; higher-version downgrade and legacy-line protections remain active.
  Android's public update/download/hash check passes with one test and no skips.
- The Web entry and three versioned assets match their public hashes. All 76
  installed Windows runtime files match the package. Gateway health reports
  0.0.2; official desktop IPC connects, the desktop owns its host, listener and
  established tunnel, reverse health succeeds and unauthorized public API returns
  401. Configuration, authorizations, IP policy, preferences, shortcuts and startup
  registration remain unchanged. No physical Android or macOS runtime acceptance
  or real conversation/model execution is claimed.

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

# Verification Status

This public record intentionally excludes real conversation titles, IDs,
content, device identifiers, account counts, private paths and deployment logs.
Private test evidence is not part of the source repository.

## Desktop-Authoritative Read State (0.0.23 / build59)

- Read synchronization no longer reads or checks auth.json or the official
  global-state file. Historical gateway receipts are preserved but no longer
  override original-owner read/unread flags on Web, Android or session lists.
- Live native context hints are connection-bound and include the desktop's
  execution-storage identity mode. Hints and reflected broadcasts never confirm
  a read; only a matching original-owner snapshot/patch does. Uncertain writes
  are not automatically replayed, and notifications retain their separate scope.
- A passive real-desktop probe confirms that original-owner snapshots include
  read flags. It does not mark any chat read. No context is observed during that
  bounded probe; the first usable native context remains a prerequisite for a
  write. No three-UI transactional or physical-device acceptance is claimed.
- Go tests and vet pass, with Windows amd64 and macOS amd64/arm64 gateway builds.
  All 72 Node checks and the isolated Chromium UI suite pass, including pending
  desktop confirmation without optimistic unread clearing. Android reports
  80 unit checks passed and one public-update opt-in skipped; Lint/build pass.
- The staged desktop gateway matches a deterministic rebuild of final sources
  and its standalone Windows gateway. Packaged desktop sources/identity match;
  selected-pattern privacy checks find no credentials or private runtime files.
  The normal APK retains the expected signer and excludes rendering probes.
- All ten public release assets are downloaded again and match their recorded
  sizes and hashes. Both update feeds match the manifests after cache refresh.
  The deployed Web entry and three assets match staged bytes; the public API
  requires authorization. Actual Electron updater check/download/pre-install
  verification passes without executing the installer. The Android public-update
  opt-in passes on retry after an initial TLS handshake interruption.
- On the subsequent deployment request, all 76 local desktop runtime files are
  replaced and hash-checked against the staged build59 package, with the previous
  installation backed up. The installed gateway reports 0.0.23; read-only doctor
  confirms connection to the original official desktop. Configuration, device
  authorizations, preferences, IP/startup settings and historical links match
  the pre-install snapshots.
- Agent restoration still requires the user's manual Start Agent action: both
  automatic-start preferences remain off, and the application window cannot be
  activated by the desktop helper. The post-install health endpoint is not yet
  reachable. Installation checks are not a claim of a running gateway, restored
  public tunnel or real three-client read-mutation acceptance.

## Startup Window Preference And Privacy Review (0.0.22 / build58)

- The requested pull retains the strict Node build guards and isolated updater
  connections from the fetched main commit. Existing local Mac tooling notes
  were preserved across the pull and remain outside this release's commit scope.
- An independent, off-by-default start-minimized preference hides the initial
  desktop window in the tray on a later launch/restart. It changes neither Agent
  auto-start switch. First-run configuration and startup failures remain visible;
  explicit reopening restores the window, and saving does not hide it immediately.
- All 72 Node checks pass, including preference persistence, input validation,
  source hygiene and runtime guards. Isolated Electron checks pass for normal,
  hidden, auto-start-hidden, first-run and failed-start windows, the four existing
  login/app-open startup combinations, the settings UI and English/Chinese UI.
  These use fake Agents and isolated preferences, not real OS startup changes.
- A bounded audit covers the fetched main history (33 commits, 585 unique file
  blobs), current source candidates, selected credential patterns and runtime
  file names. No matching secret/runtime-file findings are detected. Historical
  deployment identifiers and non-noreply Git authors remain public metadata;
  history is not rewritten. Current tests use reserved hosts, with a regression
  that prevents private deployment hosts from returning. See SECURITY.md.
- Go 1.27.3 tests and vet pass; Windows amd64 and macOS amd64/arm64 gateway
  artifacts are cross-built and packaged. Android unit checks report 80 passed
  and one opt-in public-update check skipped; Lint and the normal APK build pass.
  The APK retains the expected signing identity and excludes rendering probes.
- Packaged desktop sources match the tested inputs, and the packaged source
  privacy scan finds no matching credentials or private runtime files. These
  checks do not constitute physical Android or macOS runtime acceptance.
- Published build58 assets (10 files) are downloaded again and match their
  recorded sizes and SHA-256 values. Both main-branch update feeds match the
  release manifests. The deployed Web entry and three versioned assets match
  their staged bytes; an unauthenticated session API request returns 401.
- The actual Electron updater recognizes the current release, offers it to an
  older version, downloads the public installer and passes its pre-install
  recheck without executing it. The opt-in Android public-update test passes
  against build58 (one test, none skipped). No physical device is updated.
- Local Windows replacement was initially deferred while the desktop was locked.
  After the user unlocked it, all 76 runtime files were replaced and hash-checked
  against the staged build58 package, with the previous installation backed up.
  The reopened desktop and gateway report 0.0.22, the Agent is running, and
  read-only diagnosis confirms connection to the original official desktop IPC.
- Post-start configuration, preferences, device authorizations, IP settings,
  login startup registration, shortcut and historical-link records match the
  pre-install snapshots. The public session API still requires authorization
  (401 without credentials). No real message or approval is submitted; these
  checks are not physical-phone or complete end-to-end acceptance.

## System Node Security Update (2026-10-22)

- The Mac system Node command is upgraded from 22.13.1 to official 22.23.4,
  after archive SHA-256, extracted binary hash and signature verification.
  The prior executable is backed up. npm and global packages remain unchanged.
- All 70 Web/desktop/repository/runtime tests pass using the upgraded system
  Node. The Node HTTPS fallback regression verifies pooling is disabled and
  Connection is close, while redirects/compressed bodies remain rejected and
  no credentials are sent. A valid Brotli round-trip smoke check also passes;
  that smoke check alone is not a vulnerability regression or exploit test.
- Nine pre-update Node processes remain in the current client/tool ancestry;
  no such process is killed. Their owning clients must be restarted separately
  before those in-memory runtimes can be considered updated.
- CXPalm health still reports 0.0.21. No Electron replacement, remote service
  change, source push or new public release is performed in this follow-up.
  The transport change is source-only; embedded Node repair remains unconfirmed.

## Mac Local Installation Verification (2026-10-22)

- The complete Mac Intel application is updated from build55 to 0.0.21/build57,
  not just app.asar. All 532 packaged file/directory/symlink entries match the
  verified staged bundle. The installed gateway reports Go 1.27.3; the installed
  Electron runtime reports Electron 44.9.3 and Node 24.22.0.
- After restart, local health reports 0.0.21 and the desktop owns its gateway.
  Read-only diagnosis connects to official desktop IPC. The configured SSH
  process has an established TCP connection; no public-phone or remote Web
  end-to-end acceptance is inferred from that check.
- Connection configuration and desktop preferences match the pre-restart
  backup hashes. Existing gateway settings and device authorization fields
  match the database snapshot. The previous application and full profile
  backup are retained in ignored private storage.
- Existing IP policy rejects a direct local probe with 403. A local test using
  the configured trusted-proxy header and an allowed IP still receives 401
  without device credentials. No access-control setting is relaxed.
- No real chat message, global Node replacement, remote deployment change,
  source push or public release occurs. This is installation/health/IPC
  verification, not complete UI or physical-phone acceptance. The outstanding
  Electron embedded-Node fixes remain unconfirmed.

## Build Runtime Guard Recheck (2026-10-22)

- Rechecked the build57 source baseline. Official Node 22.23.4 and Go 1.27.3
  archives for the Mac build host match the official SHA-256 metadata. They
  are project-local build tools, not replacements for the system installation.
- All 70 Web/desktop/repository/runtime Node tests pass, including five new
  runtime-guard checks. The old Node 22.13.1 CLI is rejected; an offline npm ci
  dry-run on that runtime fails with EBADENGINE before dependency installation.
- Go default tests and vet pass with Go 1.27.3; live opt-in variables are unset.
  Windows amd64 and macOS amd64/arm64 gateway builds succeed and report embedded
  Go 1.27.3. Cross-build success is not runtime acceptance on those platforms.
- An unsigned local Mac Intel desktop bundle is generated with Electron 44.9.3
  and the new gateway; the packaged gateway matches the verified binary hash.
  The Electron runtime reports Node 24.22.0, so the outstanding upstream Node
  fixes remain open as described in docs/RUNTIME-SECURITY.md. No application UI
  acceptance, installed-app replacement, service restart or publication occurs.
- Working-tree repository checks and diff whitespace checks pass. Existing
  system tools, installed applications and remote deployment state are not
  upgraded by these local build and source changes.

## Independent Read Receipts And Runtime Refresh (0.0.21 / build57)

- CXPalm commits exact visible-reply read receipts in its own database before
  optional official synchronization. Missing official identity and an official
  timeout do not block or erase them. New replies and stale receipts remain
  distinct; offline, archived and failed-storage requests do not report success.
  List and conversation projections agree across receivers and after restart.
- Desktop refresh reconnects a closed management pipe once, without replaying
  the original command or changing Agent auto-start preferences. Final packaged
  desktop source files match the source checked by isolated Electron UI tests;
  the concurrent reconnect and explicit-start checks pass.
- Go tests/vet pass with Go 1.27.3. Windows amd64 and macOS amd64/arm64 gateway
  archives contain binaries whose embedded build information reports Go 1.27.3;
  Mac archives also verify architecture and executable mode. They are standalone
  gateways, not Mac desktop applications or Mac runtime acceptance.
- All 65 Node checks and isolated Chromium regressions pass on system Node
  22.23.4. Android: 81 tests, 80 passed and one public-network opt-in skipped;
  Lint and normal APK assembly pass. The APK retains its signer and excludes
  rendering probes. No fresh physical-phone or emulator acceptance is claimed.
- The Windows desktop is rebuilt with Electron 44.9.3; its actual packaged
  runtime versions are checked separately from system Node. Electron's official
  blocked-CA-key patch is verified in source, but the embedded Node still reports
  24.22.0, not the complete 24.22.1 security release. The independent browser, OS,
  CA and deployment-host audit and remaining updates are documented in
  [runtime security](docs/RUNTIME-SECURITY.md); no OS trust or server policy is changed.
- All ten public release assets (including three standalone gateway archives)
  match staged size/SHA-256 values. Both repository update feeds match build57;
  four deployed Web files match staged bytes. Actual Electron and Android
  update transports download and verify the public packages. No installer is
  executed by the update transport tests.
- Local installation replaces and hash-verifies all 76 packaged runtime files,
  not only the EXE/ASAR, with a complete prior-install backup. Protected config,
  devices, IP policy, preferences, shortcuts and startup registration are
  unchanged. The desktop UI restores its Agent; health and read-only official
  desktop diagnosis report 0.0.21 with official desktop IPC connected. Public
  unauthenticated session access still returns 401. No real chat action or
  physical-device read/notification acceptance is claimed.

## Account-Independent Reminders (0.0.20 / build56)

- Notification preferences, event journals and transition checkpoints use a
  stable configured-desktop scope in the gateway database. No official account,
  auth.json, access token or official read-state file is required for reminders.
  Official cross-device reads keep their existing, independent validation.
- Tests cover no auth/read-state files, auth-mode changes, logout, account
  switching, restart checkpoint recovery and a separate desktop scope. An
  authenticated two-receiver WebSocket fixture receives opted-in events without
  an official identity and still rejects the revoked receiver. Initial snapshot
  baselines, opt-in generations and durable-failure guards remain tested.
- Legacy account-scoped records are retained without any migration or merging;
  chats must be opted in once in the new scope. Subsequent official account
  switches retain preferences and journals. No real conversation mutation or
  real account switch is performed by these fixtures.
- Go tests/vet and Windows amd64 / macOS amd64+arm64 cross-builds pass. All 65
  Node tests and isolated Chromium regressions pass, including toggling reminder
  preferences while official read synchronization is unavailable.
- Android: 81 tests, 80 passed and one public-network opt-in skipped by default;
  Lint and normal APK assembly pass. The normal APK excludes rendering probes
  and retains its package and signer. This release does not claim a fresh
  emulator, physical-phone notification-delivery or macOS runtime acceptance.
- All six public release assets and four deployed Web files match staged sizes
  and SHA-256 values; both stable repository feeds match build56. The actual
  Electron updater passes current/upgrade detection, public download and
  pre-install recheck without executing an installer. Android's opt-in public
  updater download/hash test passes separately.
- Installed desktop/gateway hashes match the package. The desktop UI restores
  its owned Agent and local health reports 0.0.20 while auth.json remains absent.
  Protected configuration, authorizations, preferences, IP policy, shortcuts
  and OS startup settings remain unchanged; the public session route requires
  authorization (401). No real task is sent or real alert triggered for this
  check. No server configuration, permission or database migration is performed.

## Session-Scoped Approval And Read Availability (0.0.19 / build55)

- Web and Android approve for the current conversation only when the original
  owner request explicitly offers persistence. Command and file requests need
  `acceptForSession` in `availableDecisions`; MCP tool confirmations need the
  original request to offer the session scope and are answered with the owner's
  scoped form response. Permission-profile requests stay turn-only, no permanent
  `always` grant is exposed, and nothing is cached globally by the gateway.
  Both clients confirm the scope before sending; uncertain mutations stay
  non-replayable and the original desktop owner keeps its own safety checks.
- Unavailable desktop identity is now an optional-feature state, not a read
  error: Web and Android keep chatting normally, report cross-device read
  receipt and per-chat notification features as temporarily unavailable, never
  guess an account, never mark an unconfirmed reply read, and never replay a
  previous account's notification journal. Saved preferences, authorization
  and IP policy are preserved.
- Go tests/vet, Windows amd64 and macOS amd64/arm64 cross-builds pass. All 65
  Node checks pass and the isolated Chromium UI suite passes, covering
  approve-once/session/decline/cancel for MCP confirmations and the
  identity-unavailable path. The approval steps in that suite now await the
  exact `chat.approve` frame: a preceding `confirm()` dialog can delay an
  earlier approval, so the previous "any chat.approve" wait was a harness race
  that also failed on the previous commit. This is a test-synchronisation fix,
  not a product change.
- Android: 81 unit tests, 80 passed and one public-updater opt-in skipped by
  default; Lint and normal APK assembly pass. The normal APK excludes rendering
  probes, keeps its package ID and existing signer, and survives an isolated
  Android 35 emulator cold start (process alive, empty crash buffer).
- Twenty-two isolated Android 35 emulator rendering cases pass, including the
  new session-scoped approval probe and the identity-unavailable probe. They
  use synthetic fixtures; no real approval is submitted and no physical-phone
  acceptance is claimed.
- All six public release assets and the four deployed Web entry/module files
  pass fresh byte and SHA-256 checks. Both repository feeds match build55. The
  actual Electron updater recognizes 0.0.19 as current, offers and downloads
  the public upgrade, and rechecks it before installation without executing the
  installer; the actual Android updater's opt-in JVM check reads build55 from
  the public feed and verifies the downloaded APK.
- Installed desktop/gateway file hashes match the package and protected
  configuration, preferences, authorizations, IP policy, shortcuts and OS
  startup settings are unchanged. The reopened desktop app started the agent
  and local health reports 0.0.19; the public session route still returns 401
  without authorization. No server, firewall, TLS, proxy or permission change
  is part of this release.

## MCP Tool Confirmation Buttons (0.0.17 / build53)

- A read-only original-owner subscription confirms the previously unsupported
  request shape: mcpServer/elicitation/request, form mode, an empty object
  schema and mcp_tool_call metadata. No real request is approved by diagnosis.
- Web and Android now expose approve-once, decline and cancel for that shape.
  The gateway forwards only the action and form content to the original owner,
  without persistence metadata. Official owner-side safety checks still apply.
- URL, identity/authentication and nonempty or unsupported forms fail closed.
  Null command availableDecisions uses the protocol default; empty arrays and
  unsupported choices remain excluded. Actions precede expandable details.
- Go tests/vet, Windows amd64 and macOS amd64/arm64 cross-builds pass. All 65
  Node checks and isolated Chromium MCP accept/deny/cancel regressions pass.
- Android: 81 tests, 80 passed and one public-updater opt-in skipped by default;
  Lint and normal APK assembly pass. Package identity and signer are retained;
  the normal APK excludes rendering probes. Physical-phone acceptance remains
  separate from fixtures and emulator checks.
- Five isolated Android 35 cases pass, including MCP confirmation, identity
  exclusion and an actually visible button in the full conversation composer.
  The screenshot uses synthetic text and does not represent a real approval.
- The probe-free APK survives an emulator cold start. All six public assets
  and four deployed Web files pass byte/hash verification; both public feeds
  match build53. Android and Electron native download/hash checks pass.
- Installed desktop/gateway hashes match build53 and protected configuration
  and authorizations remain unchanged. Gateway startup and real-request
  acceptance still require runtime verification; no real approval is automated.

## Turn-Scoped Permission Approvals (0.0.16 / build52)

- Web and Android handle command, file-change and permission-request approvals.
  Permission grants are constructed from the current owner's request and use
  turn scope only. Unknown permission-profile fields cannot be approved.
- Native pipe fixtures verify accept/deny routing to the original desktop
  owner, numeric request-ID preservation and the permission-response protocol.
  Expired or changed requests are rejected before dispatch; native transport
  uncertainty remains non-replayable. These are fixtures, not real approvals.
- Go tests/vet, Windows amd64 and macOS amd64/arm64 cross-builds pass. All 64
  Node checks and isolated Chromium approval/denial/cancellation checks pass.
- Android: 80 tests, 79 passed and one public-updater opt-in skipped by default;
  Lint and normal APK assembly pass. The APK retains its package ID and signer
  and excludes rendering probes.
- Five isolated Android 35 emulator cases pass, including confirmation before
  approval, offered-decision filtering and the fixed approval area. The normal
  APK also survives an emulator cold start. No physical-phone or macOS runtime
  acceptance is claimed; no real conversation request is approved by tests.
- All six public release assets and four deployed Web files pass fresh byte
  and SHA-256 checks. Both public repository feeds match build52. Android and
  Electron native public-download/hash checks pass; the Electron installer is
  rechecked before installation but is not executed by that test.
- Installed desktop/gateway file hashes match the package and protected local
  settings and authorizations remain unchanged. This does not assert that a
  stopped gateway was started or that live approval acceptance was performed.

## Unsupported Archive Actions Hidden (0.0.15 / build51)

- No safe owner-side archive forwarding entry has been established for the
  current desktop integration. Web and Android remove both archive actions;
  archived-history browsing and read-only guards remain available. This is a
  UI fallback, not a claim that desktop-held chats can now be archived remotely.
- Go tests/vet and Windows amd64 / macOS amd64+arm64 cross-builds pass. The
  macOS binaries are not runtime-tested here. All 63 Node checks and isolated
  Chromium UI regressions pass, including desktop/mobile archived reading.
- Android: 79 tests, 78 passed and one public-updater opt-in skipped by default;
  Lint and normal APK assembly pass. The normal APK excludes rendering probes
  and retains the package identity and existing project signer.
- Four isolated Android 35 emulator cases pass: active and archived lists
  verify their actual accessibility tree has a rendered session but no archive
  or unarchive action; rejected and unknown historical receipts still render.
  Accessibility must connect before Compose renders to expose the test tree.
- Historical receipts, audit fences and non-replay protection are retained.
  No real chat is resumed, archived or unarchived by these tests. No physical
  phone acceptance is claimed.

## Compact Android Composer (0.0.14 / build50)

- Go tests and Windows amd64 / macOS amd64+arm64 cross-builds pass for the
  approved source. No Go or Web runtime logic changes in this Android release.
- All 36 i18n and Web protocol checks pass. Android unit tests, Lint and normal
  APK assembly pass; the default suite skips the opt-in public updater check.
- Isolated Android 35 emulator previews cover reading and a multiline draft
  with the keyboard open. Accessibility inspection confirms a single input,
  a labelled notification bell and no separate notification switch row.
- The normal APK excludes rendering probes and retains the package ID and
  existing signer. No physical-phone acceptance is claimed.
- Web and desktop/gateway remain 0.0.13/build49; no service restart, policy
  change, configuration migration or real-chat mutation is needed.
- All three public Android release assets pass fresh size/SHA-256 checks.
  The main Android feed matches build50; the desktop feed remains build49.
  The Android updater's opt-in JVM check reads the public feed and verifies
  the downloaded APK. It does not install the APK on a physical phone.
- Merge retains the remote desktop form-input and macOS titlebar changes.
  All 25 desktop unit tests and isolated Electron UI regression checks pass;
  these desktop source changes are not repackaged in this Android-only release.

## Archive Rejection Recovery (0.0.13 / build49)

- Go tests and vet pass; Windows amd64 and macOS amd64/arm64 cross-builds pass.
  The macOS binaries are not runtime-tested here.
- Both opt-in official CLI integration tests pass in fresh, credential-free
  CODEX_HOME directories: ownerless metadata lifecycle and a living writer's
  explicit refusal. No real chat is resumed or archived by these tests.
- All 63 Node checks and isolated Chromium/Electron UI fixtures pass. Historical
  error recovery requires the original archive command fingerprint; transport
  uncertainty and unrelated multi-step operations remain protected.
- Android: 79 tests, 78 passed and one public-network opt-in skipped by default;
  Lint and normal APK assembly pass. Eleven isolated emulator rendering cases
  pass, including rejection, other-chat, unknown and legacy operation states.
  The normal APK excludes probes, retains the existing signer and survives an
  emulator cold start. No physical phone acceptance is claimed.
- Known rejection releases only its matching client lock, retains drafts and
  attachments, and displays a dismissible conversation-local failure notice.
  The original archived-state flag is not optimistically changed.
- Limitation: the current official follower protocol has no archive forwarding
  request. While an official desktop holds the writer lease, archiving must be
  done there. No writer lock is stolen, no second executor is started, and the
  original failed operation is never replayed or removed from the audit log.
- All six public GitHub assets and the deployed Web entry/versioned modules
  pass fresh size/SHA-256 checks. Both repository feeds match build49. The real
  Android and Electron updaters verify their public downloads; the installer
  is not executed by the updater test.
- Local desktop/gateway health reports 0.0.13, owned by the desktop UI. Public
  session lookup still requires authentication. Deployment preserves config,
  preferences, authorizations, IP policy and OS startup settings.

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
- All three GitHub release assets and the deployed Web entry/versioned modules
  pass fresh public size/SHA-256 checks. The Android updater's separate opt-in
  JVM test reads build48 and verifies its downloaded APK. Both repository feeds
  match their committed manifests; local gateway health remains 0.0.10.

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

- Web / Android / desktop / gateway source: 0.0.13 / versionCode 49.
- Desktop package: releaseBuild 49.
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

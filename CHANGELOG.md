# Changelog

This source-oriented record omits private deployments and conversation details.
Versions describe implemented changes, not physical-device acceptance.

## Unreleased

- Prepare public documentation, synthetic deployment examples and repository
  privacy checks. Keep local account and runtime state outside Git.
- Project license and public repository publication remain pending.

## 0.0.16 (build52)

- Support official permission-request approval and denial on Web and Android,
  forwarding to the original desktop owner without a second executor.
- Grant only the exact requested permissions for the current turn; never offer
  permanent or session-wide permission grants.
- Respect offered command/file decisions, including cancellation; unknown and
  expired requests remain protected and uncertain mutations are never replayed.
- Pin Android approvals above the composer and confirm grants with full details.

## 0.0.15 (build51)

- Remove Web and Android archive/unarchive buttons until a safe desktop-owner
  forwarding path is available. Keep archived history browsing and read-only guards.
- Direct archived-chat users to the official desktop to unarchive, rather than
  referring to a removed mobile action. Preserve historical receipt recovery.
- Add isolated active/archived list regressions; do not steal writer leases,
  resume main chats in another executor or rewrite official archive metadata.

## 0.0.14 (Android / build50)

- Compact the composer into one row; the input grows from one to four lines.
- Move conversation notifications to an accessible bell in the horizontally
  scrollable controls strip, without a separate full-width switch row.
- Put attachments, image paste and active-turn steering in the attachment menu;
  show stop only during an active turn. Preserve send/queue and safety guards.
- Add isolated reading and keyboard composer previews, excluded from normal APKs.
- Web and desktop/gateway remain 0.0.13/build49 and need no restart.

## 0.0.13 (build49)

- Treat exact official active-writer archive refusals and known archive
  precondition failures as rejected, not indefinitely unknown or successful.
- Verify original command fingerprints before releasing legacy client archive
  locks. Keep the historical journal and never replay the old request.
- Add contextual dismissible Web/Android failure notices; rejected operations
  preserve drafts and attachments and do not block unrelated conversations.
- Bound Android notice height and add isolated archive-state rendering checks.
- Online archive forwarding is not available in the current official follower
  protocol. While the original desktop retains the writer lease, archiving
  still requires that desktop; no lease theft or second executor is used.
- Limit Windows agent resources to its executable, excluding cross-build outputs.

## 0.0.12 (Web / Android / build48)

- Scope unresolved operations, receipt details and write protection to the
  affected conversation instead of blocking the entire computer.
- Keep other conversations, new-chat creation and computer navigation usable;
  creation has its own pending-operation scope.
- Correlate recovery and manual inspection by operation ID. Reconnect queries
  receipts only and never replays uncertain writes. Retain legacy journals;
  ID-only Android records require inspection before releasing write protection.
- Windows/gateway remain 0.0.10/build46 and do not need a restart.

## 0.0.11 (Android / build47)

- Fix a frame-order race in question navigation that could pass a negative
  index to the list shortly after entering a conversation.
- Validate slider releases against the current question list rather than
  indexing a list that may have changed during the gesture.
- Add opt-in emulator rendering checks, excluded from normal APKs. No message
  content is truncated or deleted. Web and Windows/gateway remain 0.0.10/build46.

## 0.0.10

- Release the Android pending lock when the socket definitively rejects enqueue,
  retaining the draft and attachments rather than leaving sending disabled.
- Resolve absent receipts with durable device-scoped `not_sent` fences. Delayed
  requests cannot execute after recovery; uncertain operations never auto-replay.
- Ignore Android receipts that do not match the current pending operation.

## 0.0.9

- Use credential-free native HEAD probes to validate GitHub download redirects
  before fetching bodies; Electron cancels redirects in Fetch manual mode.
- Supersede build44 without replacing its published artifact bytes. The corrected
  transport passes a real Electron GitHub download/hash/pre-install check.

## 0.0.8

- Move Android and Windows update manifests to this repository and immutable
  packages to GitHub Releases. Preserve package identity and anti-downgrade checks.
- Permit only bounded, credential-free download redirects to GitHub CDN paths
  scoped to this repository. Manifest requests do not follow redirects.
- Use Electron's native networking for desktop updates so the user's existing
  operating-system proxy can be respected without changing proxy settings.

## 0.0.7

- Align session membership with official records; exclude metadata-only stubs
  from normal lists and prevent removed/archived cached owners retaining rights.
- Hide ordinary healthy sync notices and retain actionable errors.
- Web/Android layout build43 moves the latest-message arrow to the right;
  desktop release remains build42.

## 0.0.6

- Refresh stale original-owner subscriptions and recover quiet WebSocket
  connections without replaying mutations.

## Initial 0.0.1 Source Line

- Independent gateway, Web, Android and local desktop clients using official
  desktop execution ownership, revocable authorization and bounded APIs.
- Later iterations add attachment/image handling, opt-in notifications,
  history/read-state synchronization and session controls. This summary does
  not reconstruct unverified per-build release dates.

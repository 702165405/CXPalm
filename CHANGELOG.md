# Changelog

This source-oriented record omits private deployments and conversation details.
Versions describe implemented changes, not physical-device acceptance.

## Unreleased

- Prepare public documentation, synthetic deployment examples and repository
  privacy checks. Keep local account and runtime state outside Git.
- Project license and public repository publication remain pending.

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

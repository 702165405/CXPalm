# Verification Status

This public record intentionally excludes real conversation titles, IDs,
content, device identifiers, account counts, private paths and deployment logs.
Private test evidence is not part of the source repository.

## Source Baseline

- Source version: 0.0.9.
- Android: versionCode 45; desktop package: releaseBuild 45.
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

# Verification Status

This public record intentionally excludes real conversation titles, IDs,
content, device identifiers, account counts, private paths and deployment logs.
Private test evidence is not part of the source repository.

## Source Baseline

- Source version: 0.0.7.
- Android: versionCode 43; desktop package: releaseBuild 42.
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

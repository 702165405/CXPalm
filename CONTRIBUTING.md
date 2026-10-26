# Contributing

The project license is pending maintainer selection. Do not assume that the
repository is licensed under MIT or that third-party code can be copied into it.

## Issues

Include platform/OS, app version and build, expected behavior and minimal
reproduction steps. For crashes, share only a redacted exception type and stack
trace. Never attach full logcat, chat exports, account databases, device tokens,
private paths, IP addresses or screenshots containing real conversations.
Report suspected vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## Changes

- Keep changes small and include a regression test for non-trivial behavior.
- Main chats remain owned by the official desktop. Do not add a second executor.
- Preserve rooted file access, explicit authorization and non-replay of uncertain
  writes. Do not weaken update signature/hash/origin or downgrade checks.
- Use synthetic fixtures, not real chat titles, IDs, content or authentication.
- Do not add keys, generated packages, runtime files or dependency directories.
- Do not run live tests, publish packages, change a server or modify real chats
  as a side effect of a source change.

Follow [development](docs/DEVELOPMENT.md). Run relevant Go/Node/Android tests,
Go vet and Windows/macOS cross-builds. State exactly which tests ran, skipped
or failed and whether a physical device was used. Run the repository privacy
check and inspect the final diff before submitting a change.

# Public Repository Preparation

## What Is Removed Or Excluded

Public documentation omits real conversation titles, IDs, statistics and raw
device/deployment evidence. Original local records remain in ignored storage;
no local authorization, credential, draft or chat data is deleted by preparation.

Local assistant instructions (including machine-specific standing permissions),
private configuration, databases, uploads, logs, backups, signing keys, SDKs,
dependency directories and packages must remain outside Git. `.gitignore` does
not remove a file from existing Git history. Review the actual candidate files,
not just the top-level directory names.

```powershell
node --test scripts/check-repository.test.cjs
node scripts/check-repository.cjs
git status --short
git ls-files --cached --others --exclude-standard
```

The checker reads Git candidates and produces only file/rule locations, not
matching credentials. Passing this heuristic check is not a security guarantee.
Review any future staged diff and the commit history before publication.

## Deliberately Preserved Public Compatibility Identifiers

The Android namespace/application ID `io.github.u702165405.cxpalm` (plus the
debug suffix) identifies the app; it is not an account token. Renaming it breaks
in-place updates and existing app storage compatibility.

Native updater code, matching tests and local release tooling contain the
public origin `https://win.lqilt.top`. It is an existing update trust allowlist,
not a private SSH endpoint or credential. It is intentionally not replaced by
a placeholder in executable code during a privacy-only cleanup: doing so would
break installed-client update behavior. Login origins remain separate.

Before independently distributing a build, review/update all of:

- `desktop/updater.cjs` and its tests;
- `android/app/src/main/java/dev/cxpalm/UpdateProtocol.kt` and its tests;
- `scripts/prepare-desktop-release.cjs`;
- your own package identity, signing configuration, immutable URLs/manifests
  and actual public-byte/updater checks.

Do not broaden allowlists, bypass signature/hash/downgrade checks or assume a
source push publishes a binary. Nginx/config examples use reserved example
domains and fabricated paths, not the operator's real server configuration.

## Before The First Public Push

- Select the project license explicitly. No MIT/GPL grant has been made yet.
- Confirm project asset provenance and transitive dependency notices.
- Establish a private vulnerability-reporting channel; do not invent contacts.
- Keep the reported Android crash visible as unresolved until diagnosed.
- Review intended owner/repository name/visibility and the exact commit.
- Create/push only after user authorization; do not upload private test evidence
  or old project resources, credentials or runtime state.

# Native Update Manifests

`download.json` (Android) and `desktop-download.json` (Windows x64) describe the
current release assets in this repository's GitHub Releases. They contain only
public package metadata, sizes and SHA-256 values, never login credentials.

Use `scripts/prepare-android-release.cjs` and
`scripts/prepare-desktop-release.cjs` to stage manifests in ignored local output.
Validate package/signing identities, publish immutable version/build-qualified
release assets and verify actual public downloads before promoting manifests.
Do not commit package binaries or change an existing release asset's bytes.

The clients use raw GitHub URLs for manifests and permit only bounded redirects
from this repository's release assets to its GitHub CDN resource paths. Copies
of the manifests can accompany the release, but the `main/updates/` files are
the stable check endpoints. Forked distributions must change the repository
allowlists and CDN repository ID deliberately, not disable the checks.

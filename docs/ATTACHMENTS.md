# Attachments

Web and Android upload to the same authenticated gateway. Select a file with
the composer button, paste an image into the Web text box, or use Android's
Paste image button / a keyboard that supports rich-content insertion. Web also
accepts file drops. No broad storage or camera permission is required.

- Up to 8 attachments per message, 20 MiB per file; empty files are rejected.
- PNG, JPEG and GIF are validated locally and sent as native image inputs.
  Other formats remain ordinary file attachments.
- Attachments can accompany text, be sent alone, guide a running turn, or join
  the durable queue. Native Goal creation accepts text only.
- The draft lists each uploaded filename/size and lets you remove it before
  sending. Removing it from the draft does not delete a file already referenced
  by another operation or queued message.
- Uploads live under the configured gateway data directory, not the repository
  or project workspace. Device/chat-bound opaque IDs are resolved on the
  computer; clients cannot supply local paths or download arbitrary files.
- Uploaded file content is never executed by CXPalm. The official owner receives
  native image inputs and explicit local-file references. Codex still enforces
  its existing tool permissions when reading ordinary files.

The upload endpoint is `POST /api/v1/attachments?thread=<chat-id>`, with one
multipart field named `file`. It requires the same exact Host/Origin, IP policy
and revocable bearer/cookie authorization as the WebSocket API. Successful
responses contain only `{id,name,size,image}`. Chat commands carry attachment
IDs, never bytes or caller-provided paths. No public download route is added.

Storage is bounded to 512 MiB / 512 uploads overall and 256 MiB / 128 uploads
per device. Admission reserves one maximum-sized file before writing. Failed
uploads are removed. Sent files are retained so later model reads and queued
turns do not lose their source; they are not silently garbage-collected. If the
quota is reached, stop the gateway and remove only upload directories that are
no longer referenced or needed. Keep runtime data out of Git.

The CXPalm reverse proxy allows a 20 MiB total request body, including multipart
headers. Files near the 20 MiB file limit require a small framing margin. The supplied
Nginx template contains that limit; changing a template does not change a live
server. Never overwrite old immutable release artifacts when publishing this
new version line.

## Returned images

Native `imageGeneration` / `imageGenerationCall` items and paired `imagegen`
rollout outputs are projected as image messages, even without text. Saved paths
are preferred over duplicate inline bytes; hidden tool text is not forwarded.
Generated image bodies are not deferred behind a tool-output expansion.

Both clients show authenticated thumbnails from Markdown image references,
native image content and image-generation results. Click/tap to open a larger
view; Android supports pinch zoom. Image-only native messages remain visible.

`POST /api/v1/chat/image` accepts only `{thread,turn,item,image}` identities.
The server re-resolves the referenced message and image, never a caller path.
Raster content is limited to PNG/JPEG/GIF/WebP and 20 MiB; decoded dimensions
are bounded, including WebP header dimensions. SVG/HTML,
remote HTTP images, outside-root paths and symlink escapes are rejected. Allowed
local roots are that chat's working directory, gateway uploads and the configured
Codex home's `generated_images`, `artifacts` and `visualizations` directories.
On Windows, ordinary drive/UNC paths and their `\\?\` extended forms
are compared as equivalent. The file still opens through the original allowed
root, preserving rooted traversal and symlink protections.
Missing files and unsupported references show a retry state rather than a
broken anonymous file URL. Other output directories require moving the image
into an allowed root; this is not arbitrary filesystem access.

Web renders returned bytes into canvas under the existing strict CSP; it does
not enable remote/blob image loads or unsanitized HTML. Android samples previews
and large images to bound bitmap memory and does not persist image bytes in the
reading cache. Returning image references in a cached transcript does not bypass
server authorization or promise offline image availability.

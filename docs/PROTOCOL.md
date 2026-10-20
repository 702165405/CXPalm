# CXPalm protocol v1

Only methods advertised in the WSS hello capabilities are supported. Unknown
methods fail closed; caller-supplied RPC names, executable commands and local
filesystem paths are not forwarded.

## Authorization

`POST /api/v1/login` accepts `{username,password,remember,name?}` and returns a
revocable device bearer. Browser login at `/api/v1/browser/login` instead sets a
host-only Secure / HttpOnly / SameSite=Strict cookie under `/api/v1/`, with no
bearer in the response body. Remembered sessions expire after 30 days; temporary
sessions have a 24-hour server maximum. Browser temporary cookies are session-only.
Logout revokes a session. Local account rotation atomically revokes all sessions.
There is no public account setup, reset, link-exchange or QR login endpoint.

Exact Host/Origin and persisted IP policy protect public APIs. Login size,
deadline, concurrent password work and attempt rate are bounded. Passwords use
individually salted PBKDF2-HMAC-SHA256. Device expiry/revocation and IP policy
are checked throughout WebSocket delivery. Tokens never belong in query strings,
fragments or subprotocols. Responses are no-store and nosniff.

## WebSocket

`GET /api/v1/ws` carries text JSON `{id,method,...fields}` commands. Replies use
`{id,type:result|error,data|error}`. Reads include session browse/list/subscribe,
chat subscribe/history/item/catalog/read and operation lookup. Mutation methods:

- `sessions.create`, `sessions.rename`, `sessions.archive`, `sessions.unarchive`;
- `chat.send`, `chat.steer`, `chat.stop`, `chat.approve`;
- `chat.queue`, `chat.queue.cancel`;
- `chat.settings`, `chat.goal`, `chat.compact`, `chat.notifications`.

IDs are durable device-scoped operation IDs. Save before transmission; query
`operation.get {requestId}` after timeout or disconnect instead of resending.
A matching duplicate returns its original receipt. Changed content under the
same ID fails. `confirmed` means the invoked operation returned, not that a model
task finished. `unknown` never replays. `operation.acknowledge` requires explicit
`text: inspected`; it dismisses a warning without making the original ID executable.

Chat identity is checked against official history and bound to its original
owner. No main thread is independently resumed. Native execution snapshots are
separate from phone projections, lazy history and versioned delta streams.
A stream gap requests another read-only subscription, never a mutation replay.

## Input and controls

Messages are bounded to 100,000 UTF-8 bytes and at most 8 unique catalog Skill
IDs. Skill paths are resolved locally. `chat.send`, `chat.steer` and `chat.queue`
also accept at most 8 device/chat-bound attachment IDs. Text may be empty only
when attachments are present. See [attachments](ATTACHMENTS.md) for the multipart
upload route, rooted storage and image/file conversion.

`POST /api/v1/chat/image` reads only a raster image identified by its authorized
chat/turn/item/reference ID. It does not accept a path, expose a public URL or
fetch remote origins. See attachment documentation for allowed roots and limits.

`chat.settings` accepts explicitly selected model/effort/mode/permission fields
and the current controls revision. Stale revisions fail. Permissions include
read-only, manual workspace approval, automatic approval reviewer with workspace
sandbox, and explicit-confirmation full access. Settings apply to the next turn;
they do not replay messages or disable the official owner's own arbitration.
Native Goal controls and context compaction remain allowlisted, revision/status
checked operations; compact is rejected while active. Goal creation is text-only
and is not queued. Context use reports actual last-input/window occupancy.

Queued messages belong to the gateway, not the desktop queue. At most 16 pending
items per chat. Skills and attachment IDs are frozen. A head remains until its
client ID is observed natively; ambiguous results pause it, including after a
restart. Device authorization/IP policy and attachment scope are rechecked
before original-owner dispatch.

Accepted steering receives a durable presentation record and native-echo
deduplication; see [sessions](SESSIONS.md). It never manufactures a successful
entry for an uncertain operation.

## Notifications

Chats are muted unless explicitly opted in with `chat.notifications`. Preferences
are account/host/chat scoped, shared across receivers and included in live state.
Live and replay delivery both filter the current subscription generation.
`notifications.subscribe` remains independent of the displayed conversation.
See [notification semantics](NOTIFICATIONS.md) for cursor and background limits.

## Local desktop boundary

The `desktop-host` exchanges bounded JSON lines over inherited stdin/stdout,
not a public management listener. Allowlisted operations cover lifecycle,
authorization revocation, metadata/audit, IP policy, login settings and diagnostics.
The renderer is sandboxed and isolated. Connection settings are validated locally
with revision checks; no renderer-selected executable or arbitrary config path
crosses IPC. External gateways and official desktop processes are never adopted
or stopped.

Direct file browsing, shell/terminal, Git execution and side-chat APIs are absent.
Upload support is not permission to read arbitrary computer files. No external
notification channel, public relay or unauthenticated mutation route is added.

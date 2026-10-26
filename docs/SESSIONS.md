# Sessions and reading

## Source and ownership

Session identity, title, archive state and project membership come from official
Codex metadata. Desktop-origin root chats are listed; CLI/subagent threads are
not independent main-chat executors. Projectless chats remain available.
Creating, renaming and archiving are bounded original-owner operations.

History reads are rooted in the configured Codex home. System/developer messages
are excluded. Recent native state is preferred; older rollout history is read
without modifying the official database. Pages contain up to 20 turns, not a
20-turn retention limit. Long bodies and deferred tool output are fetched on
demand. Both clients offer continuous history and question navigation.

## Selecting a cold historical chat

A client `chat.subscribe` is an explicit selection, unlike background notification
or queue followers. The gateway first waits two seconds for a normal native
follow. If the selected, authorized, non-archived chat still has no live owner,
it requests `codex://threads/<id>` through the original official desktop. This
can change the official desktop's current view; it starts no turn and does not
resume the chat in a second app-server. Concurrent requests are deduplicated. Successful or failed launch attempts have
a one-minute per-chat cooldown; native follow recovery is rate-limited to at
most once every five seconds per disconnected chat.
Canceled selections, already-live chats and background subscriptions do not open
the desktop. Launch success alone never grants write authority: a real native
snapshot remains required. An OS launch failure is reported, not hidden.

Web refreshes both the list and the selected subscription, retaining drafts and
never replaying writes. Online historical preview is labeled as synchronizing,
not as a network outage. Recovered desktop/session-connection notices clear only
when they still match the visible error; unrelated operation notices remain.

## Live state and confirmed guidance

Native snapshots/patch revisions remain immutable. Presentation-only queue,
notification and confirmed-guidance records never become native execution state.
Out-of-order sequences are rejected and reconnect follows read-only state again.
Unknown mutations are never automatically retried.

An owner may acknowledge `chat.steer` with a `turnId` before a corresponding
user item appears in its stream. The gateway stores the explicitly accepted
text/attachment references with the original turn and operation ID, then projects
that record into both live and historical reading. It survives reconnect and
process restart. A later native item is deduplicated by client identity, with
bounded text-occurrence fallback. Rejected or ambiguous guidance is not displayed
as successfully sent. Records are bounded to the latest 256 accepted steers per
chat. This is not a second executor and does not resend anything.

## Whole-list synchronization

The first list request starts one gateway-wide passive sweep of all authorized
non-archived main chats, using 500-row database pages independent of client list
filters/pagination. It reuses original-owner read-only followers. Repeated list
requests do not add duplicate watchers; new/archived chats are reconciled every
15 seconds. Gateway shutdown removes these watches. No part of the sweep opens
official windows, resumes an executor, sends a message or opts into notifications.

List responses include checked/total/live progress. Connected native summaries
have `statusSource: desktop`. A rooted bounded rollout tail supplies terminal
history status with `statusSource: history` when no live owner is available.
A later task-start boundary clears old completion. History cannot establish
current activity, pending approvals or write/archive authority; truly unknown
states remain unknown. Realtime owner data overrides history as it arrives.

Android parsers accept object-property removal through array ancestors, but
still reject removing an array element (which requires resize). Each socket has
its own parser; stale socket generations cannot reset the active stream. Loss
callbacks invalidate their generation before scheduling a retry, close handshakes
are acknowledged, and exponential reconnect backoff resets only after 30 seconds
of stable connection. None of these paths replays uncertain writes.

## Read state and offline mode

Read acknowledgements are bound to the actual latest assistant reply token and
official account/host. Hidden pages, historical windows and cached offline state
cannot mark a new reply as read. The question map follows visible user turns;
explicit navigation or sending can reposition the transcript, not unrelated
settings updates.

Web keeps bounded per-authorization reading caches. Android stores bounded
reading data encrypted. Cached snapshots never restore write, approval or read
acknowledgement authority. Existing content stays visible while reconnecting;
mutations require a fresh connected original-owner snapshot.

## Silent Connection Recovery

A native snapshot/patch renews a per-chat freshness clock. After 15 seconds of
silence a watched live chat receives a read-only follow targeted at its existing
owner. The official desktop returns a current snapshot even for an unchanged
revision. If that confirmation does not arrive within eight seconds, old owner
write authority is removed and discovery follows resume. This never launches a
GUI or replays a mutation. The two-second maintenance tick and transport add time
to these thresholds; they are not an end-to-end latency guarantee.

The gateway sends an application heartbeat after ten seconds without outgoing
traffic. Web/Android invalidate a socket after 30 seconds without any valid frame;
Web also checks on foreground/pageshow/network-return events. Pending writes stay
uncertain until their original receipt is checked. Quiet idle chats are not
assumed to be running merely because the websocket is alive.

## Canonical Membership and Exceptional Notices

The interactive list excludes metadata-only named stubs with no original title
or first user message, matching the official list. Exact ID lookup still permits
safe empty-thread creation and immediate selection. This is view membership, not
a destructive deletion or a write to official SQLite/rollout files.

A completed sweep records current membership separately from cached states.
Archive/deletion is checked against official metadata; confirmed removed owners
lose live/write authority and late native frames cannot revive them. Live counts
only include current members. Transient database errors never imply deletion.
Normal unloaded histories and scan progress do not show a sync banner. Only a
reported synchronization error appears, and it clears after recovery on Web/App.

# Conversation notifications

## Explicit opt-in

Every conversation starts muted. Turn on its notification switch in the Web or
Android composer to receive completion and new-approval alerts for that chat.
The preference is stored on the gateway, scoped to the official account/host
and chat, and shared by that computer's authorized Web/Android receivers.
It is not tied to whichever chat is currently open. Desktop is a gateway only.

The gateway follows only opted-in, non-archived chats. Authorization revocation
and IP policy still apply to every outgoing frame. Disabling a chat immediately
filters live events and reconnect replay. Re-enabling creates a fresh subscription
generation: old events and stale watchers cannot leak back into the new opt-in.
Initial snapshots establish a baseline rather than announcing historical work.
Discovery runs every two seconds, so a task completed before its first monitored
snapshot may be missed. No guaranteed push delivery is claimed.

## Protocol and durability

- `chat.notifications.get {thread}` reads `{enabled,generation}`.
- `chat.notifications {thread,enabled}` is an idempotently receipted mutation;
  `enabled` must be an explicit boolean. It does not change model permissions.
- Live chat snapshots carry `notificationPreference` for synchronized controls.
- `notifications.subscribe {cursor?}` remains a separate read-only socket feed.
  Only events from currently enabled subscription generations can be delivered.
- The gateway journals the latest 256 events and transition checkpoints per
  official account/host in SQLite. Known cursors replay retained eligible events;
  an unknown cursor resynchronizes at the current head, without claiming full
  delivery. Clients retain bounded per-computer IDs and suppress duplicates.

## Receiver limits

Android keeps independent notification sockets per signed-in computer. Tapping
an alert opens its original computer/chat. System permission and the optional
foreground connection service are needed for background delivery. OS termination
or force-stop can prevent alerts.

Web requires an open connected page, browser permission and enabled completion /
approval preferences in Settings. Different computers use their own origin-bound
tabs. Same-origin tabs share deduplication storage and a Web Lock. No credential
is copied to another origin, and there is no background or external push service.

Local transport fixtures are not real-device notification acceptance.

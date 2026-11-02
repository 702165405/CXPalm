# Changelog

## 0.0.3 (build3)

- Show per-turn errors, tool error messages and interrupted/failed fallback notices
  directly inside Web and Android conversations, including read-only history.
- Keep errors bounded and plain-text; omit raw diagnostic objects and redact
  common credentials and error URLs. No automatic message or operation retry.

## 0.0.2 (build2)

- Web and Android can reuse the last newly created chat's notification, model,
  reasoning effort and permission mode, scoped to each computer and client.
- Full access requires fresh confirmation; approval grants and chat content are
  never copied. Reused settings are applied once through the original desktop
  owner after connection, with an explicit warning if not confirmed.

## 0.0.1 (build1)

- Self-hosted Web and Android access to official desktop-owned conversations.
- Device authorization, optional IP allowlisting, conversation controls and approvals.
- Attachments, Markdown/history reading, shared desktop read state and opt-in reminders.
- Android light/system/misty-blue appearance, clear computer switching and compact chat UI.
- Local desktop settings, bounded redacted diagnostics and verified repository updates.

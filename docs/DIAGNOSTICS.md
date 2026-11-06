# Local Desktop Diagnostics

CXPalm records desktop lifecycle events locally to help investigate a disappearing
tray icon, startup failures and disconnected Agent processes. Logs are not uploaded.

- Windows: `%APPDATA%\CXPalm\logs\`
- macOS: `~/Library/Application Support/CXPalm/logs/`
- Development and isolated tests use their own Electron user-data directory.

`lifecycle.jsonl` contains UTC timestamps, a random run identifier, application
version and allowlisted lifecycle fields. Up to three files are retained, each
limited to 256 KiB. `session.json` tracks whether the previous run exited cleanly.
These files are runtime data and must not be committed or served as downloads.

## Interpreting Events

- `window-hidden`: the window was hidden; this alone does not mean the app exited.
- `heartbeat`: a once-per-minute main-process heartbeat helps bound the last known
  active time. Sleep, event-loop stalls and disk failures can also leave gaps.
- `quit-request`, `quit-cancelled`, `quit-confirmed`: the desktop quit flow, including
  whether it came from the tray. Update installation confirmation is logged separately.
- `will-quit`, `process-exit`: the app reached its normal exit path and exit code.
- `system-session-query`: Windows asked whether the session may end; an update
  can request this without a completed logout or shutdown. `system-session-end`
  records the actual end notification separately. Only the fixed reasons
  `shutdown`, `close-app`, `critical` and `logoff` are retained when supplied.
- `main-exception`: exception type and allowlisted application filename/line/column,
  without the exception message or full stack path.
- `renderer-gone`, `child-gone`: Electron reported a subprocess exit and reason/code.
- `host-exit`, `host-timeout`, `host-pipe-error`: the Agent management process or pipe
  failed. Timed-out operations are not automatically replayed.
- `previous-session-unfinished`: the previous run left no clean-exit marker. This can
  happen after a force kill, native crash or power loss, but does **not** prove which
  one occurred, identify who ended the process, or establish an intrusion.

Logs do not contain chat text, passwords, tokens, device identities, domains, IPs,
configuration paths, command lines, raw IPC, stdout/stderr or memory dumps. Agent
actions record only the allowlisted operation name and success/failure. Disk errors
must not interrupt CXPalm. Log retention is bounded, so preserve relevant files soon
after a problem; do not publish raw diagnostic bundles without reviewing them.

If writing the log or session marker fails, the desktop status shows a fixed
warning rather than silently reporting healthy diagnostics. Later events retry
the writes and failed initial marker creation. Neither paths nor raw OS error
messages are exposed, and the app does not change directory permissions or move
logs to another location. A recovered log cannot reconstruct missing events.

This diagnostics addition does not enable antivirus, alter startup settings, add a
background service, automatically restart a crashed process or change authorizations.

## Connection Checks

The desktop separates local gateway/listener ownership, the official desktop IPC
probe, SSH configuration and the public HTTPS entry. Only an explicit IPC probe
records `diagnosedAt`; ordinary status refreshes do not renew that evidence. Results
older than five minutes are shown as stale, and gateway lifecycle changes clear
the previous probe. Configured SSH or HTTPS entries remain unverified: this panel
does not actively probe the public network, and loading a Web page alone does not
prove that its gateway connection works.

Stopping the gateway from the window or tray requires confirmation, with Cancel
selected by default. Ownership is checked again before dispatch. Cancellation
does not send a stop request or record a successful stop. The confirmed action
stops only this window's gateway/tunnel and disconnects clients; it does not stop
tasks running in the official Codex desktop or change authorizations.

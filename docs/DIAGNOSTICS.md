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

This diagnostics addition does not enable antivirus, alter startup settings, add a
background service, automatically restart a crashed process or change authorizations.

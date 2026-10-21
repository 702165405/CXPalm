# Isolated Android Rendering Checks

These sources are enabled only by `-PrenderProbe=true`. A normal APK contains
neither the probe activity nor its instrumentation runner. No extra test library
is required. Use a fresh Android emulator with no saved CXPalm accounts.

```powershell
./scripts/test-android-render.ps1 -Serial emulator-5580 -Gradle gradle -Repeat 3
```

The script refuses physical-device serials. It tests long code, many output
lines, long paragraphs, tables, user messages and complete question-navigation
layouts. `unmatched-question` exercises the temporary mismatch between displayed
turns and the question index during asynchronous rendering. Repetition matters
because the original negative-index failure is a frame-order race.

Developers may pass `-CaptureDirectory .local/your-private-captures` for local
replay. Files must be named `capture-N.json`, containing `snapshot` and
`historyIndex` objects; no captures are bundled in this repository or uploaded by
the runner. Fixture clients reject saved accounts and have no execution owner.

Always rebuild without `-PrenderProbe` and verify that probe classes and manifest
entries are absent before staging a distributable APK. Emulator acceptance is
not acceptance on a user's physical phone.

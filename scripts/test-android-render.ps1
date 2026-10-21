param(
    [Parameter(Mandatory)][ValidatePattern('^emulator-[0-9]+$')][string]$Serial,
    [string]$Gradle = 'gradle',
    [ValidateRange(1,30)][int]$Repeat = 1,
    [string]$CaptureDirectory = ''
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$adb = Join-Path $root '.tools\android-sdk\platform-tools\adb.exe'
if (-not (Test-Path -LiteralPath $adb)) { $adb = 'adb' }
$output = Join-Path $root '.local\android-render-tests'
New-Item -ItemType Directory -Force $output | Out-Null
& $adb -s $Serial get-state
if ($LASTEXITCODE -ne 0) { throw 'Start a fresh isolated emulator first' }
& $Gradle --offline -p (Join-Path $root 'android') -PrenderProbe=true :app:assembleDebug
if ($LASTEXITCODE -ne 0) { throw 'Probe build failed' }
& $adb -s $Serial install -r (Join-Path $root 'android\app\build\outputs\apk\debug\app-debug.apk')
if ($LASTEXITCODE -ne 0) { throw 'Probe install failed' }
$cases = @('long-code','many-lines','paragraph','table','user','chat','unmatched-question','archive-rejected','archive-other-chat','archive-unknown','archive-legacy','composer-preview','composer-preview-long','sessions-active','sessions-archived','approval-permission','approval-decline-only','composer-preview-approval')
if ($CaptureDirectory) {
    $captures = (Resolve-Path -LiteralPath $CaptureDirectory).Path
    $privateRoot = [IO.Path]::GetFullPath((Join-Path $root '.local')) + '\'
    if (-not $captures.StartsWith($privateRoot,[StringComparison]::OrdinalIgnoreCase)) { throw 'Private captures must stay under .local' }
    $destination = '/sdcard/Android/data/io.github.u702165405.cxpalm.debug/files/captures'
    & $adb -s $Serial shell mkdir -p $destination
    if ($LASTEXITCODE -ne 0) { throw 'Cannot prepare isolated capture directory' }
    foreach ($file in Get-ChildItem -LiteralPath $captures -File | Where-Object { $_.Name -match '^capture-[0-9]+\.json$' }) {
        if ($file.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Capture symlinks are not accepted' }
        & $adb -s $Serial push $file.FullName ($destination + '/' + $file.Name)
        if ($LASTEXITCODE -ne 0) { throw 'Capture copy failed' }
        $cases += $file.BaseName
    }
}
foreach ($run in 1..$Repeat) {
    foreach ($case in $cases) {
        $log = Join-Path $output "$run-$case.log"
        & $adb -s $Serial shell am instrument -w -e case $case io.github.u702165405.cxpalm.debug/dev.cxpalm.RenderingProbeRunner *> $log
        $text = Get-Content -LiteralPath $log -Raw
        if ($LASTEXITCODE -ne 0 -or $text -notmatch 'INSTRUMENTATION_CODE: -1' -or $text -notmatch 'result=PASS:') {
            throw "Android rendering regression: run $run, case $case; see $log"
        }
        Write-Output "PASS: run $run, $case"
    }
}
Write-Output 'Only the isolated emulator was used. Build again without -PrenderProbe before publishing.'

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$application = Join-Path $root 'dist\desktop\win-unpacked\CXPalm.exe'
if (-not (Test-Path -LiteralPath $application)) { throw 'Build the desktop app with scripts/build-desktop.ps1 first.' }
$config = Join-Path $root '.local\cxpalm.json'
if (Test-Path -LiteralPath $config) {
    Start-Process -FilePath $application -ArgumentList @('--config',('"{0}"' -f $config)) -WorkingDirectory $root
} else {
    Start-Process -FilePath $application -WorkingDirectory $root
}

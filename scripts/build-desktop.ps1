param([switch]$InstallDependencies, [switch]$Installer)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
& node (Join-Path $PSScriptRoot 'check-runtime.cjs')
if ($LASTEXITCODE -ne 0) { throw 'Update build Node.js before installing or packaging dependencies' }
$go = Join-Path $root '.tools\go\bin\go.exe'
if (-not (Test-Path -LiteralPath $go)) { $go = 'go' }
Push-Location -LiteralPath $root
try {
    & $go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
    & $go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed' }
    $agent = Join-Path $root '.local\desktop-build\agent'
    New-Item -ItemType Directory -Path $agent -Force | Out-Null
    & $go build -trimpath -o (Join-Path $agent 'cxpalm.exe') ./cmd/cxpalm
    if ($LASTEXITCODE -ne 0) { throw 'Agent build failed' }
    Push-Location -LiteralPath (Join-Path $root 'desktop')
    try {
        if ($InstallDependencies) {
            & npm.cmd ci --no-audit --no-fund
            if ($LASTEXITCODE -ne 0) { throw 'Dependency install failed' }
        }
        & node node_modules/electron/install.js
        if ($LASTEXITCODE -ne 0) { throw 'Electron runtime unavailable' }
        & node --test test/*.test.cjs
        if ($LASTEXITCODE -ne 0) { throw 'Desktop tests failed' }
        $metadata = Get-Content -LiteralPath 'package.json' -Raw | ConvertFrom-Json
        $version = $metadata.version
        $releaseBuild = $metadata.releaseBuild
        if ($releaseBuild -lt 1) { throw 'Release build must be a positive integer' }
        $output = "--config.directories.output=../dist/desktop-$version-build$releaseBuild"
        if ($Installer) { & .\node_modules\.bin\electron-builder.cmd --win --x64 $output }
        else { & .\node_modules\.bin\electron-builder.cmd --win --x64 --dir $output }
        if ($LASTEXITCODE -ne 0) { throw 'Desktop packaging failed' }
        if ($Installer) { & node ..\scripts\prepare-desktop-release.cjs; if ($LASTEXITCODE -ne 0) { throw 'Release staging failed' } }
    } finally { Pop-Location }
} finally { Pop-Location }

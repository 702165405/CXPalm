param([switch]$Android)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
& node (Join-Path $PSScriptRoot 'check-runtime.cjs')
if ($LASTEXITCODE -ne 0) { throw 'Update build Node.js before building release artifacts' }
Set-Location -LiteralPath $root
$localGo = Join-Path $root '.tools\go\bin\go.exe'
$go = if (Test-Path -LiteralPath $localGo) { $localGo } else { 'go' }
& $go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
& $go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed' }
New-Item -ItemType Directory -Path (Join-Path $root 'dist') -Force | Out-Null
$version = (Get-Content -LiteralPath 'desktop/package.json' -Raw | ConvertFrom-Json).version
& $go build -trimpath -o "dist/cxpalm-$version-windows-amd64.exe" ./cmd/cxpalm
if ($LASTEXITCODE -ne 0) { throw 'Agent build failed' }
if ($Android) {
    $debugKey = Join-Path $root '.local\android-debug.jks'
    if (-not (Test-Path -LiteralPath $debugKey)) {
        New-Item -ItemType Directory -Path (Split-Path -Parent $debugKey) -Force | Out-Null
        & keytool -genkeypair -noprompt -keystore $debugKey -storepass android -keypass android -alias androiddebugkey -keyalg RSA -keysize 2048 -validity 3650 -dname 'CN=CXPalm Development,O=CXPalm,C=CN'
        if ($LASTEXITCODE -ne 0) { throw 'Project-specific debug key generation failed' }
    }
    & gradle -p android :app:testDebugUnitTest :app:lintDebug :app:assembleDebug
    if ($LASTEXITCODE -ne 0) { throw 'Android checks failed' }
    $androidBuild = [regex]::Match((Get-Content 'android/app/build.gradle.kts' -Raw), 'versionCode\s*=\s*(\d+)').Groups[1].Value
    Copy-Item -LiteralPath 'android\app\build\outputs\apk\debug\app-debug.apk' -Destination "dist\CXPalm-$version-build$androidBuild-debug.apk" -Force
}

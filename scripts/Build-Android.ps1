param(
    [string[]]$Tasks = @(':app:assembleAlphaDebug'),
    [string]$Sdk = "$env:LOCALAPPDATA/Android/Sdk",
    [switch]$Offline
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$rikoRoot = Split-Path $PSScriptRoot -Parent
$rikoLock = Get-Content (Join-Path $rikoRoot 'gradle/toolchains.lock.json') -Raw | ConvertFrom-Json
$rikoJava = Join-Path $rikoRoot ".local/toolchains/$($rikoLock.jdk.directory)"
$rikoGo = Join-Path $rikoRoot '.local/toolchains/go/bin'
if (-not (Test-Path -LiteralPath "$rikoJava/bin/java.exe") -or -not (Test-Path -LiteralPath "$rikoGo/go.exe")) {
    throw 'Run scripts/Initialize-Toolchains.ps1 first.'
}
$rikoRevision = & git -C "$rikoRoot/core/src/foss/golang/clash" rev-parse HEAD
if ($LASTEXITCODE -ne 0 -or $rikoRevision -ne $rikoLock.mihomoCommit) { throw 'Unexpected Mihomo source revision.' }
$rikoCoreChanges = & git -C "$rikoRoot/core/src/foss/golang/clash" status --porcelain
if ($LASTEXITCODE -ne 0 -or $rikoCoreChanges) { throw 'Mihomo source has unrecorded changes.' }
foreach ($rikoPatch in $rikoLock.go.patches) {
    $rikoTarget = Join-Path $rikoRoot ".local/toolchains/go/$($rikoPatch.target)"
    if ((Get-FileHash -LiteralPath $rikoTarget).Hash.ToLowerInvariant() -ne $rikoPatch.patchedSha256) {
        throw "Go Android patch verification failed: $($rikoPatch.target)"
    }
}
foreach ($rikoRequired in @("platforms/android-$($rikoLock.android.sdk)", "ndk/$($rikoLock.android.ndk)", "cmake/$($rikoLock.android.cmake)")) {
    if (-not (Test-Path -LiteralPath (Join-Path $Sdk $rikoRequired))) { throw "Android dependency missing: $rikoRequired" }
}
$rikoLocalProperties = Join-Path $rikoRoot 'local.properties'
$rikoProperties = if (Test-Path -LiteralPath $rikoLocalProperties) { [IO.File]::ReadAllText($rikoLocalProperties) } else { '' }
$rikoProperties = [regex]::Replace($rikoProperties, '(?m)^sdk\.dir=.*\r?\n?', '')
$rikoProperties = "sdk.dir=$($Sdk.Replace('\', '/'))`n$rikoProperties"
[IO.File]::WriteAllText($rikoLocalProperties, $rikoProperties, [Text.UTF8Encoding]::new($false))

$rikoSaved = @{}
foreach ($rikoKey in @('JAVA_HOME', 'PATH', 'GOTOOLCHAIN', 'GOMODCACHE', 'ANDROID_HOME', 'GOPROXY')) {
    $rikoSaved[$rikoKey] = [Environment]::GetEnvironmentVariable($rikoKey, 'Process')
}
Push-Location $rikoRoot
try {
    $env:JAVA_HOME = $rikoJava
    $env:PATH = "$rikoJava/bin;$rikoGo;$env:PATH"
    $env:GOTOOLCHAIN = 'local'
    $env:GOMODCACHE = Join-Path $rikoRoot '.local/go-mod-cache'
    $env:ANDROID_HOME = $Sdk
    $rikoJavaVersion = & "$rikoJava/bin/java.exe" -version 2>&1
    if ($rikoJavaVersion[0] -notmatch ('"' + [regex]::Escape($rikoLock.jdk.version) + '"')) { throw 'Unexpected JDK version.' }
    $rikoGoVersion = & "$rikoGo/go.exe" version
    if ($rikoGoVersion -ne "go version go$($rikoLock.go.version) windows/amd64") { throw 'Unexpected Go version.' }
    $rikoArguments = @($Tasks) + @('--no-daemon', '--console=plain')
    if ($Offline) { $rikoArguments += '--offline'; $env:GOPROXY = 'off' }
    & './gradlew.bat' @rikoArguments
    if ($LASTEXITCODE -ne 0) { throw "Android Gradle task failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
    foreach ($rikoKey in $rikoSaved.Keys) { [Environment]::SetEnvironmentVariable($rikoKey, $rikoSaved[$rikoKey], 'Process') }
}

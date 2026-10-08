param()
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$rikoRoot = Split-Path $PSScriptRoot -Parent
$rikoLock = Get-Content (Join-Path $rikoRoot 'gradle/toolchains.lock.json') -Raw | ConvertFrom-Json
$rikoDownloads = Join-Path $rikoRoot '.local/downloads'
$rikoTools = Join-Path $rikoRoot '.local/toolchains'
New-Item -ItemType Directory -Force $rikoDownloads, $rikoTools | Out-Null

foreach ($rikoTool in @($rikoLock.jdk, $rikoLock.go)) {
    $rikoArchive = Join-Path $rikoDownloads $rikoTool.archive
    if (-not (Test-Path -LiteralPath $rikoArchive)) {
        $rikoPartial = "$rikoArchive.part"
        try {
            Invoke-WebRequest -Uri $rikoTool.url -OutFile $rikoPartial
            if ((Get-FileHash -LiteralPath $rikoPartial).Hash.ToLowerInvariant() -ne $rikoTool.sha256) {
                throw "Toolchain download checksum mismatch: $($rikoTool.archive)"
            }
            Move-Item -LiteralPath $rikoPartial -Destination $rikoArchive
        } finally {
            if (Test-Path -LiteralPath $rikoPartial) { Remove-Item -LiteralPath $rikoPartial }
        }
    }
    if ((Get-FileHash -LiteralPath $rikoArchive).Hash.ToLowerInvariant() -ne $rikoTool.sha256) {
        throw "Toolchain archive checksum mismatch: $($rikoTool.archive)"
    }
    if (-not (Test-Path -LiteralPath (Join-Path $rikoTools "$($rikoTool.directory)/bin"))) {
        Expand-Archive -LiteralPath $rikoArchive -DestinationPath $rikoTools
    }
}

$rikoGoRoot = Join-Path $rikoTools $rikoLock.go.directory
foreach ($rikoPatch in $rikoLock.go.patches) {
    $rikoPatchFile = Join-Path $rikoRoot $rikoPatch.file
    $rikoTarget = Join-Path $rikoGoRoot $rikoPatch.target
    if ((Get-FileHash -LiteralPath $rikoPatchFile).Hash.ToLowerInvariant() -ne $rikoPatch.sha256) {
        throw "Go patch checksum mismatch: $($rikoPatch.file)"
    }
    if ((Get-FileHash -LiteralPath $rikoTarget).Hash.ToLowerInvariant() -eq $rikoPatch.patchedSha256) { continue }
    & git -C $rikoGoRoot apply --check $rikoPatchFile
    if ($LASTEXITCODE -ne 0) { throw "Go patch does not apply: $($rikoPatch.file)" }
    & git -C $rikoGoRoot apply $rikoPatchFile
    if ($LASTEXITCODE -ne 0) { throw "Go patch failed: $($rikoPatch.file)" }
    if ((Get-FileHash -LiteralPath $rikoTarget).Hash.ToLowerInvariant() -ne $rikoPatch.patchedSha256) {
        throw "Patched Go source checksum mismatch: $($rikoPatch.target)"
    }
}
Write-Output 'Verified project-local JDK, Go archives and Android Go patches.'

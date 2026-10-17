param([switch]$Offline)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:GOCACHE = Join-Path (Get-Location) '.cache/go-build'
$env:GOMODCACHE = Join-Path (Get-Location) '.cache/go-mod'
$artifacts = Get-Content artifacts/baseline.lock.json -Raw | ConvertFrom-Json
New-Item -ItemType Directory -Force models,runtime,bin | Out-Null
function Fetch-Artifact($Artifact, $Path) {
    if (!(Test-Path -LiteralPath $Path)) {
        if ($Offline) { throw "Missing offline artifact: $Path" }
        & curl.exe -L --fail --retry 2 -o $Path $Artifact.url
        if ($LASTEXITCODE -ne 0) { throw "Download failed: $Path" }
    }
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Artifact.sha256) {
        throw "Checksum mismatch: $Path. Remove the incomplete file and rerun setup."
    }
}
Fetch-Artifact $artifacts.engine 'runtime/llama-vulkan.zip'
Fetch-Artifact $artifacts.model ('models/' + $artifacts.model.filename)
Expand-Archive -LiteralPath runtime/llama-vulkan.zip -DestinationPath runtime/llama -Force
if ($Offline) { $env:GOPROXY = 'off'; $env:GOSUMDB = 'off'; $env:GOTOOLCHAIN = 'local' }
& go mod download
if ($LASTEXITCODE -ne 0) { throw 'Go dependency setup failed' }
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Contract tests failed' }
& go build -buildvcs=false -o bin/bots.exe ./cmd/bots
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
& bin/bots.exe diagnostics
if ($LASTEXITCODE -ne 0) { throw 'Diagnostics failed' }
Write-Output 'Setup complete. Register bots/extract and bots/workspace-qa, then run eval.'

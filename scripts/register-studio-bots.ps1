# Registers the bots Bot Studio uses and skips any that are already installed.
param([string[]]$Bots = @('manager', 'writer', 'landing-page', 'code-writer', 'math-solver', 'qa-assistant'))
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
if (!(Test-Path -LiteralPath bin/bots.exe)) { throw 'Run scripts/setup.ps1 first.' }
$installed = @((& bin/bots.exe bot list | ConvertFrom-Json) | ForEach-Object { $_.id })
foreach ($id in $Bots) {
    if ($installed -contains $id) { Write-Output "already registered: $id"; continue }
    & bin/bots.exe bot register "bots/$id" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Registration failed: $id" }
    Write-Output "registered: $id"
}

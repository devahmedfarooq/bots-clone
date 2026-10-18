param([string]$Config = 'framework.json', [int]$Port = 8080, [int]$Queue = 8)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
if (!(Test-Path -LiteralPath bin/bots.exe)) { throw 'Run scripts/setup.ps1 first.' }
if (!(Test-Path -LiteralPath $Config)) { throw 'Framework configuration is missing.' }
& bin/bots.exe -config $Config serve -listen "127.0.0.1:$Port" -queue $Queue
if ($LASTEXITCODE -ne 0) { throw "Service exited with code $LASTEXITCODE" }

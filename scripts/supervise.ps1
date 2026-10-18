param([string]$Config = 'framework.json', [int]$Port = 8080, [int]$Queue = 8)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
New-Item -ItemType Directory -Force .local | Out-Null
$configPath = (Resolve-Path -LiteralPath $Config).Path
$framework = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$engineFile = $framework.engine.binary
if (![System.IO.Path]::IsPathRooted($engineFile)) { $engineFile = Join-Path (Split-Path $configPath -Parent) $engineFile }
$enginePath = [System.IO.Path]::GetFullPath($engineFile)
$exe = (Resolve-Path -LiteralPath bin/bots.exe).Path
$arguments = @('-config', ('"' + $configPath + '"'), 'serve', '-listen', "127.0.0.1:$Port", '-queue', $Queue)
$delay = 2
while ($true) {
    $started = Get-Date
    $serviceProcess = $null
    try {
        $serviceProcess = Start-Process -FilePath $exe -ArgumentList $arguments -WindowStyle Hidden -PassThru -RedirectStandardOutput .local/service.stdout.log -RedirectStandardError .local/service.stderr.log
        $null = $serviceProcess.Handle
        $serviceProcess.WaitForExit()
        if ($serviceProcess.ExitCode -eq 0) { break }
        throw "Service exited with code $($serviceProcess.ExitCode)"
    } catch {
        $entry = "$(Get-Date -Format o) Service stopped: $_. Restarting in $delay seconds."
        Add-Content -LiteralPath .local/supervisor.log -Value $entry
        Write-Warning $entry
    } finally {
        if ($serviceProcess -and $serviceProcess.HasExited) {
            Get-CimInstance Win32_Process -Filter "ParentProcessId=$($serviceProcess.Id)" -ErrorAction SilentlyContinue |
                Where-Object { $_.ExecutablePath -and [System.IO.Path]::GetFullPath($_.ExecutablePath) -eq $enginePath } |
                ForEach-Object { Stop-Process -Id $_.ProcessId -ErrorAction SilentlyContinue }
        }
    }
    Start-Sleep -Seconds $delay
    if (((Get-Date) - $started).TotalMinutes -ge 5) { $delay = 2 } else { $delay = [Math]::Min(60, $delay * 2) }
}

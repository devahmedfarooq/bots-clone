param([string]$TaskName = 'Local Bots', [switch]$Remove)
$ErrorActionPreference = 'Stop'
if ($Remove) {
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
    return
}
$supervisor = Join-Path $PSScriptRoot 'supervise.ps1'
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$supervisor`"" -WorkingDirectory (Split-Path $PSScriptRoot -Parent)
$user = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $user
$principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings | Out-Null
Write-Output "Registered $TaskName for login. Start it with: Start-ScheduledTask -TaskName '$TaskName'"

param([ValidateSet('start','run','build','stop','restart','logs','state','pair','check','dot')][string]$Action='start')
$ErrorActionPreference='Stop'
$script=Join-Path $PSScriptRoot 'server/run.sh'
$linuxPath=(& wsl.exe wslpath -a $script).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Unable to resolve WSL path.' }
& wsl.exe bash $linuxPath $Action
if ($LASTEXITCODE -ne 0) { throw "Phone Connect action failed: $Action" }

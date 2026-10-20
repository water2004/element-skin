param([int]$Port = 5174)
$ErrorActionPreference = 'Stop'
$node = Get-Command node -ErrorAction Stop
& $node.Source (Join-Path $PSScriptRoot 'server.mjs') --port $Port
exit $LASTEXITCODE

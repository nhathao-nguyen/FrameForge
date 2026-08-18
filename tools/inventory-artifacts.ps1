[CmdletBinding()]
param([switch]$Strict)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($env:NH_MEDIA_DATABASE_URL)) { throw 'NH_MEDIA_DATABASE_URL is required.' }
$old = $env:NH_MEDIA_INVENTORY_STRICT
if ($Strict) { $env:NH_MEDIA_INVENTORY_STRICT = '1' }
try {
    Push-Location $repoRoot
    try { & go run .\services\api\cmd\artifact-inventory; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE } }
    finally { Pop-Location }
} finally { $env:NH_MEDIA_INVENTORY_STRICT = $old }

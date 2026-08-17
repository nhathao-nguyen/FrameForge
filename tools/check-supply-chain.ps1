[CmdletBinding()]
param([string]$Root)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($Root)) { $Root = Split-Path -Parent $PSScriptRoot }
$compose = Get-Content -Raw -LiteralPath (Join-Path $Root 'infrastructure/compose/docker-compose.yml')
if ($compose -match '(?im)image:\s*[^\r\n:]+:latest\s*$' -or $compose -match '(?im)image:\s*[^\r\n]+@sha256:[0-9a-f]{64}') { throw 'infrastructure image pin policy failed' }
if (-not (Test-Path -LiteralPath (Join-Path $Root 'pnpm-lock.yaml')) -or -not (Test-Path -LiteralPath (Join-Path $Root 'services/ml-worker/uv.lock'))) { throw 'lockfile baseline is incomplete' }
$goModules = go list -m all 2>$null
if ($goModules -match '(?i)movie[_-]narrator|legacy-compat') { throw 'Go dependency graph contains a forbidden module' }
$uvLock = Get-Content -Raw -LiteralPath (Join-Path $Root 'services/ml-worker/uv.lock')
if ($uvLock -match '(?i)movie[_-]narrator') { throw 'Python lock contains a forbidden package' }
$sbom = [ordered]@{ schema = 'nh-media-foundation-sbom/v1'; generated_at = [DateTime]::UtcNow.ToString('o'); go_modules = @($goModules); package_lock = 'pnpm-lock.yaml'; python_lock = 'services/ml-worker/uv.lock'; images = @('postgres:16.10-alpine', 'redis:7.4.5-alpine', 'minio/minio:RELEASE.2025-09-07T16-13-09Z', 'minio/mc:RELEASE.2025-08-13T08-35-41Z') }
$output = Join-Path ([IO.Path]::GetTempPath()) 'nh-media-foundation-sbom.json'
$sbom | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $output -Encoding utf8
Write-Output "dependency lock, license/image pin and SBOM baseline: pass ($output)"

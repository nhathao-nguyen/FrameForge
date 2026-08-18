[CmdletBinding()]
param(
    [string]$OutputDirectory = '',
    [switch]$SkipInventory
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) { $OutputDirectory = Join-Path (Split-Path -Parent $repoRoot) 'NH-Media-backups' }
$resolvedOutput = [IO.Path]::GetFullPath($OutputDirectory)
$resolvedRepo = [IO.Path]::GetFullPath($repoRoot).TrimEnd('\') + '\'
if ($resolvedOutput.StartsWith($resolvedRepo, [StringComparison]::OrdinalIgnoreCase)) { throw 'Backup output must be outside the repository tree.' }
if ([string]::IsNullOrWhiteSpace($env:NH_MEDIA_DATABASE_URL)) { throw 'NH_MEDIA_DATABASE_URL is required.' }
$pgDump = Get-Command pg_dump.exe -ErrorAction SilentlyContinue
if ($null -eq $pgDump) { $pgDump = Get-Command pg_dump -ErrorAction SilentlyContinue }
if ($null -eq $pgDump) { throw 'pg_dump is required; install the pinned PostgreSQL client on the Windows owner machine.' }
New-Item -ItemType Directory -Path $resolvedOutput -Force | Out-Null
$stamp = Get-Date -AsUTC -Format 'yyyyMMddTHHmmssZ'
$backup = Join-Path $resolvedOutput ('nh-media-' + $stamp + '.dump')
$manifest = Join-Path $resolvedOutput ('nh-media-' + $stamp + '.manifest.json')
if (Test-Path -LiteralPath $backup) { throw 'Refusing to overwrite an existing backup.' }
& $pgDump.Source '--format=custom' '--no-owner' '--no-acl' '--file' $backup '--dbname' $env:NH_MEDIA_DATABASE_URL
if ($LASTEXITCODE -ne 0) { throw 'pg_dump failed.' }
$hash = (Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash.ToLowerInvariant()
$record = [ordered]@{ schema_version = 'nh-media/backup-manifest/v1'; created_at = (Get-Date).ToUniversalTime().ToString('o'); database_backup = [ordered]@{ file = $backup; sha256 = $hash; size_bytes = (Get-Item -LiteralPath $backup).Length }; object_inventory = $null; restore_policy = 'restore only into a separately created empty target database; never overwrite source' }
if (-not $SkipInventory) {
    $record.object_inventory = [ordered]@{ command = 'go run ./services/api/cmd/artifact-inventory'; status = 'run separately with the same environment; output remains outside the repository' }
}
$record | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $manifest -Encoding UTF8 -NoNewline
Get-Content -LiteralPath $manifest

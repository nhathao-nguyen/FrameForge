[CmdletBinding()]
param(
    [string]$Repository = 'https://github.com/zcbacxc/movie-narrator.git',
    [string]$OutputPath = ''
)

$ErrorActionPreference = 'Stop'
$git = Get-Command git.exe -ErrorAction SilentlyContinue
if ($null -eq $git) { throw 'git is required.' }
$head = (& $git.Source 'ls-remote' $Repository 'HEAD' 'refs/heads/main' | Select-Object -First 1)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($head)) { throw 'Unable to refresh upstream ref metadata.' }
$commit = ($head -split "`t")[0]
if ($commit -notmatch '^[0-9a-f]{40}$') { throw 'Upstream ref was not a full commit hash.' }
$result = [ordered]@{ schema_version = 'nh-media/upstream-refresh/v1'; repository = $Repository; refreshed_at = (Get-Date).ToUniversalTime().ToString('o'); head = $commit; source_checkout = $false; product_dependency = $false; license_identifier = 'AGPL-3.0-or-later' }
if (-not [string]::IsNullOrWhiteSpace($OutputPath)) { $result | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $OutputPath -Encoding UTF8 -NoNewline }
$result | ConvertTo-Json -Depth 5

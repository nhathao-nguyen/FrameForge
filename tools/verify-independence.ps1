[CmdletBinding()]
param(
    [string]$Root,
    [string[]]$ScanPath
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($Root)) { $Root = Split-Path -Parent $PSScriptRoot }
if (-not $ScanPath) {
    $ScanPath = @('apps', 'services', 'packages', 'infrastructure', 'tests/contract', 'tests/integration', 'tests/e2e', 'tests/unit', 'go.mod', 'package.json', 'pnpm-lock.yaml', 'pnpm-workspace.yaml')
}
$patterns = @('movie_narrator', 'movie-narrator', 'legacy-compat', 'zcbacxc/movie')
$matches = @()
foreach ($relative in $ScanPath) {
    $target = Join-Path $Root $relative
    if (-not (Test-Path -LiteralPath $target)) { continue }
    $found = rg --hidden --no-ignore-vcs -n -i ($patterns -join '|') --glob '!node_modules/**' --glob '!target/**' --glob '!.venv/**' -- $target 2>$null
    if ($LASTEXITCODE -eq 0) { $matches += $found }
}
if ($matches.Count -gt 0) { Write-Output ($matches -join [Environment]::NewLine); throw 'independence scan failed' }
Write-Output 'independence scan: pass (no upstream source/package/image/import or legacy compatibility in product graph)'

[CmdletBinding()]
param(
    [ValidateSet('validate', 'package', 'rollback')]
    [string]$Action = 'validate',
    [string]$Version = '0.1.0-gate-f',
    [string]$OutputRoot = (Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'NH-Media\staging'),
    [string]$TargetVersion = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$versionRoot = Join-Path $OutputRoot $Version

function Relative([string]$Path) {
    $baseUri = [Uri]::new(($repoRoot.TrimEnd('\') + '\'))
    $fileUri = [Uri]::new($Path)
    return [Uri]::UnescapeDataString($baseUri.MakeRelativeUri($fileUri).ToString()).Replace('\', '/')
}

function SourceFiles {
    $roots = @((Join-Path $repoRoot 'apps/desktop'), (Join-Path $repoRoot 'apps/web/src'), (Join-Path $repoRoot 'packages/sdk/src'))
    return Get-ChildItem -LiteralPath $roots -Recurse -File | Where-Object { $_.FullName -notmatch '\\(target|node_modules|\.next)\\' }
}

function AssertBoundary {
    $forbidden = @('movie_narrator', 'NH_MEDIA_DATABASE_URL', 'REDIS_PASSWORD', 'MINIO_ROOT_PASSWORD', 'NH_API_SESSION_SECRET', 'TAURI_SIGNING_PRIVATE_KEY')
    foreach ($file in (SourceFiles)) {
        $content = Get-Content -LiteralPath $file.FullName -Raw
        foreach ($term in $forbidden) {
            if ($content.IndexOf($term, [StringComparison]::OrdinalIgnoreCase) -ge 0) {
                throw ('desktop source boundary contains forbidden term in ' + (Relative $file.FullName))
            }
        }
    }
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot 'apps/desktop/src-tauri/capabilities/default.json'))) {
        throw 'Tauri capability manifest is missing.'
    }
}

function BuildManifest {
    New-Item -ItemType Directory -Force -Path $versionRoot | Out-Null
    $entries = @()
    foreach ($file in (SourceFiles | Sort-Object FullName)) {
        $entries += [pscustomobject]@{ path = Relative $file.FullName; sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant(); size_bytes = $file.Length }
    }
    $signingKeyPresent = -not [string]::IsNullOrWhiteSpace($env:TAURI_SIGNING_PRIVATE_KEY)
    [pscustomobject]@{
        product = 'NH-Media'
        version = $Version
        package_kind = 'staging-source-boundary'
        signing = if ($signingKeyPresent) { 'external-tauri-key-configured' } else { 'external-signing-key-required' }
        rollback = 'pointer-update-only; prior version directories are retained'
        contains_server_compute = $false
        contains_data_services = $false
        contains_provider_secrets = $false
        files = $entries
    } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $versionRoot 'package-manifest.json') -Encoding UTF8
}

switch ($Action) {
    'validate' {
        AssertBoundary
        BuildManifest
        Write-Output ('Desktop staging boundary validated: ' + (Join-Path $versionRoot 'package-manifest.json'))
        if ([string]::IsNullOrWhiteSpace($env:TAURI_SIGNING_PRIVATE_KEY)) {
            Write-Warning 'No external TAURI_SIGNING_PRIVATE_KEY was provided; this is unsigned staging evidence, not a production signing PASS.'
        }
    }
    'package' {
        AssertBoundary
        $bundle = Join-Path $repoRoot 'apps/desktop/src-tauri/target/release/bundle'
        if (-not (Test-Path -LiteralPath $bundle)) {
            throw 'No Tauri release bundle exists. Run the approved Tauri build with an external signing key first.'
        }
        BuildManifest
        Copy-Item -LiteralPath $bundle -Destination (Join-Path $versionRoot 'bundle') -Recurse -Force
        Write-Output ('Staging package copied: ' + $versionRoot)
    }
    'rollback' {
        if ([string]::IsNullOrWhiteSpace($TargetVersion)) { throw 'TargetVersion is required for rollback.' }
        $target = Join-Path $OutputRoot $TargetVersion
        if (-not (Test-Path -LiteralPath (Join-Path $target 'package-manifest.json'))) { throw 'Rollback target has no validated package manifest.' }
        New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
        [pscustomobject]@{ active_version = $TargetVersion; changed_at = (Get-Date).ToUniversalTime().ToString('o') } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $OutputRoot 'current.json') -Encoding UTF8
        Write-Output ('Desktop staging pointer rolled back to ' + $TargetVersion)
    }
}

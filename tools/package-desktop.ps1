[CmdletBinding()]
param(
    [ValidateSet('validate', 'package', 'verify', 'rollback')]
    [string]$Action = 'validate',
    [string]$Version = '0.1.0-gate-f',
    [string]$OutputRoot = (Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'NH-Media\staging'),
    [string]$TargetVersion = '',
    [string]$LanServerIp = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$versionRoot = Join-Path $OutputRoot $Version
$manifestPath = Join-Path $versionRoot 'package-manifest.json'
$signatureVerifier = Join-Path $repoRoot 'tools\verify-tauri-signature.mjs'

function Resolve-PrivateLanIPv4([string]$Value) {
    if ([string]::IsNullOrWhiteSpace($Value)) { return '' }
    $parsed = $null
    if (-not [Net.IPAddress]::TryParse($Value, [ref]$parsed) -or $parsed.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork -or [Net.IPAddress]::IsLoopback($parsed)) {
        throw 'LanServerIp must be an IPv4 address on the private LAN.'
    }
    $octets = $parsed.GetAddressBytes()
    $privateLan = $octets[0] -eq 10 -or
        ($octets[0] -eq 172 -and $octets[1] -ge 16 -and $octets[1] -le 31) -or
        ($octets[0] -eq 192 -and $octets[1] -eq 168)
    if (-not $privateLan) { throw 'LanServerIp must be an RFC1918 private-LAN IPv4 address.' }
    return $parsed.IPAddressToString
}

$resolvedLanServerIp = Resolve-PrivateLanIPv4 $LanServerIp
$desktopApiOrigin = if ($resolvedLanServerIp -ne '') { 'http://' + $resolvedLanServerIp + ':8080' } else { '' }

function Relative([string]$Path) {
    $baseUri = [Uri]::new(($repoRoot.TrimEnd('\') + '\'))
    $fileUri = [Uri]::new($Path)
    return [Uri]::UnescapeDataString($baseUri.MakeRelativeUri($fileUri).ToString()).Replace('\', '/')
}

function SourceFiles {
    $roots = @((Join-Path $repoRoot 'apps/desktop'), (Join-Path $repoRoot 'apps/web/src'), (Join-Path $repoRoot 'packages/sdk/src'))
    return Get-ChildItem -LiteralPath $roots -Recurse -File | Where-Object { $_.FullName -notmatch '\\(target|node_modules|\.next|dist)\\' }
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
    $capabilityPath = Join-Path $repoRoot 'apps/desktop/src-tauri/capabilities/default.json'
    if (-not (Test-Path -LiteralPath $capabilityPath)) { throw 'Tauri capability manifest is missing.' }
    $capability = Get-Content -Raw -LiteralPath $capabilityPath
    foreach ($permission in @('fs:', 'shell:', 'process:', 'sql:', 'http:')) {
        if ($capability.IndexOf($permission, [StringComparison]::OrdinalIgnoreCase) -ge 0) { throw ('forbidden desktop permission is present: ' + $permission) }
    }
    $configPath = Join-Path $repoRoot 'apps/desktop/src-tauri/tauri.conf.json'
    $config = Get-Content -Raw -LiteralPath $configPath | ConvertFrom-Json
    if (-not $config.bundle.active) { throw 'Tauri bundling is disabled; the release pipeline cannot produce T434 artifacts.' }
    if (@($config.bundle.targets) -notcontains 'nsis') { throw 'Windows NSIS is not enabled in the Tauri bundle targets.' }
}

function Get-ManifestEntries([string]$Root, [string]$Prefix = '') {
    if (-not (Test-Path -LiteralPath $Root)) { throw ('Package root does not exist: ' + $Root) }
    $entries = @()
    foreach ($file in (Get-ChildItem -LiteralPath $Root -Recurse -File | Sort-Object FullName)) {
        $relative = [Uri]::UnescapeDataString(([Uri]::new(($Root.TrimEnd('\') + '\'))).MakeRelativeUri([Uri]::new($file.FullName)).ToString()).Replace('\', '/')
        $path = if ($Prefix -eq '') { $relative } else { $Prefix.TrimEnd('/') + '/' + $relative }
        $entries += [pscustomobject]@{ path = $path; sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant(); size_bytes = $file.Length }
    }
    return $entries
}

function Get-SourceManifestEntries {
    $entries = @()
    foreach ($file in (SourceFiles | Sort-Object FullName)) {
        $entries += [pscustomobject]@{
            path = 'source/' + (Relative $file.FullName)
            sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant()
            size_bytes = $file.Length
        }
    }
    return $entries
}

function AssertNoSecretFiles([string]$Root) {
    $secretNames = Get-ChildItem -LiteralPath $Root -Recurse -File | Where-Object { $_.Name -match '(?i)(^|[._-])(private|secret|credential|token|password)([._-]|$)|\.pem$|\.key$' }
    if ($secretNames) { throw ('package contains a secret-looking file name: ' + $secretNames[0].FullName) }
}

function EnsureExternalSigningKey {
    $keyPath = $env:TAURI_SIGNING_PRIVATE_KEY_PATH
    $keyValue = $env:TAURI_SIGNING_PRIVATE_KEY
    if ([string]::IsNullOrWhiteSpace($keyPath) -and [string]::IsNullOrWhiteSpace($keyValue)) {
        throw 'External owner input required: set TAURI_SIGNING_PRIVATE_KEY or TAURI_SIGNING_PRIVATE_KEY_PATH in the current process only, then rerun package. Never commit or paste the private key into the repository.'
    }
    if (-not [string]::IsNullOrWhiteSpace($keyPath) -and -not (Test-Path -LiteralPath $keyPath -PathType Leaf)) {
        throw 'External owner input required: TAURI_SIGNING_PRIVATE_KEY_PATH does not point to a readable private key file.'
    }
    if ([string]::IsNullOrWhiteSpace($env:TAURI_SIGNING_PRIVATE_KEY_PASSWORD)) {
        throw 'External owner input required: TAURI_SIGNING_PRIVATE_KEY_PASSWORD is missing. Set it in the current process only, then rerun package. Never commit or paste the signing password into the repository.'
    }
}

function GetSigningKeyArguments {
    EnsureExternalSigningKey
    if (-not [string]::IsNullOrWhiteSpace($env:TAURI_SIGNING_PRIVATE_KEY_PATH)) {
        return @('--private-key-path', $env:TAURI_SIGNING_PRIVATE_KEY_PATH)
    }
    if (Test-Path -LiteralPath $env:TAURI_SIGNING_PRIVATE_KEY -PathType Leaf) {
        return @('--private-key-path', $env:TAURI_SIGNING_PRIVATE_KEY)
    }
    return @('--private-key', $env:TAURI_SIGNING_PRIVATE_KEY)
}

function EnsureExternalPublicKey {
    if ([string]::IsNullOrWhiteSpace($env:TAURI_SIGNING_PUBLIC_KEY_PATH)) {
        throw 'External owner input required: set TAURI_SIGNING_PUBLIC_KEY_PATH in the current process only, then rerun package.'
    }
    if (-not (Test-Path -LiteralPath $env:TAURI_SIGNING_PUBLIC_KEY_PATH -PathType Leaf)) {
        throw 'External owner input required: TAURI_SIGNING_PUBLIC_KEY_PATH does not point to a readable public key file.'
    }
    if (-not (Test-Path -LiteralPath $signatureVerifier -PathType Leaf)) {
        throw 'Tauri signature verifier tool is missing.'
    }
}

function GetPublicKeySha256 {
    EnsureExternalPublicKey
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $env:TAURI_SIGNING_PUBLIC_KEY_PATH).Hash.ToLowerInvariant()
}

function VerifySignatures([string]$BundleRoot) {
    EnsureExternalPublicKey
    $payloads = @(Get-ChildItem -LiteralPath $BundleRoot -Recurse -File | Where-Object { $_.Extension -in @('.exe', '.msi', '.zip') } | Sort-Object FullName)
    if ($payloads.Count -eq 0) { throw 'No Windows bundle payload (.exe/.msi/.zip) exists for signature verification.' }
    foreach ($payload in $payloads) {
        $signaturePath = $payload.FullName + '.sig'
        if (-not (Test-Path -LiteralPath $signaturePath -PathType Leaf)) { throw ('Signature sidecar is missing: ' + $signaturePath) }
        & node $signatureVerifier --public-key $env:TAURI_SIGNING_PUBLIC_KEY_PATH --file $payload.FullName --signature $signaturePath | Out-Null
        if ($LASTEXITCODE -ne 0) { throw ('Tauri signature verification failed for ' + $payload.FullName) }
    }
}

function SignBundle([string]$BundleRoot) {
    EnsureExternalSigningKey
    $payloads = @(Get-ChildItem -LiteralPath $BundleRoot -Recurse -File | Where-Object { $_.Extension -in @('.exe', '.msi', '.zip') } | Sort-Object FullName)
    if ($payloads.Count -eq 0) { throw 'No Windows bundle payload (.exe/.msi/.zip) exists to sign.' }
    foreach ($payload in $payloads) {
        $signingArgs = @('tauri', 'signer', 'sign') + @(GetSigningKeyArguments) + @($payload.FullName)
        & cargo @signingArgs | Out-Null
        if ($LASTEXITCODE -ne 0) { throw ('Tauri signing failed for ' + $payload.FullName) }
    }
    $missing = @($payloads | Where-Object { -not (Test-Path -LiteralPath ($_.FullName + '.sig')) })
    if ($missing.Count -gt 0) { throw ('Tauri signer did not produce signature sidecars for: ' + (($missing | ForEach-Object FullName) -join ', ')) }
    VerifySignatures $BundleRoot
}

function BuildDesktopBundle {
    $hadApiUrl = Test-Path Env:NEXT_PUBLIC_NH_MEDIA_API_URL
    $previousApiUrl = $env:NEXT_PUBLIC_NH_MEDIA_API_URL
    $overlayPath = ''
    Push-Location $repoRoot
    try {
        $arguments = @('tauri', 'build', '--config', 'apps/desktop/src-tauri/tauri.conf.json')
        if ($desktopApiOrigin -ne '') {
            $env:NEXT_PUBLIC_NH_MEDIA_API_URL = $desktopApiOrigin
            $csp = "default-src 'self'; connect-src 'self' http://127.0.0.1:8080 http://localhost:8080 $desktopApiOrigin; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'"
            $overlay = [ordered]@{ app = [ordered]@{ security = [ordered]@{ csp = $csp } } } | ConvertTo-Json -Depth 5 -Compress
            $overlayPath = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-tauri-lan-' + [Guid]::NewGuid().ToString('N') + '.json')
            $overlay | Set-Content -LiteralPath $overlayPath -Encoding UTF8
            $arguments += @('--config', $overlayPath)
        }
        $arguments += @('--no-sign', '--ci')
        & cargo @arguments
        if ($LASTEXITCODE -ne 0) { throw 'Tauri desktop build failed; no package/sign/verify/rollback step may continue.' }
    } finally {
        Pop-Location
        if ($overlayPath -ne '' -and (Test-Path -LiteralPath $overlayPath)) { Remove-Item -LiteralPath $overlayPath -Force }
        if ($hadApiUrl) { $env:NEXT_PUBLIC_NH_MEDIA_API_URL = $previousApiUrl } else { Remove-Item Env:NEXT_PUBLIC_NH_MEDIA_API_URL -ErrorAction SilentlyContinue }
    }
}

function BuildManifest([string]$PackageKind, [string]$BundleRoot = '') {
    New-Item -ItemType Directory -Force -Path $versionRoot | Out-Null
    $entries = @(Get-SourceManifestEntries)
    if ($BundleRoot -ne '') { $entries += @(Get-ManifestEntries $BundleRoot 'bundle') }
    $signing = if ($BundleRoot -ne '') { 'tauri-signer-signature-sidecars-present' } else { 'unsigned-development-staging' }
    $publicKeySha256 = if ($BundleRoot -ne '') { GetPublicKeySha256 } else { '' }
    [ordered]@{
        schema_version = 'nh-media/desktop-package-manifest/v2'
        product = 'NH-Media'
        version = $Version
        package_kind = $PackageKind
        signing = $signing
        rollback = 'pointer-update-only; prior version directories are retained; manifest hashes are rechecked before pointer update'
        contains_server_compute = $false
        contains_data_services = $false
        contains_provider_secrets = $false
        deployment_profile = if ($desktopApiOrigin -ne '') { 'lan' } else { 'loopback' }
        api_origin = $desktopApiOrigin
        csp = if ($desktopApiOrigin -ne '') { 'loopback plus exact LAN API origin ' + $desktopApiOrigin } else { 'loopback-only baseline; LAN builds must use an exact external origin overlay' }
        public_key_sha256 = $publicKeySha256
        files = @($entries | Sort-Object path)
    } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $manifestPath -Encoding UTF8
}

function VerifyPackage([string]$Root) {
    $manifestFile = Join-Path $Root 'package-manifest.json'
    if (-not (Test-Path -LiteralPath $manifestFile)) { throw ('Package manifest is missing: ' + $manifestFile) }
    $manifest = Get-Content -Raw -LiteralPath $manifestFile | ConvertFrom-Json
    if ($manifest.signing -ne 'tauri-signer-signature-sidecars-present') { throw 'Only a signed Tauri staging package may pass package verification or rollback.' }
    if ($desktopApiOrigin -ne '' -and [string]$manifest.api_origin -ne $desktopApiOrigin) { throw ('Package LAN API origin does not match ' + $desktopApiOrigin) }
    foreach ($entry in @($manifest.files)) {
        $entryPath = [string]$entry.path
        $file = if ($entryPath.StartsWith('source/')) { Join-Path $repoRoot $entryPath.Substring(7).Replace('/', '\') } else { Join-Path $Root $entryPath }
        if (-not (Test-Path -LiteralPath $file)) { throw ('Manifest file is missing: ' + $entry.path) }
        $item = Get-Item -LiteralPath $file
        if ([int64]$item.Length -ne [int64]$entry.size_bytes) { throw ('Manifest size mismatch: ' + $entry.path) }
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $file).Hash.ToLowerInvariant()
        if ($hash -ne [string]$entry.sha256) { throw ('Manifest checksum mismatch: ' + $entry.path) }
    }
    AssertNoSecretFiles $Root
    if ($manifest.signing -eq 'tauri-signer-signature-sidecars-present') {
        if ([string]$manifest.public_key_sha256 -ne (GetPublicKeySha256)) { throw 'Package public-key fingerprint does not match the supplied external public key.' }
        VerifySignatures (Join-Path $Root 'bundle')
    }
    return $manifest
}

switch ($Action) {
    'validate' {
        AssertBoundary
        BuildManifest 'unsigned-development-staging'
        Write-Output ('Desktop development staging boundary validated: ' + $manifestPath)
        Write-Warning 'This manifest is unsigned development evidence and cannot satisfy the T434 signed-staging DoD.'
    }
    'package' {
        AssertBoundary
        EnsureExternalSigningKey
        BuildDesktopBundle
        $bundle = Join-Path $repoRoot 'apps/desktop/src-tauri/target/release/bundle'
        if (-not (Test-Path -LiteralPath $bundle)) {
            throw 'No Tauri release bundle exists. Run the approved cargo tauri build with the external key available to the signing step first.'
        }
        SignBundle $bundle
        $destination = Join-Path $versionRoot 'bundle'
        if (Test-Path -LiteralPath $destination) { Remove-Item -LiteralPath $destination -Recurse -Force }
        New-Item -ItemType Directory -Force -Path $destination | Out-Null
        Get-ChildItem -LiteralPath $bundle -Force | Copy-Item -Destination $destination -Recurse -Force
        AssertNoSecretFiles $destination
        BuildManifest 'signed-tauri-staging' $destination
        $null = VerifyPackage $versionRoot
        Write-Output ('Signed desktop staging package copied: ' + $versionRoot)
    }
    'verify' {
        AssertBoundary
        $null = VerifyPackage $versionRoot
        Write-Output ('Desktop package verification passed: ' + $versionRoot)
    }
    'rollback' {
        if ([string]::IsNullOrWhiteSpace($TargetVersion)) { throw 'TargetVersion is required for rollback.' }
        $target = Join-Path $OutputRoot $TargetVersion
        $null = VerifyPackage $target
        New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
        [ordered]@{
            schema_version = 'nh-media/desktop-active-pointer/v1'
            active_version = $TargetVersion
            manifest_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $target 'package-manifest.json')).Hash.ToLowerInvariant()
            changed_at = (Get-Date).ToUniversalTime().ToString('o')
        } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $OutputRoot 'current.json') -Encoding UTF8
        Write-Output ('Desktop staging pointer rolled back to signed version ' + $TargetVersion)
    }
}

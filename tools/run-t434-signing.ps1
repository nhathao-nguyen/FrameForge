[CmdletBinding()]
param(
    [string]$Version = 't434-signed',
    [string]$PrivateKeyPath = (Join-Path $env:USERPROFILE '.tauri\nh-media.key'),
    [string]$PnpmPath = '',
    [string]$PublicKeyPath = '',
    [string]$LanServerIp = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$packageScript = Join-Path $repoRoot 'tools\package-desktop.ps1'
$stagingRoot = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'NH-Media\staging'
$versionRoot = Join-Path $stagingRoot $Version
$manifestPath = Join-Path $versionRoot 'package-manifest.json'
$secure = $null
$ptr = [IntPtr]::Zero
$exitCode = 0

function Invoke-PackageAction([string]$Action, [string]$TargetVersion = '') {
    $arguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $packageScript, '-Action', $Action, '-Version', $Version)
    if (-not [string]::IsNullOrWhiteSpace($LanServerIp)) {
        $arguments += @('-LanServerIp', $LanServerIp)
    }
    if (-not [string]::IsNullOrWhiteSpace($TargetVersion)) {
        $arguments += @('-TargetVersion', $TargetVersion)
    }
    & powershell @arguments
    if ($LASTEXITCODE -ne 0) {
        throw ('T434 package action failed: ' + $Action)
    }
}

function Resolve-PnpmPath {
    if (-not [string]::IsNullOrWhiteSpace($PnpmPath)) {
        if (-not (Test-Path -LiteralPath $PnpmPath -PathType Leaf)) {
            throw ('pnpm.cmd was not found at the supplied path: ' + $PnpmPath)
        }
        return (Resolve-Path -LiteralPath $PnpmPath).Path
    }

    foreach ($commandName in @('pnpm.cmd', 'pnpm')) {
        $command = Get-Command $commandName -ErrorAction SilentlyContinue
        if ($null -ne $command -and (Test-Path -LiteralPath $command.Source -PathType Leaf)) {
            return $command.Source
        }
    }

    $candidates = @(
        (Join-Path $env:USERPROFILE '.cache\codex-runtimes\codex-primary-runtime\dependencies\bin\fallback\pnpm.cmd'),
        (Join-Path $env:USERPROFILE 'AppData\Local\pnpm\pnpm.cmd'),
        (Join-Path $env:USERPROFILE 'AppData\Roaming\npm\pnpm.cmd')
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    throw 'pnpm.cmd was not found. Install pnpm or rerun with -PnpmPath <full path to pnpm.cmd>.'
}

function Test-TamperRejection {
    $artifact = Get-ChildItem -LiteralPath (Join-Path $versionRoot 'bundle') -Recurse -File |
        Where-Object { $_.Extension -in @('.exe', '.msi', '.zip') } |
        Select-Object -First 1
    if ($null -eq $artifact) { throw 'No signed bundle payload was available for the tamper test.' }

    $temporaryRoot = Join-Path $env:TEMP ('nh-media-t434-tamper-' + [guid]::NewGuid().ToString('N'))
    $temporaryFile = Join-Path $temporaryRoot $artifact.Name
    $temporarySignature = $temporaryFile + '.sig'
    New-Item -ItemType Directory -Force -Path $temporaryRoot | Out-Null
    try {
        Copy-Item -LiteralPath $artifact.FullName -Destination $temporaryFile
        Copy-Item -LiteralPath ($artifact.FullName + '.sig') -Destination $temporarySignature
        $bytes = [IO.File]::ReadAllBytes($temporaryFile)
        $bytes[0] = $bytes[0] -bxor 1
        [IO.File]::WriteAllBytes($temporaryFile, $bytes)
        & node (Join-Path $repoRoot 'tools\verify-tauri-signature.mjs') --public-key $env:TAURI_SIGNING_PUBLIC_KEY_PATH --file $temporaryFile --signature $temporarySignature | Out-Null
        if ($LASTEXITCODE -eq 0) { throw 'Tampered artifact was unexpectedly accepted.' }
        Write-Output 'Tamper rejection PASS'
    }
    finally {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

try {
    if (-not (Test-Path -LiteralPath $PrivateKeyPath -PathType Leaf)) {
        throw ('Signing key file was not found: ' + $PrivateKeyPath)
    }
    if (-not (Test-Path -LiteralPath $packageScript -PathType Leaf)) {
        throw ('Package script was not found: ' + $packageScript)
    }

    Set-Location -LiteralPath $repoRoot
    $env:NH_MEDIA_PNPM_PATH = Resolve-PnpmPath
    Remove-Item Env:TAURI_SIGNING_PRIVATE_KEY -ErrorAction SilentlyContinue
    $env:TAURI_SIGNING_PRIVATE_KEY_PATH = (Resolve-Path -LiteralPath $PrivateKeyPath).Path
    if ([string]::IsNullOrWhiteSpace($PublicKeyPath)) { $PublicKeyPath = $PrivateKeyPath + '.pub' }
    if (-not (Test-Path -LiteralPath $PublicKeyPath -PathType Leaf)) {
        throw ('Public signing key file was not found: ' + $PublicKeyPath)
    }
    $env:TAURI_SIGNING_PUBLIC_KEY_PATH = (Resolve-Path -LiteralPath $PublicKeyPath).Path

    $secure = Read-Host 'Tauri signing password' -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    $env:TAURI_SIGNING_PRIVATE_KEY_PASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)

    $profileText = if ([string]::IsNullOrWhiteSpace($LanServerIp)) { 'loopback' } else { 'LAN server ' + $LanServerIp }
    Write-Output ('T434 signing run started for version ' + $Version + ' (' + $profileText + ')')
    Invoke-PackageAction 'validate'
    Invoke-PackageAction 'package'
    Invoke-PackageAction 'verify'
    Invoke-PackageAction 'rollback' $Version
    Test-TamperRejection

    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw ('Signed package manifest was not found: ' + $manifestPath)
    }
    $manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
    Write-Output ''
    Write-Output 'Signed bundle artifacts:'
    foreach ($entry in @($manifest.files | Where-Object { $_.path -like 'bundle/*' })) {
        Write-Output ('- ' + $entry.path + ' size=' + $entry.size_bytes + ' sha256=' + $entry.sha256)
    }
    Write-Output ''
    Write-Output ('T434 package, manifest verification and rollback completed: ' + $versionRoot)
}
catch {
    $exitCode = 1
    Write-Host ''
    Write-Host ('T434 FAILED: ' + $_.Exception.Message) -ForegroundColor Red
    Write-Host 'Signing key contents and password were not printed.' -ForegroundColor DarkGray
}
finally {
    if ($ptr -ne [IntPtr]::Zero) {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
    }
    if ($null -ne $secure) {
        $secure.Dispose()
    }
    Remove-Item Env:TAURI_SIGNING_PRIVATE_KEY_PATH -ErrorAction SilentlyContinue
    Remove-Item Env:TAURI_SIGNING_PRIVATE_KEY_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:TAURI_SIGNING_PRIVATE_KEY -ErrorAction SilentlyContinue
    Remove-Item Env:TAURI_SIGNING_PUBLIC_KEY_PATH -ErrorAction SilentlyContinue
    Remove-Item Env:NH_MEDIA_PNPM_PATH -ErrorAction SilentlyContinue
}

if ($exitCode -ne 0) {
    Read-Host 'Press Enter to close this window'
    exit $exitCode
}

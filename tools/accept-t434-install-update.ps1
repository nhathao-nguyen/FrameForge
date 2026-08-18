[CmdletBinding()]
param(
    [string]$Version = 't434-signed',
    [string]$NextVersion = 't434-signed-next',
    [string]$OutputRoot = (Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'NH-Media\staging'),
    [string]$InstallRoot = (Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) ('NH-Media\t434-install-test-' + [Guid]::NewGuid().ToString('N'))),
    [string]$PublicKeyPath = (Join-Path $env:USERPROFILE '.tauri\nh-media.key.pub'),
    [switch]$KeepInstallRoot
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$packageScript = Join-Path $repoRoot 'tools\package-desktop.ps1'
$signatureVerifier = Join-Path $repoRoot 'tools\verify-tauri-signature.mjs'
$markerName = '.nh-media-t434-test-root'
$createdInstallRoot = $false
$oldPublicKeyPath = $env:TAURI_SIGNING_PUBLIC_KEY_PATH

function AssertSafeInstallRoot {
    $resolvedRoot = [IO.Path]::GetFullPath($InstallRoot)
    $allowedRoot = [IO.Path]::GetFullPath((Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'NH-Media'))
    if (-not $resolvedRoot.StartsWith($allowedRoot.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) {
        throw 'InstallRoot must be a child of the per-user NH-Media test directory.'
    }
    if (Test-Path -LiteralPath $resolvedRoot) {
        throw 'InstallRoot already exists; use a fresh test directory so the result is attributable.'
    }
    New-Item -ItemType Directory -Force -Path $resolvedRoot | Out-Null
    Set-Content -LiteralPath (Join-Path $resolvedRoot $markerName) -Value 'T434 acceptance root; safe to remove after this run.' -Encoding UTF8
    $script:createdInstallRoot = $true
}

function Invoke-PackageVerify([string]$PackageVersion) {
    if (-not (Test-Path -LiteralPath $packageScript -PathType Leaf)) { throw 'Desktop package script is missing.' }
    if (-not (Test-Path -LiteralPath $PublicKeyPath -PathType Leaf)) { throw 'Owner public signing key is required for T434 verification.' }
    $env:TAURI_SIGNING_PUBLIC_KEY_PATH = (Resolve-Path -LiteralPath $PublicKeyPath).Path
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $packageScript -Action verify -Version $PackageVersion -OutputRoot $OutputRoot
    if ($LASTEXITCODE -ne 0) { throw ('Signed package verification failed for ' + $PackageVersion) }
    $manifestPath = Join-Path (Join-Path $OutputRoot $PackageVersion) 'package-manifest.json'
    return Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
}

function Get-Installer([string]$PackageVersion) {
    $bundle = Join-Path (Join-Path $OutputRoot $PackageVersion) 'bundle'
    $installer = Get-ChildItem -LiteralPath $bundle -Recurse -File | Where-Object { $_.Extension -eq '.exe' -and $_.Name -notlike '*.sig' -and $_.Name -notlike 'unins*' } | Select-Object -First 1
    if ($null -eq $installer) { throw ('No NSIS installer was found in signed package ' + $PackageVersion) }
    return $installer
}

function InstallOrUpdate([IO.FileInfo]$Installer) {
    $arguments = @('/S', ('/D=' + [IO.Path]::GetFullPath($InstallRoot)))
    $process = Start-Process -FilePath $Installer.FullName -ArgumentList $arguments -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw ('NSIS installer failed with exit code ' + $process.ExitCode) }
    $installed = Get-ChildItem -LiteralPath $InstallRoot -Recurse -File -Filter '*.exe' | Where-Object { $_.Name -notlike 'unins*' } | Select-Object -First 1
    if ($null -eq $installed) { throw 'The installer did not produce an installed application executable.' }
    return $installed
}

function AssertLaunches([IO.FileInfo]$Executable) {
    $process = Start-Process -FilePath $Executable.FullName -WorkingDirectory $Executable.DirectoryName -PassThru
    try {
        Start-Sleep -Seconds 3
        if ($process.HasExited) { throw ('Installed app exited during launch check with code ' + $process.ExitCode) }
    }
    finally {
        if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue }
    }
}

function Set-StagingPointer([string]$TargetVersion) {
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $packageScript -Action rollback -Version $TargetVersion -TargetVersion $TargetVersion -OutputRoot $OutputRoot
    if ($LASTEXITCODE -ne 0) { throw ('Staging pointer update failed for ' + $TargetVersion) }
    $pointer = Get-Content -Raw -LiteralPath (Join-Path $OutputRoot 'current.json') | ConvertFrom-Json
    if ([string]$pointer.active_version -ne $TargetVersion) { throw 'Staging pointer did not select the requested version.' }
}

function AssertTamperRejected([IO.FileInfo]$Installer) {
    $temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-t434-tamper-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force -Path $temporaryRoot | Out-Null
    $temporaryFile = Join-Path $temporaryRoot $Installer.Name
    try {
        Copy-Item -LiteralPath $Installer.FullName -Destination $temporaryFile
        Copy-Item -LiteralPath ($Installer.FullName + '.sig') -Destination ($temporaryFile + '.sig')
        $bytes = [IO.File]::ReadAllBytes($temporaryFile)
        $bytes[0] = $bytes[0] -bxor 1
        [IO.File]::WriteAllBytes($temporaryFile, $bytes)
        & node $signatureVerifier --public-key $env:TAURI_SIGNING_PUBLIC_KEY_PATH --file $temporaryFile --signature ($temporaryFile + '.sig') | Out-Null
        if ($LASTEXITCODE -eq 0) { throw 'Tampered update artifact was accepted.' }
    }
    finally {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

try {
    Set-Location -LiteralPath $repoRoot
    AssertSafeInstallRoot
    $manifestN = Invoke-PackageVerify $Version
    $manifestNext = Invoke-PackageVerify $NextVersion
    if ([string]$manifestN.public_key_sha256 -eq '' -or [string]$manifestN.public_key_sha256 -ne [string]$manifestNext.public_key_sha256) {
        throw 'N and N+1 packages are not signed by the same external identity.'
    }
    $installerN = Get-Installer $Version
    $installerNext = Get-Installer $NextVersion
    $installedN = InstallOrUpdate $installerN
    AssertLaunches $installedN
    Set-StagingPointer $NextVersion
    $installedNext = InstallOrUpdate $installerNext
    AssertLaunches $installedNext
    AssertTamperRejected $installerNext
    Set-StagingPointer $Version
    Write-Output ('T434 install/update/tamper/rollback PASS: ' + $Version + ' -> ' + $NextVersion + ' -> ' + $Version)
}
finally {
    if ([string]::IsNullOrWhiteSpace($oldPublicKeyPath)) { Remove-Item Env:TAURI_SIGNING_PUBLIC_KEY_PATH -ErrorAction SilentlyContinue } else { $env:TAURI_SIGNING_PUBLIC_KEY_PATH = $oldPublicKeyPath }
    if ($createdInstallRoot -and -not $KeepInstallRoot) {
        Remove-Item -LiteralPath $InstallRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

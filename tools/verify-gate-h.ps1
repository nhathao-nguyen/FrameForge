[CmdletBinding()]
param([switch]$IncludeLive)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    $results = [ordered]@{}
    function Checked([string]$Name, [scriptblock]$Action) {
        & $Action
        if ($LASTEXITCODE -ne 0) { throw ($Name + ' failed with exit code ' + $LASTEXITCODE) }
        $results[$Name] = 'PASS'
    }
    Checked 'T600 Go security/process tests' { go test ./packages/shared-contracts/go/security ./services/media-worker/internal/process ./services/media-worker/internal/runtime -count=1 }
    Checked 'T601 telemetry/health/inventory tests' { go test ./services/api/internal/telemetry ./services/api/internal/health ./services/api/internal/inventory -count=1 }
    Checked 'T602 inventory/storage tests' { go test ./services/api/internal/storage ./services/api/internal/inventory -count=1 }
    $scripts = Get-ChildItem tools -Filter *.ps1 -File
    foreach ($script in $scripts) {
        $tokens = $null; $parseErrors = $null
        [void][System.Management.Automation.Language.Parser]::ParseFile($script.FullName, [ref]$tokens, [ref]$parseErrors)
        if ($parseErrors.Count -gt 0) { throw ('PowerShell parser failed: ' + $script.Name) }
    }
    $results['PowerShell parser'] = 'PASS'
    $forbidden = @()
    $forbidden += @(rg -n -F 'shell=True' services packages apps infrastructure 2>$null)
    $forbidden += @(rg -n -F 'os.system(' services packages apps infrastructure 2>$null)
    $forbidden += @(rg -n -F 'movie_narrator' services packages apps infrastructure 2>$null)
    if ($forbidden.Count -gt 0) { throw ('independence/security scan found forbidden product coupling: ' + ($forbidden -join '; ')) }
    $results['T600 independence/unsafe subprocess scan'] = 'PASS'
    if ($IncludeLive) {
        $liveEvidence = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-gate-h-verify-' + [guid]::NewGuid().ToString() + '.json')
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'accept-gate-h-live.ps1') -EvidencePath $liveEvidence
        if ($LASTEXITCODE -ne 0) { throw 'live Local acceptance failed' }
        $results['T603 live local functional path'] = 'PASS'
    } else {
        $results['T603 live local functional path'] = 'SKIPPED (use -IncludeLive on the owner/LAN server)'
    }
    $results | ConvertTo-Json -Depth 5
} finally { Pop-Location }

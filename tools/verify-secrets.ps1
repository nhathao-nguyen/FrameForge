[CmdletBinding()]
param([string]$Root)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($Root)) { $Root = Split-Path -Parent $PSScriptRoot }
$files = git -C $Root ls-files --cached --others --exclude-standard | Where-Object { $_ -notmatch '^(docs/|tests/reference-behavior/|.*\.lock$)' }
$patterns = @('-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----', '\bAKIA[0-9A-Z]{16}\b', '\bgh[pousr]_[A-Za-z0-9]{20,}\b', '\bxox[baprs]-[A-Za-z0-9-]{20,}\b', '(?i)presigned[_-]?url\s*[:=]\s*https?://[^\s"'']+\?(?=[^\r\n]*(X-Amz|Signature|token))')
$hits = @()
foreach ($file in $files) {
    $path = Join-Path $Root $file
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { continue }
    $content = Get-Content -Raw -LiteralPath $path
    foreach ($pattern in $patterns) { if ($content -match $pattern) { $hits += "$file matches secret pattern" } }
}
if ($hits.Count -gt 0) { throw ($hits -join [Environment]::NewLine) }
$example = Get-Content -LiteralPath (Join-Path $Root '.env.example')
foreach ($line in $example) { if ($line -match 'PASSWORD|SECRET|KEY' -and $line -notmatch 'CHANGE_ME|=$') { throw '.env.example contains a non-placeholder secret value' } }
Write-Output 'secret scan and .env.example placeholder check: pass'

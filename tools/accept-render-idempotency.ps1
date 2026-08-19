[CmdletBinding()]
param(
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
    [string]$EvidenceInput = 'docs/evidence/real-local-gate-h-live-20260819-r31.json',
    [string]$EvidenceOutput = 'docs/evidence/real-local-render-idempotency-20260819-r34.json',
    [ValidateSet('youtube_16_9', 'shorts_9_16', 'square_1_1')]
    [string]$RenderProfileKey = 'youtube_16_9',
    [string]$EnvFile = ''
)

$ErrorActionPreference = 'Stop'
$apiRoot = $ApiBaseUrl.TrimEnd('/') + '/api/v1'

function Import-EnvFile([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq '' -or $trimmed.StartsWith('#')) { continue }
        if ($trimmed -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { throw 'Invalid environment line.' }
        $value = $Matches[2].Trim()
        if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) { $value = $value.Substring(1, $value.Length - 2) }
        Set-Item -Path ('Env:' + $Matches[1]) -Value $value
    }
}

Import-EnvFile $EnvFile
if ([string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_USERNAME) -or [string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_PASSWORD)) { throw 'LocalAuth environment is required.' }
if (-not (Test-Path -LiteralPath $EvidenceInput -PathType Leaf)) { throw 'Input evidence was not found.' }

$input = Get-Content -Raw -LiteralPath $EvidenceInput | ConvertFrom-Json
$projectId = [string]$input.project_id
$timelineVersionId = [string]$input.timeline_version_id
$loginBody = @{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD } | ConvertTo-Json -Compress
$login = Invoke-RestMethod -Method Post -Uri ($apiRoot + '/auth/local/login') -ContentType 'application/json' -Body $loginBody
$token = [string]$login.access_token
$body = @{ timeline_version_id = $timelineVersionId; render_profile = @{ profile_key = $RenderProfileKey; version = 1 }; preview = $false; overrides = @{} } | ConvertTo-Json -Depth 20 -Compress
$statusCode = 0
$responseBody = $null
try {
    $response = Invoke-WebRequest -UseBasicParsing -Method Post -Uri ($apiRoot + '/projects/' + $projectId + '/renders') -Headers @{ Authorization = 'Bearer ' + $token } -ContentType 'application/json' -Body $body -TimeoutSec 30
    $statusCode = [int]$response.StatusCode
    $responseBody = $response.Content | ConvertFrom-Json
} catch {
    $errorResponse = $_.Exception.Response
    $statusCode = [int]$errorResponse.StatusCode.value__
    $responseBody = [pscustomobject]@{ error = 'HTTP ' + $statusCode + ' returned by render request' }
}

$safeBody = [ordered]@{}
if ($null -ne $responseBody) {
    foreach ($property in $responseBody.psobject.Properties) {
        if ($property.Name -notin @('access_token', 'refresh_token', 'token')) { $safeBody[$property.Name] = $property.Value }
    }
}
$report = [ordered]@{
    schema_version = 'nh-media/render-idempotency/v1'
    project_id = $projectId
    timeline_version_id = $timelineVersionId
    render_profile_key = $RenderProfileKey
    http_status = $statusCode
    response = [pscustomobject]@{ error = [string]$safeBody['error'] }
    no_job_created = ($statusCode -in @(409, 412))
    safe_error = (($safeBody.Keys -notcontains 'traceback') -and ($safeBody | ConvertTo-Json -Compress) -notmatch '(?i)password|token|secret|bearer|stack trace|\\\\')
    status = if ($statusCode -in @(409, 412) -and ($safeBody | ConvertTo-Json -Compress) -notmatch '(?i)traceback|password|token|secret|bearer|stack trace') { 'PASS' } else { 'NOT_PASS' }
    finished_at = (Get-Date).ToUniversalTime().ToString('o')
}
$parent = Split-Path -Parent ([IO.Path]::GetFullPath($EvidenceOutput))
New-Item -ItemType Directory -Force -Path $parent | Out-Null
$report | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $EvidenceOutput -Encoding UTF8
Get-Content -Raw -LiteralPath $EvidenceOutput

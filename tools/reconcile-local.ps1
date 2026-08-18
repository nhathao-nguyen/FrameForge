[CmdletBinding()]
param(
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
    [string]$WorkerToken = $env:NH_MEDIA_WORKER_TOKEN
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($WorkerToken)) { throw 'NH_MEDIA_WORKER_TOKEN is required and is never written to a report.' }
$headers = @{ 'X-NH-Worker-Token' = $WorkerToken }
$response = Invoke-RestMethod -Method Post -Uri ($ApiBaseUrl.TrimEnd('/') + '/internal/v1/ops/reconcile') -Headers $headers -ContentType 'application/json' -Body '{}'
$response | ConvertTo-Json -Depth 5

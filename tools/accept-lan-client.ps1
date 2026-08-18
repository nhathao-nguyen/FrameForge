#requires -Version 5.1

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ServerLanIp,
    [int]$ApiPort = 8080,
    [int]$WebPort = 3000,
    [string]$EvidencePath = (Join-Path ([IO.Path]::GetTempPath()) ('nh-media-lan-client-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json')),
    [PSCredential]$Credential,
    [switch]$UseEnvironmentCredential,
    [switch]$KeepProject
)

$ErrorActionPreference = 'Stop'
$runId = 'lan-client-' + [guid]::NewGuid().ToString()
$startedAt = (Get-Date).ToUniversalTime()
$results = [ordered]@{
    schema_version = 'nh-media/lan-client-acceptance/v1'
    run_id = $runId
    started_at = $startedAt.ToString('o')
    status = 'NOT_PASS'
    server = [ordered]@{}
    client = [ordered]@{}
    checks = [ordered]@{}
}
$projectId = ''
$uploadContext = $null
$failure = $null
$script:stage = 'initialization'

function ConvertTo-IPv4([string]$Value) {
    $parsed = $null
    if (-not [Net.IPAddress]::TryParse($Value, [ref]$parsed) -or $parsed.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork) {
        throw 'ServerLanIp must be an IPv4 address, not a hostname or URL.'
    }
    if ([Net.IPAddress]::IsLoopback($parsed) -or $parsed.IPAddressToString -like '169.254.*') {
        throw 'ServerLanIp must be a routable private-LAN IPv4 address, not loopback/APIPA.'
    }
    $octets = $parsed.GetAddressBytes()
    $privateLan = $octets[0] -eq 10 -or
        ($octets[0] -eq 172 -and $octets[1] -ge 16 -and $octets[1] -le 31) -or
        ($octets[0] -eq 192 -and $octets[1] -eq 168)
    if (-not $privateLan) { throw 'ServerLanIp must be an RFC1918 private-LAN IPv4 address.' }
    return $parsed.IPAddressToString
}

function Get-ClientIPv4 {
    try {
        return @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop |
            Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
            Select-Object -ExpandProperty IPAddress -Unique | Sort-Object)
    } catch {
        return @([Net.Dns]::GetHostAddresses([Net.Dns]::GetHostName()) |
            Where-Object { $_.AddressFamily -eq [Net.Sockets.AddressFamily]::InterNetwork -and $_.IPAddressToString -notlike '127.*' } |
            Select-Object -ExpandProperty IPAddressToString -Unique | Sort-Object)
    }
}

function Assert-ExternalClient([string]$TargetIp, [string[]]$Addresses) {
    if ($Addresses.Count -eq 0) { throw 'The client has no non-loopback IPv4 address to record.' }
    if ($Addresses -contains $TargetIp) {
        throw ('This process appears to run on the server machine because ' + $TargetIp + ' is a local client address. Run this script on a different physical device.')
    }
}

function Test-TcpPort([string]$TargetHost, [int]$Port) {
    $tcp = [Net.Sockets.TcpClient]::new()
    try {
        $task = $tcp.ConnectAsync($TargetHost, $Port)
        if (-not $task.Wait(5000)) { return $false }
        return $tcp.Connected
    } catch {
        return $false
    } finally {
        $tcp.Dispose()
    }
}

function Get-HttpFailure([object]$ErrorRecord, [string]$Stage) {
    $response = $ErrorRecord.Exception.Response
    if ($null -eq $response) { return ($Stage + ': ' + $ErrorRecord.Exception.Message) }
    $status = [int]$response.StatusCode
    $body = ''
    try {
        $reader = [IO.StreamReader]::new($response.GetResponseStream())
        $body = $reader.ReadToEnd()
        $reader.Dispose()
    } catch { $body = '' }
    if ($body.Length -gt 512) { $body = $body.Substring(0, 512) }
    if ([string]::IsNullOrWhiteSpace($body)) { return ($Stage + ': HTTP ' + $status) }
    return ($Stage + ': HTTP ' + $status + ' response=' + $body)
}

function Invoke-Api([string]$Method, [string]$Path, [string]$Token = '', [object]$Body = $null, [hashtable]$ExtraHeaders = @{}, [string]$Stage = '') {
    if (-not [string]::IsNullOrWhiteSpace($Stage)) { $script:stage = $Stage }
    $headers = @{ 'X-NH-Acceptance-Client-Id' = $runId }
    if ($Token -ne '') { $headers.Authorization = 'Bearer ' + $Token }
    foreach ($key in $ExtraHeaders.Keys) { $headers[$key] = $ExtraHeaders[$key] }
    $params = @{ Method = $Method; Uri = $script:apiRoot + $Path; Headers = $headers; TimeoutSec = 15; ErrorAction = 'Stop' }
    if ($null -ne $Body) {
        $params.ContentType = 'application/json'
        $params.Body = ($Body | ConvertTo-Json -Depth 10 -Compress)
    }
    try { return Invoke-RestMethod @params } catch { throw (Get-HttpFailure $_ $script:stage) }
}

function Invoke-HealthCheck([string]$Stage, [string]$Uri) {
    $script:stage = $Stage
    try {
        return (Invoke-WebRequest -UseBasicParsing $Uri -TimeoutSec 15 -Headers @{ 'X-NH-Acceptance-Client-Id' = $runId }).StatusCode -eq 200
    } catch {
        throw (Get-HttpFailure $_ $Stage)
    }
}

function Read-SseFrame([string]$Project, [string]$Job, [int64]$LastEventId) {
    $handler = [System.Net.Http.HttpClientHandler]::new()
    $handler.UseProxy = $false
    $http = [System.Net.Http.HttpClient]::new($handler)
    $http.Timeout = [TimeSpan]::FromSeconds(15)
    $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::Get, ($script:apiRoot + '/projects/' + $Project + '/jobs/' + $Job + '/events/stream'))
    $request.Headers.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new('Bearer', $script:token)
    $request.Headers.Add('X-NH-Acceptance-Client-Id', $runId)
    $request.Headers.Add('Last-Event-ID', [string]$LastEventId)
    $request.Headers.Accept.Add([System.Net.Http.Headers.MediaTypeWithQualityHeaderValue]::new('text/event-stream'))
    $response = $null
    $reader = $null
    try {
        $response = $http.SendAsync($request, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
        if (-not $response.IsSuccessStatusCode) { throw ('SSE reconnect returned HTTP ' + [int]$response.StatusCode) }
        $reader = [IO.StreamReader]::new($response.Content.ReadAsStreamAsync().GetAwaiter().GetResult())
        $lines = [Collections.Generic.List[string]]::new()
        for ($index = 0; $index -lt 8; $index++) {
            $readTask = $reader.ReadLineAsync()
            if (-not $readTask.Wait(15000)) { throw 'SSE frame read timed out after 15 seconds.' }
            $line = $readTask.GetAwaiter().GetResult()
            if ($null -eq $line) { break }
            [void]$lines.Add($line)
            if ($line -eq '') { break }
        }
        return ($lines -join "`n")
    } finally {
        if ($null -ne $reader) { $reader.Dispose() }
        if ($null -ne $response) { $response.Dispose() }
        if ($null -ne $request) { $request.Dispose() }
        $http.Dispose()
    }
}

try {
    $script:stage = 'runtime_compatibility'
    Add-Type -AssemblyName System.Net.Http -ErrorAction Stop
    $results.checks.runtime_compatibility = $null -ne ('System.Net.Http.HttpClient' -as [type])
    if (-not $results.checks.runtime_compatibility) { throw 'System.Net.Http.HttpClient is unavailable.' }

    $script:stage = 'server_ip_validation'
    $normalizedServerIp = ConvertTo-IPv4 $ServerLanIp
    $script:stage = 'client_identity'
    $clientAddresses = @(Get-ClientIPv4)
    Assert-ExternalClient $normalizedServerIp $clientAddresses

    $script:apiRoot = 'http://' + $normalizedServerIp + ':' + $ApiPort + '/api/v1'
    $webRoot = 'http://' + $normalizedServerIp + ':' + $WebPort
    $results.server = [ordered]@{
        lan_ip = $normalizedServerIp
        api_url = 'http://' + $normalizedServerIp + ':' + $ApiPort
        web_url = $webRoot
    }
    $results.client = [ordered]@{
        machine_name = [Net.Dns]::GetHostName()
        ipv4 = $clientAddresses
        windows_version = [Environment]::OSVersion.VersionString
        powershell_version = $PSVersionTable.PSVersion.ToString()
        external_to_server = $true
    }

    $script:stage = 'tcp_reachability'
    $results.checks.api_tcp_reachable = Test-TcpPort $normalizedServerIp $ApiPort
    $results.checks.web_tcp_reachable = Test-TcpPort $normalizedServerIp $WebPort
    if (-not $results.checks.api_tcp_reachable -or -not $results.checks.web_tcp_reachable) {
        throw 'The client cannot reach both LAN TCP endpoints.'
    }
    $results.checks.web_reachable = Invoke-HealthCheck 'web_health' $webRoot
    $results.checks.api_live = Invoke-HealthCheck 'api_live' ($script:apiRoot + '/live')
    $results.checks.api_ready = Invoke-HealthCheck 'api_ready' ($script:apiRoot + '/ready')

    if ($UseEnvironmentCredential) {
        if ([string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_USERNAME) -or [string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_PASSWORD)) {
            throw 'UseEnvironmentCredential requires NH_API_ADMIN_USERNAME and NH_API_ADMIN_PASSWORD in the current process.'
        }
        $username = $env:NH_API_ADMIN_USERNAME
        $password = $env:NH_API_ADMIN_PASSWORD
        $results.client.credential_source = 'environment'
    } else {
        if ($null -eq $Credential) {
            $Credential = Get-Credential -UserName 'admin' -Message 'Enter the server LocalAuth account; the password is held in memory only.'
        }
        if ($null -eq $Credential) { throw 'A LocalAuth credential is required.' }
        $username = $Credential.UserName
        $password = $Credential.GetNetworkCredential().Password
        $results.client.credential_source = 'interactive'
    }
    $login = Invoke-Api 'POST' '/auth/local/login' '' @{ username = $username; password = $password } @{} 'auth_login'
    $script:token = [string]$login.access_token
    if ([string]::IsNullOrWhiteSpace($script:token)) { throw 'LocalAuth did not return an access token.' }
    $results.checks.authentication = $true
    $session = Invoke-Api 'GET' '/auth/session' $script:token $null @{} 'auth_session'
    $results.checks.workspace_membership = $null -ne $session.memberships
    $projects = Invoke-Api 'GET' '/projects' $script:token $null @{} 'project_list'
    $results.checks.project_list = $null -ne $projects.items

    $project = Invoke-Api 'POST' '/projects' $script:token @{ name = 'lan-client-' + (Get-Date -Format 'yyyyMMdd-HHmmss'); workflow_key = 'movie_recap' } @{'Idempotency-Key' = $runId + '-project'} 'project_create'
    $projectId = [string]$project.id
    if ([string]::IsNullOrWhiteSpace($projectId)) { throw 'The LAN client could not create a Project.' }
    $results.checks.project_path = $true
    $results.client.project_id = $projectId

    $upload = Invoke-Api 'POST' ('/projects/' + $projectId + '/assets/upload-sessions') $script:token @{
        kind = 'video'; filename = 'lan-client-initiation-probe.mp4'; content_type = 'video/mp4'; size_bytes = 1; multipart = $true
    } @{'Idempotency-Key' = $runId + '-upload'} 'upload_initiate'
    $uploadContext = [ordered]@{ asset_id = [string]$upload.asset.id; upload_id = [string]$upload.upload.id }
    if ([string]::IsNullOrWhiteSpace($uploadContext.asset_id) -or [string]::IsNullOrWhiteSpace($uploadContext.upload_id)) { throw 'The LAN client could not initiate an upload session.' }
    $results.checks.upload_initiated = $true
    $null = Invoke-Api 'POST' ('/projects/' + $projectId + '/assets/' + $uploadContext.asset_id + '/upload-sessions/' + $uploadContext.upload_id + '/abort') $script:token @{} @{} 'upload_abort'
    $results.checks.upload_aborted = $true
    $uploadContext = $null

    $job = Invoke-Api 'POST' ('/projects/' + $projectId + '/jobs') $script:token @{ kind = 'analysis'; mode = 'automatic'; input = @{ mode = 'deterministic' }; auto_start = $true } @{'Idempotency-Key' = $runId + '-job'} 'job_create'
    $jobId = [string]$job.id
    if ([string]::IsNullOrWhiteSpace($jobId)) { throw 'The LAN client could not create a Job.' }
    $results.checks.job_created = $true
    $terminal = $null
    for ($attempt = 0; $attempt -lt 120; $attempt++) {
        Start-Sleep -Milliseconds 500
        $current = Invoke-Api 'GET' ('/projects/' + $projectId + '/jobs/' + $jobId) $script:token $null @{} 'job_poll'
        if ($current.status -in @('completed', 'failed', 'dead_lettered', 'cancelled')) { $terminal = $current; break }
    }
    if ($null -eq $terminal) { throw 'The LAN Job did not reach a terminal state within 60 seconds.' }
    $results.checks.job_status_visibility = $true
    $results.checks.job_terminal_status = [string]$terminal.status
    $events = Invoke-Api 'GET' ('/projects/' + $projectId + '/jobs/' + $jobId + '/events?after_sequence=0&limit=100') $script:token $null @{} 'event_replay'
    $eventItems = @($events.items)
    $results.checks.event_replay = $null -ne $events.items
    $lastSequence = [int64]0
    foreach ($event in $eventItems) { if ([int64]$event.sequence -gt $lastSequence) { $lastSequence = [int64]$event.sequence } }
    $script:stage = 'sse_initial_snapshot'
    $firstFrame = Read-SseFrame $projectId $jobId 0
    $reconnectCursor = if ($lastSequence -gt 0) { $lastSequence - 1 } else { 0 }
    $script:stage = 'sse_reconnect_replay'
    $secondFrame = Read-SseFrame $projectId $jobId $reconnectCursor
    $results.checks.sse_initial_snapshot = $firstFrame -match 'event: stream\.snapshot'
    $results.checks.sse_reconnect_replay = ($secondFrame -match 'id: ' -and ($secondFrame -match 'event: '))
    if (-not $results.checks.sse_initial_snapshot -or -not $results.checks.sse_reconnect_replay) { throw 'SSE snapshot/reconnect evidence was incomplete.' }
    $results.checks.no_local_backend_dependency = $true
    $results.status = 'PASS'
} catch {
    $failure = $_
    $results.error = 'acceptance_failed'
    $results.details = $_.Exception.Message
    $results.failure_stage = $script:stage
} finally {
    if ($null -ne $uploadContext -and $projectId -ne '' -and $null -ne $script:token) {
        try { $null = Invoke-Api 'POST' ('/projects/' + $projectId + '/assets/' + $uploadContext.asset_id + '/upload-sessions/' + $uploadContext.upload_id + '/abort') $script:token @{} } catch { $results.cleanup = 'upload_abort_failed' }
    }
    if (-not $KeepProject -and $projectId -ne '' -and $null -ne $script:token) {
        try { $null = Invoke-Api 'DELETE' ('/projects/' + $projectId) $script:token @{}; $results.checks.project_cleanup = $true } catch { $results.checks.project_cleanup = $false }
    }
    $results.finished_at = (Get-Date).ToUniversalTime().ToString('o')
    $username = $null
    $password = $null
    $Credential = $null
    $parent = Split-Path -Parent $EvidencePath
    if (-not [string]::IsNullOrWhiteSpace($parent)) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    $results | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $EvidencePath -Encoding UTF8
}

$results | ConvertTo-Json -Depth 12
Write-Output ('Evidence file: ' + $EvidencePath)
if ($null -ne $failure) { exit 1 }

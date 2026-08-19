[CmdletBinding()]
param(
    [string]$EvidenceOutput = 'docs/evidence/real-local-resource-observation-20260819.json',
    [string]$EnvFile = ''
)

$ErrorActionPreference = 'Stop'
if (-not [string]::IsNullOrWhiteSpace($EnvFile)) {
    foreach ($line in Get-Content -LiteralPath $EnvFile) {
        $trimmed = $line.Trim()
        if ($trimmed -eq '' -or $trimmed.StartsWith('#')) { continue }
        if ($trimmed -match '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { Set-Item -Path ('Env:' + $Matches[1]) -Value $Matches[2].Trim().Trim('"') }
    }
}
$dockerStats = (& docker stats --no-stream --format '{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}|{{.MemPerc}}|{{.PIDs}}' 2>&1 | Out-String).Trim()
$composePs = (& docker compose --file infrastructure/compose/docker-compose.yml --profile local ps --format json 2>&1 | Out-String).Trim()
$ollamaPs = (& ollama ps 2>&1 | Out-String).Trim()
$nvidiaSmi = (& nvidia-smi --query-gpu=name,memory.used,memory.total,utilization.gpu --format=csv,noheader,nounits 2>&1 | Out-String).Trim()
$processTable = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue)
$processes = @(Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match 'nh-media|ollama|cargo-tauri|rustc' } | ForEach-Object { [ordered]@{ name = $_.ProcessName; pid = $_.Id; cpu_sec = [double]$_.CPU; working_set_mb = [math]::Round($_.WorkingSet64 / 1MB, 1); responding = $_.Responding } })
$pythonWorkers = @($processTable | Where-Object { $_.Name -match 'python' -and $_.CommandLine -match 'nh_media[.]worker' } | ForEach-Object { $process = Get-Process -Id $_.ProcessId -ErrorAction SilentlyContinue; if ($process) { [ordered]@{ name = $_.Name; pid = $_.ProcessId; command = $_.CommandLine; working_set_mb = [math]::Round($process.WorkingSet64 / 1MB, 1); cpu_sec = [double]$process.CPU } } })
$listeners = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in @(3000, 8080) } | ForEach-Object { [ordered]@{ address = $_.LocalAddress; port = $_.LocalPort; pid = $_.OwningProcess } })
$disk = @(Get-PSDrive -PSProvider FileSystem -ErrorAction SilentlyContinue | Where-Object { $_.Name -in @('C', 'D') } | ForEach-Object { [ordered]@{ drive = $_.Name; free_gb = [math]::Round($_.Free / 1GB, 2); used_gb = [math]::Round(($_.Used) / 1GB, 2) } })
$redisContainer = (& docker compose --file infrastructure/compose/docker-compose.yml --profile local ps -q redis 2>$null | Select-Object -First 1).Trim()
$redisAuth = if ($env:REDIS_PASSWORD) { 'REDISCLI_AUTH=' + $env:REDIS_PASSWORD } else { '' }
$redisStreams = if ($redisContainer -and $redisAuth) { @(& docker exec -e $redisAuth $redisContainer redis-cli --no-auth-warning --scan --pattern 'nh-media:*:worker' 2>$null) } else { @() }
$redisPending = @($redisStreams | ForEach-Object { $stream = $_.Trim(); if ($stream) { [ordered]@{ stream = $stream; pending = ((& docker exec -e $redisAuth $redisContainer redis-cli --no-auth-warning XPENDING $stream nh-media-workers 2>$null | Select-Object -First 1).Trim()) } } })
$postgresContainer = (& docker compose --file infrastructure/compose/docker-compose.yml --profile local ps -q postgres 2>$null | Select-Object -First 1).Trim()
$postgresSessions = if ($postgresContainer -and $env:POSTGRES_APP_USER -and $env:POSTGRES_APP_PASSWORD -and $env:POSTGRES_DB) { ((& docker exec -e ('PGPASSWORD=' + $env:POSTGRES_APP_PASSWORD) $postgresContainer psql -U $env:POSTGRES_APP_USER -d $env:POSTGRES_DB -Atc "select count(*) from pg_stat_activity where datname=current_database();" 2>$null | Select-Object -First 1).Trim()) } else { '' }
$minioContainer = (& docker compose --file infrastructure/compose/docker-compose.yml --profile local ps -q minio 2>$null | Select-Object -First 1).Trim()
$minioUsage = if ($minioContainer) { ((& docker exec $minioContainer sh -c 'du -sh /data 2>/dev/null || true' 2>$null | Select-Object -First 1).Trim()) } else { '' }
$report = [ordered]@{
    schema_version = 'nh-media/real-local-resource-observation/v1'
    observed_at = (Get-Date).ToUniversalTime().ToString('o')
    docker_stats = $dockerStats
    compose_ps = $composePs
    ollama_ps = $ollamaPs
    nvidia_smi = $nvidiaSmi
    relevant_processes = $processes
    python_workers = $pythonWorkers
    listeners = $listeners
    disk = $disk
    redis_pending = $redisPending
    postgres_active_sessions = $postgresSessions
    minio_data_usage = $minioUsage
    checks = [ordered]@{
        infrastructure_running = ($composePs -notmatch '(?i)error|missing|required variable' -and $composePs -match 'postgres|redis|minio')
        api_listener = (@($listeners | Where-Object port -eq 8080).Count -gt 0)
        web_listener = (@($listeners | Where-Object port -eq 3000).Count -gt 0)
        python_worker_observed = ($pythonWorkers.Count -gt 0)
        runaway_process_observed = ($processes | Where-Object { $_.working_set_mb -gt 4096 }).Count -gt 0
    }
}
$report.status = if ($report.checks.infrastructure_running -and $report.checks.api_listener -and $report.checks.web_listener -and $report.checks.python_worker_observed -and -not $report.checks.runaway_process_observed) { 'PASS' } else { 'NOT_PASS' }
$parent = Split-Path -Parent ([IO.Path]::GetFullPath($EvidenceOutput))
New-Item -ItemType Directory -Force -Path $parent | Out-Null
$report | ConvertTo-Json -Depth 15 | Set-Content -LiteralPath $EvidenceOutput -Encoding UTF8
Get-Content -Raw -LiteralPath $EvidenceOutput

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$BackupPath,
    [Parameter(Mandatory = $true)][string]$TargetDatabaseUrl,
    [string]$ManifestPath = ''
)

$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $BackupPath -PathType Leaf)) { throw 'Backup file does not exist.' }
if ([string]::IsNullOrWhiteSpace($TargetDatabaseUrl)) { throw 'A separate target database URL is required.' }
$targetName = ''
try { $targetUri = [Uri]$TargetDatabaseUrl; $targetName = $targetUri.AbsolutePath.Trim('/') } catch { }
if ($targetName -notmatch '(?i)(restore|recovery|rehearsal)') { throw 'Target database name must explicitly contain restore, recovery, or rehearsal.' }
if (-not [string]::IsNullOrWhiteSpace($env:NH_MEDIA_DATABASE_URL) -and $TargetDatabaseUrl -eq $env:NH_MEDIA_DATABASE_URL) { throw 'Refusing to restore over the source database.' }
$pgRestore = Get-Command pg_restore.exe -ErrorAction SilentlyContinue
if ($null -eq $pgRestore) { $pgRestore = Get-Command pg_restore -ErrorAction SilentlyContinue }
if ($null -eq $pgRestore) { throw 'pg_restore is required.' }
$psql = Get-Command psql.exe -ErrorAction SilentlyContinue
if ($null -eq $psql) { $psql = Get-Command psql -ErrorAction SilentlyContinue }
if ($null -eq $psql) { throw 'psql is required to verify that the restore target is clean.' }
$tableCount = (& $psql.Source "--dbname=$TargetDatabaseUrl" '--set=ON_ERROR_STOP=1' '--tuples-only' '--no-align' '--command=SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=''public'' AND c.relkind IN (''r'',''p'',''v'',''m'',''f'',''S'')') -join ''
if ($LASTEXITCODE -ne 0) { throw 'Could not verify that the restore target is clean.' }
$parsedTableCount = 0
if (-not [int]::TryParse($tableCount.Trim(), [ref]$parsedTableCount)) { throw 'Could not parse the restore target table count.' }
if ($parsedTableCount -ne 0) { throw 'Restore target must be a clean database with no public user tables.' }
if (-not [string]::IsNullOrWhiteSpace($ManifestPath)) {
    $manifest = Get-Content -Raw -LiteralPath $ManifestPath | ConvertFrom-Json
    $expected = [string]$manifest.database_backup.sha256
    $actual = (Get-FileHash -LiteralPath $BackupPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expected -and $expected.ToLowerInvariant() -ne $actual) { throw 'Backup checksum does not match the manifest.' }
}
# No --clean/--create is used. The operator must provide a new empty target,
# making this procedure non-destructive to the source and recoverable on error.
& $pgRestore.Source '--exit-on-error' '--no-owner' '--no-acl' '--dbname' $TargetDatabaseUrl $BackupPath
if ($LASTEXITCODE -ne 0) { throw 'pg_restore failed; source database was not modified.' }
Write-Output ('restore completed into separate target: ' + $targetName)

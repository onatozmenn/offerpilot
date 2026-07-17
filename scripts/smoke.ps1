[CmdletBinding()]
param(
    [Parameter()]
    [ValidateNotNullOrEmpty()]
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',

    [Parameter()]
    [ValidateRange(10, 900)]
    [int]$TimeoutSeconds = 120,

    [Parameter()]
    [long]$Seed = 20260717,

    [Parameter()]
    [switch]$CleanupCompose,

    [Parameter()]
    [switch]$KeepData,

    [Parameter()]
    [string]$ComposeProjectName = ''
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

if ($KeepData -and -not $CleanupCompose) {
    throw '-KeepData requires -CleanupCompose; cleanup is never implicit.'
}

$script:RepositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$script:ApiBaseUrl = $ApiBaseUrl.TrimEnd('/')
$script:Deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
$script:ConvertFromJsonSupportsDateKind = (Get-Command ConvertFrom-Json).Parameters.ContainsKey('DateKind')
$stopwatch = [System.Diagnostics.Stopwatch]::StartNew()

function ConvertFrom-OfferPilotJson {
    param([Parameter(Mandatory = $true)][string]$Json)

    if ($script:ConvertFromJsonSupportsDateKind) {
        return $Json | ConvertFrom-Json -DateKind String
    }
    return $Json | ConvertFrom-Json
}

function Get-RemainingTimeoutSeconds {
    $remaining = [Math]::Ceiling(($script:Deadline - [DateTime]::UtcNow).TotalSeconds)
    if ($remaining -lt 1) {
        throw "Smoke deadline exceeded after $TimeoutSeconds seconds."
    }
    return [int]$remaining
}

function Get-ResponseHeader {
    param(
        [Parameter(Mandatory = $true)]$Response,
        [Parameter(Mandatory = $true)][string]$Name
    )

    try {
        $value = $Response.Headers[$Name]
        if ($null -ne $value) {
            return [string]$value
        }
    }
    catch {
    }

    try {
        $values = $Response.Headers.GetValues($Name)
        if ($null -ne $values) {
            return [string](@($values)[0])
        }
    }
    catch {
    }

    return ''
}

function Read-ErrorResponseBody {
    param([Parameter(Mandatory = $true)]$Response)

    $body = ''
    try {
        if ($null -ne $Response.Content -and $Response.Content -isnot [string]) {
            $body = $Response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        }
        elseif ($Response.Content -is [string]) {
            $body = [string]$Response.Content
        }
    }
    catch {
    }

    if ([string]::IsNullOrEmpty($body)) {
        try {
            $stream = $Response.GetResponseStream()
            if ($null -ne $stream) {
                $reader = New-Object System.IO.StreamReader($stream)
                try {
                    $body = $reader.ReadToEnd()
                }
                finally {
                    $reader.Dispose()
                }
            }
        }
        catch {
        }
    }

    if ($body.Length -gt 16384) {
        return $body.Substring(0, 16384)
    }
    return $body
}

function Get-SafeHttpFailure {
    param(
        [Parameter(Mandatory = $true)]$ErrorRecord,
        [Parameter(Mandatory = $true)][string]$RequestId
    )

    $response = $ErrorRecord.Exception.Response
    if ($null -eq $response) {
        return "network failure; request_id=$RequestId; message=$($ErrorRecord.Exception.Message)"
    }

    $statusCode = [int]$response.StatusCode
    $responseRequestId = Get-ResponseHeader -Response $response -Name 'X-Request-ID'
    if ([string]::IsNullOrWhiteSpace($responseRequestId)) {
        $responseRequestId = $RequestId
    }

    $body = ''
    if ($null -ne $ErrorRecord.ErrorDetails -and -not [string]::IsNullOrWhiteSpace([string]$ErrorRecord.ErrorDetails.Message)) {
        $body = [string]$ErrorRecord.ErrorDetails.Message
    }
    if ([string]::IsNullOrWhiteSpace($body)) {
        $body = Read-ErrorResponseBody -Response $response
    }
    $problemCode = ''
    $problemDetail = ''
    if (-not [string]::IsNullOrWhiteSpace($body)) {
        try {
            $problem = ConvertFrom-OfferPilotJson -Json $body
            if ($null -ne $problem.code) {
                $problemCode = [string]$problem.code
            }
            if ($null -ne $problem.detail) {
                $problemDetail = [string]$problem.detail
            }
            if ($null -ne $problem.request_id -and -not [string]::IsNullOrWhiteSpace([string]$problem.request_id)) {
                $responseRequestId = [string]$problem.request_id
            }
        }
        catch {
        }
    }

    if ($statusCode -ge 500) {
        return "HTTP $statusCode; code=$problemCode; request_id=$responseRequestId; response body redacted"
    }
    return "HTTP $statusCode; code=$problemCode; detail=$problemDetail; request_id=$responseRequestId"
}

function Invoke-OfferPilotRequest {
    param(
        [Parameter(Mandatory = $true)][ValidateSet('GET', 'POST')][string]$Method,
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][int[]]$ExpectedStatus,
        [Parameter()]$Body
    )

    $requestId = 'smoke-' + [Guid]::NewGuid().ToString('N')
    $parameters = @{
        Uri = $script:ApiBaseUrl + $Path
        Method = $Method
        Headers = @{
            Accept = 'application/json'
            'X-Request-ID' = $requestId
        }
        TimeoutSec = Get-RemainingTimeoutSeconds
        UseBasicParsing = $true
    }
    if ($PSBoundParameters.ContainsKey('Body')) {
        $parameters.Body = $Body | ConvertTo-Json -Depth 12 -Compress
        $parameters.ContentType = 'application/json'
    }

    try {
        $response = Invoke-WebRequest @parameters
    }
    catch {
        $safeFailure = Get-SafeHttpFailure -ErrorRecord $_ -RequestId $requestId
        throw "$Method $Path failed: $safeFailure"
    }

    $statusCode = [int]$response.StatusCode
    if ($ExpectedStatus -notcontains $statusCode) {
        $responseRequestId = Get-ResponseHeader -Response $response -Name 'X-Request-ID'
        throw "$Method $Path returned unexpected HTTP $statusCode; request_id=$responseRequestId"
    }

    $parsedBody = $null
    if (-not [string]::IsNullOrWhiteSpace([string]$response.Content)) {
        try {
            $parsedBody = ConvertFrom-OfferPilotJson -Json ([string]$response.Content)
        }
        catch {
            $responseRequestId = Get-ResponseHeader -Response $response -Name 'X-Request-ID'
            throw "$Method $Path returned malformed JSON; request_id=$responseRequestId"
        }
    }

    return [PSCustomObject]@{
        StatusCode = $statusCode
        RequestId = Get-ResponseHeader -Response $response -Name 'X-Request-ID'
        Body = $parsedBody
    }
}

function Wait-ForReadiness {
    $lastFailure = 'no response'
    while ([DateTime]::UtcNow -lt $script:Deadline) {
        try {
            $response = Invoke-OfferPilotRequest -Method GET -Path '/health/ready' -ExpectedStatus @(200)
            if ($response.Body.status -eq 'ready') {
                return
            }
            $lastFailure = "unexpected status '$($response.Body.status)'"
        }
        catch {
            $lastFailure = $_.Exception.Message
        }
        [System.Threading.Thread]::Sleep(250)
    }
    throw "Readiness deadline exceeded: $lastFailure"
}

function Invoke-ExplicitComposeCleanup {
    if (-not $CleanupCompose) {
        return
    }

    $arguments = @('compose')
    if (-not [string]::IsNullOrWhiteSpace($ComposeProjectName)) {
        $arguments += @('-p', $ComposeProjectName)
    }
    $arguments += @('down', '--remove-orphans')
    if (-not $KeepData) {
        $arguments += '--volumes'
    }

    Push-Location $script:RepositoryRoot
    try {
        & docker @arguments
        if ($LASTEXITCODE -ne 0) {
            throw "Explicit Compose cleanup failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
}

$completed = $false
try {
    Wait-ForReadiness

    $experimentName = 'Smoke ' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss') + ' seed ' + $Seed
    $experimentResponse = Invoke-OfferPilotRequest -Method POST -Path '/v1/demo/experiments' -ExpectedStatus @(201) -Body @{
        name = $experimentName
        policy_kind = 'segmented_epsilon_greedy'
        epsilon = 0.15
    }
    $experimentId = [string]$experimentResponse.Body.id
    if ([string]::IsNullOrWhiteSpace($experimentId)) {
        throw 'Create experiment response omitted id.'
    }

    $maxDecisions = 40
    $runResponse = Invoke-OfferPilotRequest -Method POST -Path "/v1/experiments/$experimentId/simulation-runs" -ExpectedStatus @(201) -Body @{
        seed = $Seed
        requests_per_second = 40
        max_decisions = $maxDecisions
    }
    $runId = [string]$runResponse.Body.run_id
    if ([string]::IsNullOrWhiteSpace($runId)) {
        throw 'Create simulation response omitted run_id.'
    }

    $run = $runResponse.Body
    while (@('starting', 'running', 'stopping') -contains [string]$run.status) {
        if ([DateTime]::UtcNow -ge $script:Deadline) {
            throw "Simulation $runId did not reach a terminal state before the deadline."
        }
        [System.Threading.Thread]::Sleep(250)
        $run = (Invoke-OfferPilotRequest -Method GET -Path "/v1/simulation-runs/$runId" -ExpectedStatus @(200)).Body
    }

    if ($run.status -ne 'completed') {
        throw "Simulation ended with status '$($run.status)' and code '$($run.error_code)'."
    }
    if ([int64]$run.decision_count -ne $maxDecisions -or [int64]$run.outcome_count -ne $maxDecisions) {
        throw "Simulation count mismatch: decisions=$($run.decision_count), outcomes=$($run.outcome_count), expected=$maxDecisions."
    }

    $summaryPath = "/v1/experiments/$experimentId/summary?max_learning_points=120"
    $summaryBefore = (Invoke-OfferPilotRequest -Method GET -Path $summaryPath -ExpectedStatus @(200)).Body
    if ([int64]$summaryBefore.decision_count -ne $maxDecisions -or [int64]$summaryBefore.outcome_count -ne $maxDecisions) {
        throw "Summary count mismatch: decisions=$($summaryBefore.decision_count), outcomes=$($summaryBefore.outcome_count)."
    }
    if ($null -eq $summaryBefore.average_reward -or $null -eq $summaryBefore.engagement_rate) {
        throw 'Summary omitted observed average reward or engagement rate.'
    }
    if ($null -eq $summaryBefore.random_benchmark.expected_average_reward -or $null -eq $summaryBefore.oracle_benchmark.expected_average_reward) {
        throw 'Summary omitted simulation benchmark values.'
    }
    if (@($summaryBefore.learning_series).Count -lt 1) {
        throw 'Summary omitted the learning series.'
    }

    $feed = (Invoke-OfferPilotRequest -Method GET -Path "/v1/experiments/$experimentId/decisions?limit=200" -ExpectedStatus @(200)).Body
    $captured = @($feed.items | Where-Object { $null -ne $_.outcome } | Select-Object -First 1)
    if ($captured.Count -ne 1) {
        throw 'Decision feed did not provide a terminal outcome payload for idempotency validation.'
    }
    $decision = $captured[0]
    $replayResponse = Invoke-OfferPilotRequest -Method POST -Path '/v1/outcomes' -ExpectedStatus @(200) -Body @{
        event_id = [string]$decision.outcome.event_id
        decision_id = [string]$decision.decision_id
        outcome = [string]$decision.outcome.outcome
        occurred_at = [string]$decision.outcome.occurred_at
    }
    if ([string]$replayResponse.Body.event_id -ne [string]$decision.outcome.event_id) {
        throw 'Idempotent outcome replay returned a different event.'
    }

    $summaryAfter = (Invoke-OfferPilotRequest -Method GET -Path $summaryPath -ExpectedStatus @(200)).Body
    if ([int64]$summaryAfter.decision_count -ne [int64]$summaryBefore.decision_count -or
        [int64]$summaryAfter.outcome_count -ne [int64]$summaryBefore.outcome_count -or
        [int64]$summaryAfter.policy_version -ne [int64]$summaryBefore.policy_version) {
        throw 'Idempotent replay changed summary counts or policy version.'
    }

    $completed = $true
    $stopwatch.Stop()
    Write-Output 'OfferPilot smoke test passed.'
    Write-Output "seed=$Seed decisions=$($run.decision_count) outcomes=$($run.outcome_count) policy_version=$($summaryAfter.policy_version) elapsed_ms=$($stopwatch.ElapsedMilliseconds)"
}
finally {
    try {
        Invoke-ExplicitComposeCleanup
    }
    catch {
        if ($completed) {
            throw
        }
        Write-Warning $_.Exception.Message
    }
}
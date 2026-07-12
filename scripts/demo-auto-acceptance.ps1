# Phase R1.2-Auto - Demo automated acceptance orchestrator.
# Usage: .\scripts\demo-auto-acceptance.ps1 [-ApiBase http://127.0.0.1:8080] [-SkipBuild] [-SkipApiTests]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [switch]$SkipBuild,
    [switch]$SkipApiTests,
    [string]$ReportMd = "docs/DEMO_AUTO_ACCEPTANCE_REPORT.md",
    [string]$ReportJson = "docs/demo-auto-acceptance.json"
)

$ErrorActionPreference = "Continue"
$repoRoot = Split-Path -Parent $PSScriptRoot
$backendDir = Join-Path $repoRoot "backend"
$startedAt = (Get-Date).ToUniversalTime()
$steps = @()
$overallFailed = 0
$overallBlocked = 0
$overallDeferred = 0
$overallWarning = 0
$overallCodeFailed = 0
$overallNonAiFailed = 0

function Add-Step {
    param(
        [string]$Name,
        [string]$Category = "code_check",
        [string]$Status,
        [int]$ExitCode = 0,
        [string]$Detail = "",
        [string]$ReasonCode = "",
        [string]$LogFile = ""
    )
    $durationMs = 0
    $script:steps += @{
        name       = $Name
        category   = $Category
        status     = $Status
        exitCode   = $ExitCode
        detail     = $Detail
        reasonCode = $ReasonCode
        logFile    = $LogFile
        durationMs = $durationMs
        finishedAt = (Get-Date).ToUniversalTime().ToString("o")
    }
    if ($Status -eq "failed") { $script:overallFailed++ }
    if ($Status -eq "blocked") { $script:overallBlocked++ }
    if ($Status -eq "deferred") { $script:overallDeferred++ }
    if ($Status -eq "warning") { $script:overallWarning++ }
    if ($Status -eq "failed" -and $Category -in @("code_check", "build", "seed", "smoke")) { $script:overallCodeFailed++ }
    if ($Status -eq "failed" -and $Category -ne "external_provider") { $script:overallNonAiFailed++ }
    Write-Host ("[{0}] {1} (exit {2}) {3}" -f $Status.ToUpper(), $Name, $ExitCode, $Detail)
}

function Run-Step {
    param(
        [string]$Name,
        [string]$Category = "code_check",
        [scriptblock]$Block,
        [int[]]$BlockedExitCodes = @(2, 3),
        [int[]]$WarningExitCodes = @(),
        [string]$LogFile = ""
    )
    Write-Host ""
    Write-Host "=== $Name ==="
    $stepStart = Get-Date
    $global:LASTEXITCODE = 0
    try {
        $result = & $Block
        if ($null -ne $result -and $result -is [int]) {
            $code = [int]$result
        } else {
            $code = $LASTEXITCODE
        }
        if ($null -eq $code) { $code = 0 }
        $durationMs = [int]((Get-Date) - $stepStart).TotalMilliseconds
        if ($BlockedExitCodes -contains $code) {
            Add-Step -Name $Name -Category $Category -Status "blocked" -ExitCode $code -Detail "blocked_by_config_or_credentials" -ReasonCode "environment_blocked" -LogFile $LogFile
            $script:steps[$script:steps.Count - 1].durationMs = $durationMs
        } elseif ($WarningExitCodes -contains $code) {
            Add-Step -Name $Name -Category $Category -Status "warning" -ExitCode $code -Detail "completed_with_warning" -ReasonCode "warning" -LogFile $LogFile
            $script:steps[$script:steps.Count - 1].durationMs = $durationMs
        } elseif ($code -ne 0) {
            Add-Step -Name $Name -Category $Category -Status "failed" -ExitCode $code -ReasonCode "code_failed" -LogFile $LogFile
            $script:steps[$script:steps.Count - 1].durationMs = $durationMs
        } else {
            Add-Step -Name $Name -Category $Category -Status "passed" -ExitCode 0 -ReasonCode "passed" -LogFile $LogFile
            $script:steps[$script:steps.Count - 1].durationMs = $durationMs
        }
    } catch {
        Add-Step -Name $Name -Category $Category -Status "failed" -ExitCode 1 -Detail $_.Exception.Message -ReasonCode "code_failed" -LogFile $LogFile
        $script:steps[$script:steps.Count - 1].durationMs = [int]((Get-Date) - $stepStart).TotalMilliseconds
    }
}

function Test-BackendUp {
    try {
        $h = Invoke-RestMethod -Method Get -Uri "$ApiBase/health" -TimeoutSec 5
        return ($null -ne $h.data.status)
    } catch {
        return $false
    }
}

$backendUp = Test-BackendUp
$testEnv = @{
    apiBase   = $ApiBase
    backendUp = $backendUp
    appEnv    = $null
    goTestEnvMode = "isolated_test_env"
    startedAt = $startedAt.ToString("o")
    hostname  = $env:COMPUTERNAME
    phase     = "Phase P4-R"
}

Write-Host "TradeMind Phase P4-R Demo Regression Acceptance"
Write-Host "API: $ApiBase | Backend up: $backendUp"

$goPackages = @(
    "./...",
    "./internal/providers/platform/douyinshop/...",
    "./internal/modules/productpublish/...",
    "./internal/modules/ordersync/...",
    "./internal/modules/aiproducttext/...",
    "./internal/modules/aiproductimage/...",
    "./internal/modules/aiopsworkbench/...",
    "./internal/modules/taskcenter/...",
    "./internal/modules/customerchat/...",
    "./internal/modules/operationdashboard/...",
    "./internal/modules/order/...",
    "./internal/modules/inventory/...",
    "./internal/modules/configstatus/...",
    "./internal/pkg/adminperm/..."
)

Run-Step "go test regression" -Category "code_check" -LogFile "artifacts/demo-acceptance/go-test.log" {
    Push-Location $repoRoot
    & node "$PSScriptRoot/go-test-isolated.mjs" test ./...
    $code = $LASTEXITCODE
    Pop-Location
    return $code
}

if (-not $SkipBuild) {
    Run-Step "go build backend" -Category "build" {
        Push-Location $backendDir
        New-Item -ItemType Directory -Path "tmp" -Force | Out-Null
        & go build -o tmp/server ./cmd/server/...
        $code = $LASTEXITCODE
        Pop-Location
        return $code
    }

    Run-Step "pnpm build:admin" -Category "build" {
        Push-Location $repoRoot
        & pnpm build:admin
        $code = $LASTEXITCODE
        Pop-Location
        return $code
    }

    Run-Step "git diff --check" -Category "code_check" {
        Push-Location $repoRoot
        & git diff --check
        $code = $LASTEXITCODE
        Pop-Location
        return $code
    }
} else {
    Add-Step -Name "go build backend" -Category "build" -Status "skipped" -Detail "-SkipBuild"
    Add-Step -Name "pnpm build:admin" -Category "build" -Status "skipped" -Detail "-SkipBuild"
    Add-Step -Name "git diff --check" -Category "code_check" -Status "skipped" -Detail "-SkipBuild"
}

Run-Step "check-ui-copy" -Category "code_check" {
    & node "$PSScriptRoot/check-ui-copy.mjs" --strict --report "docs/COPYWRITING_AUDIT.auto.md" --json "docs/global-status-copywriting-scan.json"
    return $LASTEXITCODE
}

Run-Step "demo-empty-state-scan" -Category "smoke" {
    & "$PSScriptRoot/demo-empty-state-scan.ps1" -OutFile "docs/demo-empty-state-scan.auto.json"
    return $LASTEXITCODE
}

Run-Step "demo-sensitive-confirm-scan" -Category "smoke" {
    & "$PSScriptRoot/demo-sensitive-confirm-scan.ps1" -OutFile "docs/demo-sensitive-confirm-scan.auto.json"
    return $LASTEXITCODE
}

Run-Step "security-release-check" -Category "code_check" {
    & "$PSScriptRoot/security-release-check.ps1" -OutFile "docs/SECURITY_RELEASE_CHECK.auto.md"
    return $LASTEXITCODE
}

Run-Step "check-doc-links" -Category "code_check" {
    & "$PSScriptRoot/check-doc-links.ps1" -OutFile "docs/DOCS_CONSISTENCY_CHECK.md"
    return $LASTEXITCODE
}

$apiStepNames = @(
    "demo-route-smoke", "seed-demo-data", "seed-demo-permissions",
    "demo-dashboard-smoke", "demo-rbac-smoke", "demo-order-inventory-customer-smoke",
    "ai-text-route-smoke", "ai-text-trial-run",
    "ai-image-route-smoke", "ai-image-trial-run", "publish-batch-perf", "ai-operation-workbench-perf"
)

if ($SkipApiTests -or -not $backendUp) {
    $reason = if ($SkipApiTests) { "-SkipApiTests" } else { "backend not reachable" }
    foreach ($n in $apiStepNames) {
        Add-Step -Name $n -Category "smoke" -Status "skipped" -Detail $reason
    }
} else {
    Run-Step "demo-route-smoke" -Category "smoke" {
        & "$PSScriptRoot/demo-route-smoke.ps1" -ApiBase $ApiBase -OutFile "docs/demo-route-smoke.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "seed-demo-data" -Category "seed" {
        & "$PSScriptRoot/seed-demo-data.ps1" -ApiBase $ApiBase -OutFile "docs/demo-dataset.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "seed-demo-permissions" -Category "seed" {
        & "$PSScriptRoot/seed-demo-permissions.ps1" -ApiBase $ApiBase
        return $LASTEXITCODE
    }
    Run-Step "demo-dashboard-smoke" -Category "smoke" {
        & "$PSScriptRoot/demo-dashboard-smoke.ps1" -ApiBase $ApiBase
        return $LASTEXITCODE
    }
    Run-Step "demo-rbac-smoke" -Category "smoke" {
        & "$PSScriptRoot/demo-rbac-smoke.ps1" -ApiBase $ApiBase
        return $LASTEXITCODE
    }
    Run-Step "demo-order-inventory-customer-smoke" -Category "smoke" {
        & "$PSScriptRoot/demo-order-inventory-customer-smoke.ps1" -ApiBase $ApiBase
        return $LASTEXITCODE
    }
    Run-Step "ai-text-route-smoke" -Category "smoke" {
        & "$PSScriptRoot/ai-text-route-smoke.ps1" -ApiBase $ApiBase -OutFile "docs/ai-text-route-smoke.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "ai-text-trial-run" -Category "external_provider" {
        & "$PSScriptRoot/ai-text-trial-run.ps1" -ApiBase $ApiBase -OutFile "docs/ai-text-trial-run.auto.json"
        return $LASTEXITCODE
    } -BlockedExitCodes @(2, 3) -WarningExitCodes @(5)
    Run-Step "ai-image-route-smoke" -Category "smoke" {
        & "$PSScriptRoot/ai-image-route-smoke.ps1" -ApiBase $ApiBase -OutFile "docs/ai-image-route-smoke.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "ai-image-trial-run" -Category "external_provider" {
        & "$PSScriptRoot/ai-image-trial-run.ps1" -ApiBase $ApiBase -OutFile "docs/ai-image-trial-run.auto.json"
        return $LASTEXITCODE
    } -BlockedExitCodes @(2, 3) -WarningExitCodes @(5)
    Run-Step "publish-batch-perf" -Category "smoke" {
        & "$PSScriptRoot/publish-batch-perf.ps1" -ApiBase $ApiBase -OutFile "docs/publish-batch-perf.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "ai-operation-workbench-perf" -Category "smoke" {
        & "$PSScriptRoot/ai-operation-workbench-perf.ps1" -ApiBase $ApiBase -OutFile "docs/ai-operation-workbench-perf.auto.json"
        return $LASTEXITCODE
    }
}

$manualItems = @(
    "Real preprod SSH deployment",
    "Nginx / HTTPS",
    "Storage public access",
    "Preprod backup and rollback",
    "1366 / 1024 visual walkthrough",
    "Douyin real OAuth",
    "Douyin readonly E2E",
    "Douyin write E2E",
    "48-72h gray observation",
    "Tag deferred review"
)

$automatableConclusion = if ($overallFailed -eq 0) {
    if ($overallBlocked -gt 0) { "passed_with_blocked" } else { "passed" }
} else { "failed" }

$finalStatus = @{
    stage      = "Production Capability Development In Progress"
    release    = "MVP Demo Ready"
    tag        = "Tag deferred"
    production = "Not Production Ready"
    douyin     = "Douyin Release Candidate"
}

$report = @{
    phase                 = "Phase P4-R"
    testEnvironment       = $testEnv
    startedAt             = $startedAt.ToString("o")
    finishedAt            = (Get-Date).ToUniversalTime().ToString("o")
    steps                 = $steps
    automatableConclusion = $automatableConclusion
    summary               = @{
        total       = @($steps).Count
        passed      = @($steps | Where-Object { $_.status -eq "passed" }).Count
        warning     = $overallWarning
        blocked     = $overallBlocked
        deferred    = $overallDeferred
        failed      = $overallFailed
        codeFailed  = $overallCodeFailed
        nonAiFailed = $overallNonAiFailed
    }
    failedStepCount       = $overallFailed
    blockedStepCount      = $overallBlocked
    manualTestItems       = $manualItems
    finalStatus           = $finalStatus
    artifacts             = @{
        routeSmoke       = "docs/demo-route-smoke.auto.json"
        demoDataset      = "docs/demo-dataset.auto.json"
        fullProjectReport = "docs/demo-auto-acceptance.full-project.json"
        dashboardSmoke   = "docs/demo-dashboard-smoke.auto.json"
        rbacSmoke        = "docs/demo-rbac-smoke.auto.json"
        oicSmoke         = "docs/demo-order-inventory-customer-smoke.auto.json"
        emptyStateScan   = "docs/demo-empty-state-scan.auto.json"
        sensitiveScan    = "docs/demo-sensitive-confirm-scan.auto.json"
        globalStatusScan = "docs/global-status-copywriting-scan.json"
        aiTextTrial      = "docs/ai-text-trial-run.auto.json"
        aiImageTrial     = "docs/ai-image-trial-run.auto.json"
        publishBatchPerf = "docs/publish-batch-perf.auto.json"
        workbenchPerf    = "docs/ai-operation-workbench-perf.auto.json"
        copywritingAudit = "docs/COPYWRITING_AUDIT.auto.md"
        securityCheck    = "docs/SECURITY_RELEASE_CHECK.auto.md"
        docsConsistency  = "docs/DOCS_CONSISTENCY_CHECK.md"
    }
}

$dir = Split-Path -Parent $ReportJson
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 10 | Set-Content -Path $ReportJson -Encoding UTF8
$fullProjectJson = Join-Path $repoRoot "docs/demo-auto-acceptance.full-project.json"
$report | ConvertTo-Json -Depth 10 | Set-Content -Path $fullProjectJson -Encoding UTF8

$backendLabel = if ($backendUp) { "reachable" } else { "unreachable (API steps skipped)" }
$mdLines = New-Object System.Collections.Generic.List[string]
$mdLines.Add("# TradeMind Phase P4-R Demo Regression Auto Acceptance Report")
$mdLines.Add("")
$mdLines.Add("> Generated: $($report.finishedAt)")
$mdLines.Add("> API: $ApiBase | Backend: $backendLabel")
$mdLines.Add("")
$mdLines.Add("## Phase")
$mdLines.Add("")
$mdLines.Add("**Phase P4-R** - Demo regression stabilization with isolated Go test environment (not final manual acceptance)")
$mdLines.Add("")
$mdLines.Add("## Summary")
$mdLines.Add("")
$mdLines.Add("| Metric | Value |")
$mdLines.Add("| --- | --- |")
$mdLines.Add("| Conclusion | **$automatableConclusion** |")
$mdLines.Add("| Failed steps | $overallFailed |")
$mdLines.Add("| Blocked steps | $overallBlocked |")
$mdLines.Add("| Code failed | $overallCodeFailed |")
$mdLines.Add("| Non-AI failed | $overallNonAiFailed |")
$mdLines.Add("")
$mdLines.Add("## Step results")
$mdLines.Add("")
$mdLines.Add("| Step | Category | Status | Exit | Reason | Detail |")
$mdLines.Add("| --- | --- | --- | --- | --- | --- |")
foreach ($s in $steps) {
    $mdLines.Add("| $($s.name) | $($s.category) | $($s.status) | $($s.exitCode) | $($s.reasonCode) | $($s.detail) |")
}
$mdLines.Add("")
$mdLines.Add("## Artifacts")
$mdLines.Add("")
$mdLines.Add("- [demo-route-smoke.auto.json](demo-route-smoke.auto.json)")
$mdLines.Add("- [demo-dataset.auto.json](demo-dataset.auto.json)")
$mdLines.Add("- [ai-text-trial-run.auto.json](ai-text-trial-run.auto.json)")
$mdLines.Add("- [ai-image-trial-run.auto.json](ai-image-trial-run.auto.json)")
$mdLines.Add("- [publish-batch-perf.auto.json](publish-batch-perf.auto.json)")
$mdLines.Add("- [ai-operation-workbench-perf.auto.json](ai-operation-workbench-perf.auto.json)")
$mdLines.Add("- [COPYWRITING_AUDIT.auto.md](COPYWRITING_AUDIT.auto.md)")
$mdLines.Add("- [SECURITY_RELEASE_CHECK.auto.md](SECURITY_RELEASE_CHECK.auto.md)")
$mdLines.Add("- [DOCS_CONSISTENCY_CHECK.md](DOCS_CONSISTENCY_CHECK.md)")
$mdLines.Add("")
$mdLines.Add("## Manual test checklist (out of scope for automation)")
$mdLines.Add("")
foreach ($m in $manualItems) {
    $mdLines.Add("- [ ] $m")
}
$mdLines.Add("")
$mdLines.Add("## Final status")
$mdLines.Add("")
$mdLines.Add('```text')
$mdLines.Add("Production Capability Development In Progress")
$mdLines.Add("MVP Demo Ready")
$mdLines.Add("Tag deferred")
$mdLines.Add("Not Production Ready")
$mdLines.Add("Douyin Release Candidate")
$mdLines.Add('```')
$mdLines.Add("")
$mdLines.Add("Tag remains deferred in this phase. No real Douyin E2E. No production gray release.")

$mdDir = Split-Path -Parent $ReportMd
if ($mdDir -and -not (Test-Path $mdDir)) { New-Item -ItemType Directory -Path $mdDir -Force | Out-Null }
($mdLines -join "`n").TrimEnd() | Set-Content -Path $ReportMd -Encoding UTF8
$fullProjectMd = Join-Path $repoRoot "docs/DEMO_AUTO_ACCEPTANCE_FULL_PROJECT_REPORT.md"
($mdLines -join "`n").TrimEnd() | Set-Content -Path $fullProjectMd -Encoding UTF8

Write-Host ""
Write-Host "=== Summary ==="
Write-Host "Automatable conclusion: $automatableConclusion"
Write-Host "Failed: $overallFailed | Blocked: $overallBlocked | Code failed: $overallCodeFailed | Non-AI failed: $overallNonAiFailed"
Write-Host "Wrote $ReportMd"
Write-Host "Wrote $ReportJson"

if ($overallFailed -gt 0) { exit 1 }
exit 0

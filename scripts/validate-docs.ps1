[CmdletBinding()]
param()

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$script:ValidationErrors = New-Object 'System.Collections.Generic.List[string]'
$script:RepositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$trimCharacters = [char[]]@(
    [System.IO.Path]::DirectorySeparatorChar,
    [System.IO.Path]::AltDirectorySeparatorChar
)
$script:RepositoryPrefix = $script:RepositoryRoot.TrimEnd($trimCharacters) + [System.IO.Path]::DirectorySeparatorChar

function Add-ValidationError {
    param([Parameter(Mandatory = $true)][string]$Message)

    $script:ValidationErrors.Add($Message)
}

function Get-RepositoryRelativePath {
    param([Parameter(Mandatory = $true)][string]$Path)

    try {
        $fullPath = [System.IO.Path]::GetFullPath($Path)
    }
    catch {
        return $null
    }

    if ([string]::Equals($fullPath, $script:RepositoryRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        return '.'
    }

    if (-not $fullPath.StartsWith($script:RepositoryPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $null
    }

    return $fullPath.Substring($script:RepositoryPrefix.Length).Replace('\', '/')
}

function Test-ExcludedRepositoryPath {
    param([Parameter(Mandatory = $true)][string]$Path)

    $relativePath = Get-RepositoryRelativePath -Path $Path
    if ($null -eq $relativePath) {
        return $true
    }

    return $relativePath -match '(^|/)(\.git|node_modules|dist|coverage)(/|$)'
}

function Get-LinkDestination {
    param(
        [Parameter(Mandatory = $true)][string]$RawTarget,
        [Parameter(Mandatory = $true)][string]$SourcePath
    )

    $target = $RawTarget.Trim()
    if ($target.StartsWith('<')) {
        $closingBracket = $target.IndexOf('>')
        if ($closingBracket -lt 1) {
            Add-ValidationError "$SourcePath has a malformed angle-bracket link target."
            return $null
        }

        $target = $target.Substring(1, $closingBracket - 1)
    }
    else {
        $target = ($target -split '\s+', 2)[0]
    }

    if ([string]::IsNullOrWhiteSpace($target) -or $target.StartsWith('#')) {
        return $null
    }

    if ($target.StartsWith('//') -or $target -match '^[A-Za-z][A-Za-z0-9+.-]*:') {
        return $null
    }

    $separatorIndex = $target.IndexOfAny([char[]]@('#', '?'))
    if ($separatorIndex -ge 0) {
        $target = $target.Substring(0, $separatorIndex)
    }

    if ([string]::IsNullOrWhiteSpace($target)) {
        return $null
    }

    if ($target -match '%(?![0-9A-Fa-f]{2})') {
        Add-ValidationError "$SourcePath has an invalid percent-encoded link target '$target'."
        return $null
    }

    try {
        return [System.Uri]::UnescapeDataString($target)
    }
    catch {
        Add-ValidationError "$SourcePath has an invalid encoded link target '$target'."
        return $null
    }
}

function Test-MarkdownLinks {
    param(
        [Parameter(Mandatory = $true)][System.IO.FileInfo]$File,
        [Parameter(Mandatory = $true)][string]$Text
    )

    $sourcePath = Get-RepositoryRelativePath -Path $File.FullName
    $matches = [regex]::Matches($Text, '!?(?:\[[^\]\r\n]*\])\((?<target>[^)\r\n]+)\)')
    foreach ($match in $matches) {
        $destination = Get-LinkDestination -RawTarget $match.Groups['target'].Value -SourcePath $sourcePath
        if ($null -eq $destination) {
            continue
        }

        try {
            if ($destination.StartsWith('/') -or $destination.StartsWith('\')) {
                $relativeDestination = $destination.TrimStart([char[]]@('/', '\'))
                $candidatePath = Join-Path $script:RepositoryRoot $relativeDestination
            }
            else {
                $candidatePath = Join-Path $File.DirectoryName $destination
            }

            $candidatePath = [System.IO.Path]::GetFullPath($candidatePath)
        }
        catch {
            Add-ValidationError "$sourcePath has an invalid local link target '$destination'."
            continue
        }

        if ($null -eq (Get-RepositoryRelativePath -Path $candidatePath)) {
            Add-ValidationError "$sourcePath links outside the repository to '$destination'."
            continue
        }

        if (-not (Test-Path -LiteralPath $candidatePath)) {
            Add-ValidationError "$sourcePath has a broken local link to '$destination'."
        }
    }
}

function Test-GeneratedPathMatch {
    param(
        [Parameter(Mandatory = $true)][string]$PlannedPath,
        [Parameter(Mandatory = $true)][string]$GeneratedPath
    )

    $normalizedPlannedPath = $PlannedPath.Replace('\', '/')
    $normalizedGeneratedPath = $GeneratedPath.Replace('\', '/')
    if ($normalizedGeneratedPath.IndexOf('*') -ge 0) {
        $wildcardPattern = $normalizedGeneratedPath.Replace('**', '*')
        return $normalizedPlannedPath -like $wildcardPattern
    }

    return [string]::Equals(
        $normalizedPlannedPath,
        $normalizedGeneratedPath,
        [System.StringComparison]::OrdinalIgnoreCase
    )
}

$requiredPaths = @(
    'AGENTS.md',
    'docs/13-file-manifest.md',
    'docs/file-spec-template.md',
    'docs/generated-files.md',
    'docs/file-specs'
)
foreach ($requiredPath in $requiredPaths) {
    $fullRequiredPath = Join-Path $script:RepositoryRoot $requiredPath
    if (-not (Test-Path -LiteralPath $fullRequiredPath)) {
        Add-ValidationError "Repository root is missing required path '$requiredPath'."
    }
}

$markdownFiles = @()
try {
    $markdownFiles = @(
        Get-ChildItem -LiteralPath $script:RepositoryRoot -Filter '*.md' -File -Recurse -Force |
            Where-Object { -not (Test-ExcludedRepositoryPath -Path $_.FullName) } |
            Sort-Object FullName
    )
}
catch {
    Add-ValidationError "Unable to enumerate Markdown files under the repository root."
}

$fileTexts = @{}
foreach ($markdownFile in $markdownFiles) {
    $relativeMarkdownPath = Get-RepositoryRelativePath -Path $markdownFile.FullName
    try {
        $text = [System.IO.File]::ReadAllText($markdownFile.FullName)
        $fileTexts[$markdownFile.FullName.ToLowerInvariant()] = $text
    }
    catch {
        Add-ValidationError "Unable to read Markdown file '$relativeMarkdownPath'."
        continue
    }

    if (-not [regex]::IsMatch($text, '(?m)^# [^\r\n]+')) {
        Add-ValidationError "$relativeMarkdownPath does not contain an H1 heading."
    }

    Test-MarkdownLinks -File $markdownFile -Text $text
}

$instructionFiles = @(
    $markdownFiles | Where-Object {
        [string]::Equals($_.Name, 'AGENTS.md', [System.StringComparison]::OrdinalIgnoreCase) -or
        [string]::Equals($_.Name, 'copilot-instructions.md', [System.StringComparison]::OrdinalIgnoreCase)
    }
)
if ($instructionFiles.Count -ne 1) {
    Add-ValidationError "Expected exactly one AGENTS.md or copilot-instructions.md file; found $($instructionFiles.Count)."
}

$mandatorySections = @(
    'Status',
    'Purpose',
    'Depends On',
    'Public Surface',
    'Required Behavior',
    'Failure Cases',
    'Non-Goals',
    'Validation',
    'Completion Checklist'
)
$specRoot = Join-Path $script:RepositoryRoot 'docs/file-specs'
$specFiles = @(
    $markdownFiles | Where-Object {
        $relativePath = Get-RepositoryRelativePath -Path $_.FullName
        $relativePath.StartsWith('docs/file-specs/', [System.StringComparison]::OrdinalIgnoreCase) -and
        -not [string]::Equals($relativePath, 'docs/file-specs/README.md', [System.StringComparison]::OrdinalIgnoreCase)
    }
)
$specRecords = New-Object 'System.Collections.Generic.List[object]'
$specByPlannedPath = @{}
$specByRelativePath = @{}

foreach ($specFile in $specFiles) {
    $relativeSpecPath = Get-RepositoryRelativePath -Path $specFile.FullName
    $textKey = $specFile.FullName.ToLowerInvariant()
    if (-not $fileTexts.ContainsKey($textKey)) {
        continue
    }

    $specText = [string]$fileTexts[$textKey]
    $firstLine = ($specText -split '\r?\n', 2)[0].TrimStart([char]0xFEFF)
    if ($firstLine -notmatch '^# File Spec: `(?<planned>[^`]+)`$') {
        Add-ValidationError "$relativeSpecPath must start with an exact '# File Spec: <planned path>' H1."
        continue
    }

    $plannedPath = $Matches['planned'].Replace('\', '/')
    if ([System.IO.Path]::IsPathRooted($plannedPath) -or @($plannedPath -split '/') -contains '..') {
        Add-ValidationError "$relativeSpecPath declares invalid planned path '$plannedPath'."
    }

    foreach ($section in $mandatorySections) {
        $sectionPattern = '(?m)^## ' + [regex]::Escape($section) + '\s*$'
        $sectionCount = [regex]::Matches($specText, $sectionPattern).Count
        if ($sectionCount -ne 1) {
            Add-ValidationError "$relativeSpecPath must contain exactly one '## $section' section; found $sectionCount."
        }
    }

    $status = $null
    $statusSection = [regex]::Match($specText, '(?ms)^## Status\s*\r?\n(?<body>.*?)(?=^## |\z)')
    if ($statusSection.Success) {
        $statusBody = $statusSection.Groups['body'].Value.Trim()
        if ($statusBody -match '^`(?<status>specified|in_progress|implemented|validated)`$') {
            $status = $Matches['status']
        }
        else {
            Add-ValidationError "$relativeSpecPath has an invalid Status value."
        }
    }

    $plannedKey = $plannedPath.ToLowerInvariant()
    if ($specByPlannedPath.ContainsKey($plannedKey)) {
        Add-ValidationError "$relativeSpecPath duplicates planned path '$plannedPath'."
    }

    $relativeKey = $relativeSpecPath.ToLowerInvariant()
    if ($specByRelativePath.ContainsKey($relativeKey)) {
        Add-ValidationError "Duplicate specification path '$relativeSpecPath'."
    }

    $record = [pscustomobject]@{
        PlannedPath = $plannedPath
        RelativePath = $relativeSpecPath
        Status = $status
    }
    $specRecords.Add($record)
    $specByPlannedPath[$plannedKey] = $record
    $specByRelativePath[$relativeKey] = $record
}

$manifestPath = Join-Path $script:RepositoryRoot 'docs/13-file-manifest.md'
$manifestRecords = New-Object 'System.Collections.Generic.List[object]'
$manifestById = @{}
$manifestByPlannedPath = @{}
$manifestSpecCounts = @{}

if (Test-Path -LiteralPath $manifestPath) {
    try {
        $manifestText = [System.IO.File]::ReadAllText($manifestPath)
        $manifestLines = $manifestText -split '\r?\n'
        $manifestRowPattern = '^\|\s*(?<id>[A-Z]\d{3})\s*\|\s*`(?<planned>[^`]+)`\s*\|\s*\[spec\]\((?<spec>[^)]+)\)\s*\|\s*`(?<status>specified|in_progress|implemented|validated)`\s*\|\s*$'

        foreach ($manifestLine in $manifestLines) {
            if ($manifestLine -notmatch '^\|\s*[A-Z]\d{3}\s*\|') {
                continue
            }

            if ($manifestLine -notmatch $manifestRowPattern) {
                Add-ValidationError 'docs/13-file-manifest.md contains a malformed manifest row.'
                continue
            }

            $id = $Matches['id']
            $plannedPath = $Matches['planned'].Replace('\', '/')
            $rawSpecTarget = $Matches['spec']
            $status = $Matches['status']
            $decodedSpecTarget = Get-LinkDestination -RawTarget $rawSpecTarget -SourcePath 'docs/13-file-manifest.md'
            if ($null -eq $decodedSpecTarget) {
                Add-ValidationError "Manifest row '$id' must use a valid local specification link."
                continue
            }

            try {
                $specFullPath = [System.IO.Path]::GetFullPath((Join-Path (Split-Path -Parent $manifestPath) $decodedSpecTarget))
            }
            catch {
                Add-ValidationError "Manifest row '$id' has an invalid specification path."
                continue
            }

            $relativeSpecPath = Get-RepositoryRelativePath -Path $specFullPath
            if ($null -eq $relativeSpecPath) {
                Add-ValidationError "Manifest row '$id' points outside the repository."
                continue
            }

            $idKey = $id.ToLowerInvariant()
            $plannedKey = $plannedPath.ToLowerInvariant()
            $specKey = $relativeSpecPath.ToLowerInvariant()
            if ($manifestById.ContainsKey($idKey)) {
                Add-ValidationError "Manifest ID '$id' appears more than once."
            }
            if ($manifestByPlannedPath.ContainsKey($plannedKey)) {
                Add-ValidationError "Manifest planned path '$plannedPath' appears more than once."
            }
            if ($manifestSpecCounts.ContainsKey($specKey)) {
                $manifestSpecCounts[$specKey] = [int]$manifestSpecCounts[$specKey] + 1
                Add-ValidationError "Manifest specification path '$relativeSpecPath' appears more than once."
            }
            else {
                $manifestSpecCounts[$specKey] = 1
            }

            $record = [pscustomobject]@{
                Id = $id
                PlannedPath = $plannedPath
                SpecPath = $relativeSpecPath
                Status = $status
            }
            $manifestRecords.Add($record)
            $manifestById[$idKey] = $record
            $manifestByPlannedPath[$plannedKey] = $record

            if (-not (Test-Path -LiteralPath $specFullPath -PathType Leaf)) {
                Add-ValidationError "Manifest row '$id' references missing specification '$relativeSpecPath'."
                continue
            }

            if (-not $specByRelativePath.ContainsKey($specKey)) {
                Add-ValidationError "Manifest row '$id' does not reference a valid file specification."
                continue
            }

            $specRecord = $specByRelativePath[$specKey]
            if (-not [string]::Equals($specRecord.PlannedPath, $plannedPath, [System.StringComparison]::Ordinal)) {
                Add-ValidationError "Manifest row '$id' planned path does not match its specification H1."
            }
            if ($null -eq $specRecord.Status -or -not [string]::Equals($specRecord.Status, $status, [System.StringComparison]::Ordinal)) {
                Add-ValidationError "Manifest row '$id' status does not match its specification status."
            }
        }
    }
    catch {
        Add-ValidationError 'Unable to parse docs/13-file-manifest.md.'
    }
}

if ($manifestRecords.Count -eq 0) {
    Add-ValidationError 'docs/13-file-manifest.md contains no valid manifest rows.'
}

foreach ($specRecord in $specRecords) {
    $specKey = $specRecord.RelativePath.ToLowerInvariant()
    if (-not $manifestSpecCounts.ContainsKey($specKey)) {
        Add-ValidationError "$($specRecord.RelativePath) does not appear in the manifest."
    }
    elseif ([int]$manifestSpecCounts[$specKey] -ne 1) {
        Add-ValidationError "$($specRecord.RelativePath) must appear exactly once in the manifest."
    }
}

$generatedPath = Join-Path $script:RepositoryRoot 'docs/generated-files.md'
$generatedPaths = New-Object 'System.Collections.Generic.List[string]'
if (Test-Path -LiteralPath $generatedPath) {
    try {
        $generatedText = [System.IO.File]::ReadAllText($generatedPath)
        $generatedMatches = [regex]::Matches($generatedText, '(?m)^\|\s*`(?<path>[^`]+)`\s*\|')
        $generatedKeys = @{}
        foreach ($generatedMatch in $generatedMatches) {
            $generatedFilePath = $generatedMatch.Groups['path'].Value.Replace('\', '/')
            $generatedKey = $generatedFilePath.ToLowerInvariant()
            if ($generatedKeys.ContainsKey($generatedKey)) {
                Add-ValidationError "Generated path '$generatedFilePath' appears more than once."
                continue
            }

            $generatedKeys[$generatedKey] = $true
            $generatedPaths.Add($generatedFilePath)
        }
    }
    catch {
        Add-ValidationError 'Unable to parse docs/generated-files.md.'
    }
}

if ($generatedPaths.Count -eq 0) {
    Add-ValidationError 'docs/generated-files.md contains no concrete generated paths.'
}

foreach ($generatedFilePath in $generatedPaths) {
    foreach ($manifestRecord in $manifestRecords) {
        if (Test-GeneratedPathMatch -PlannedPath $manifestRecord.PlannedPath -GeneratedPath $generatedFilePath) {
            Add-ValidationError "Generated path '$generatedFilePath' has manual manifest row '$($manifestRecord.Id)'."
        }
    }

    foreach ($specRecord in $specRecords) {
        if (Test-GeneratedPathMatch -PlannedPath $specRecord.PlannedPath -GeneratedPath $generatedFilePath) {
            Add-ValidationError "Generated path '$generatedFilePath' has manual specification '$($specRecord.RelativePath)'."
        }
    }
}

if ($script:ValidationErrors.Count -gt 0) {
    [Console]::Error.WriteLine("Documentation validation failed with $($script:ValidationErrors.Count) error(s):")
    foreach ($validationError in $script:ValidationErrors) {
        [Console]::Error.WriteLine(" - $validationError")
    }
    exit 1
}

Write-Output (
    'Documentation validation passed: {0} Markdown files, {1} file specs, {2} manifest entries, {3} generated paths.' -f
    $markdownFiles.Count,
    $specRecords.Count,
    $manifestRecords.Count,
    $generatedPaths.Count
)
exit 0
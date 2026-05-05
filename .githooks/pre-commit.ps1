# pre-commit.ps1: SBOM version compliance check for shared Go modules.
#
# Warns (and blocks the commit) when files inside a shared Go module
# (shared/<name>/) are staged but the corresponding versioned 'replace'
# directive in a consumer go.mod has not been bumped.
#
# Compatible with Windows PowerShell 5.1 and PowerShell 7+.

$ErrorActionPreference = 'Stop'

# ── Helpers ───────────────────────────────────────────────────────────────────

function HasHead {
    git rev-parse --verify HEAD 2>$null | Out-Null
    return ($LASTEXITCODE -eq 0)
}

# Extracts the version from a versioned 'replace' directive, e.g.
#   replace nvm/preferences v1.2.3 => ../shared/preferences
# Returns $null if not found.
function Get-ModuleVersion {
    param([string]$Content, [string]$Module)
    $escaped = [regex]::Escape($Module)
    $m = [regex]::Match($Content, "(?m)^\s*replace\s+$escaped\s+(v\d+\.\d+\.\d+\S*)")
    if ($m.Success) { return $m.Groups[1].Value }
    return $null
}

function Coalesce($a, $b) { if ($null -ne $a) { $a } else { $b } }

# ── Gather staged files ───────────────────────────────────────────────────────

$staged = @(git diff --cached --name-only 2>$null)
if ($staged.Count -eq 0) { exit 0 }

# Normalise to forward slashes for consistent matching
$staged = $staged | ForEach-Object { $_.Replace('\', '/') }

$repoRoot = (git rev-parse --show-toplevel).Trim().Replace('\', '/')

# ── Discover shared modules ───────────────────────────────────────────────────
# Scans every go.mod under shared/ and maps relative-dir -> module name.

$shared = @{}  # e.g. "shared/preferences" => "nvm/preferences"

Get-ChildItem -Path "$repoRoot/shared" -Filter 'go.mod' -Recurse -ErrorAction SilentlyContinue |
    ForEach-Object {
        $content = Get-Content $_.FullName -Raw
        if ($content -match '(?m)^module\s+(\S+)') {
            $relDir = $_.DirectoryName.Replace('\', '/') -replace "^$([regex]::Escape($repoRoot))/", ''
            $shared[$relDir] = $Matches[1]
        }
    }

if ($shared.Count -eq 0) { exit 0 }

# ── Discover consumer go.mod files ───────────────────────────────────────────
# A consumer has a *versioned* replace directive:  replace <mod> vX.Y.Z => ...
# Unversioned replacements (shared/settings/go.mod style) are intentionally
# excluded — they are the definition, not the reference.

$consumers = @{}  # relPath => [string[]] consumed module names

Get-ChildItem -Path $repoRoot -Filter 'go.mod' -Recurse -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -notlike '*\.git*' } |
    ForEach-Object {
        $content   = Get-Content $_.FullName -Raw
        $relPath   = $_.FullName.Replace('\', '/') -replace "^$([regex]::Escape($repoRoot))/", ''
        foreach ($dir in $shared.Keys) {
            $module  = $shared[$dir]
            $escaped = [regex]::Escape($module)
            if ($content -match "(?m)^\s*replace\s+$escaped\s+v\d+") {
                if (-not $consumers.ContainsKey($relPath)) { $consumers[$relPath] = [System.Collections.Generic.List[string]]::new() }
                $consumers[$relPath].Add($module)
            }
        }
    }

if ($consumers.Count -eq 0) { exit 0 }

# ── Check for missing version bumps ──────────────────────────────────────────

$warnings = [System.Collections.Generic.List[string]]::new()
$hasHead  = HasHead

foreach ($dir in $shared.Keys) {
    $module = $shared[$dir]

    # Skip if no staged files are inside this shared module directory.
    $affected = $staged | Where-Object { $_ -like "$dir/*" }
    if (-not $affected) { continue }

    foreach ($gomod in $consumers.Keys) {
        if ($consumers[$gomod] -notcontains $module) { continue }

        if ($staged -contains $gomod) {
            # go.mod IS staged — verify the version actually changed.
            $stagedContent = (git show ":$gomod" 2>$null) -join "`n"
            $stagedVer     = Get-ModuleVersion $stagedContent $module

            $headVer = $null
            if ($hasHead) {
                $headContent = (git show "HEAD:$gomod" 2>$null) -join "`n"
                $headVer     = Get-ModuleVersion $headContent $module
            }

            if ($stagedVer -eq $headVer) {
                $disp = Coalesce $headVer 'none'
                $warnings.Add("  [$module]  staged changes detected but version in '$gomod' was not bumped (still $disp)")
            }
        } else {
            # go.mod NOT staged — it wasn't updated at all.
            $currentVer = $null
            if ($hasHead) {
                $headContent = (git show "HEAD:$gomod" 2>$null) -join "`n"
                $currentVer  = Get-ModuleVersion $headContent $module
            } else {
                $fileContent = Get-Content "$repoRoot/$gomod" -Raw -ErrorAction SilentlyContinue
                $currentVer  = Get-ModuleVersion $fileContent $module
            }
            $disp = Coalesce $currentVer 'none'
            $warnings.Add("  [$module]  staged changes detected but '$gomod' was not updated (current: $disp)")
        }
    }
}

# ── Output ────────────────────────────────────────────────────────────────────

if ($warnings.Count -gt 0) {
    Write-Host ""
    Write-Host "pre-commit: shared module version bump required (SBOM compliance)"
    Write-Host "----------------------------------------------------------------"
    Write-Host "Staged changes were found in shared Go modules, but the versioned"
    Write-Host "'replace' directive(s) in the consumer go.mod file(s) were not bumped:"
    Write-Host ""
    $warnings | ForEach-Object { Write-Host $_ }
    Write-Host ""
    Write-Host "  TO FIX:"
    Write-Host "    1. Bump the version in the 'replace' directive of the consumer go.mod."
    Write-Host "       Example: replace nvm/preferences v1.0.1 => ../shared/preferences"
    Write-Host "    2. Also bump the matching 'require' line to the same version."
    Write-Host "    3. Re-stage the go.mod:  git add <go.mod path>"
    Write-Host "    4. Re-run your commit."
    Write-Host ""
    Write-Host "  TO UNSTAGE only the shared module changes (other staged files untouched):"
    if ($hasHead) {
        Write-Host "    git restore --staged shared/"
    } else {
        Write-Host "    git rm --cached -r shared/    # first commit only (no HEAD yet)"
    }
    Write-Host ""
    Write-Host "  To skip this check for one commit:  git commit --no-verify"
    Write-Host ""
    exit 1
}

exit 0

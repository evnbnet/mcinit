
#Requires -Version 5.1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Repo    = 'evnbnet/mcinit'
$Root    = Join-Path $env:LOCALAPPDATA 'mcinit'
$BinDir  = Join-Path $Root 'bin'

$Accent = if ($Host.UI.SupportsVirtualTerminal) { "$([char]27)[38;2;125;86;244m" } else { '' }
$Dim    = if ($Host.UI.SupportsVirtualTerminal) { "$([char]27)[90m" } else { '' }
$Green  = if ($Host.UI.SupportsVirtualTerminal) { "$([char]27)[38;2;4;181;117m" } else { '' }
$Reset  = if ($Host.UI.SupportsVirtualTerminal) { "$([char]27)[0m" } else { '' }

function Write-Step { param([string]$Message) Write-Host "  $Green+$Reset $Message" }
function Write-Note { param([string]$Message) Write-Host "    $Dim$Message$Reset" }

function Get-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
    }
}

Write-Host ''
Write-Host "  ${Accent}mcinit${Reset} installer"
Write-Host ''

$arch = Get-Arch

Write-Step 'Looking up the latest release'
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{
    'User-Agent' = 'mcinit-installer'
    'Accept'     = 'application/vnd.github+json'
}
$tag     = $release.tag_name
$version = $tag.TrimStart('v')
$asset   = "mcinit_${version}_windows_${arch}.zip"

$download = ($release.assets | Where-Object { $_.name -eq $asset } | Select-Object -First 1).browser_download_url
if (-not $download) {
    throw "Release $tag does not include $asset"
}
Write-Note "$tag · $arch"

$temp = Join-Path ([System.IO.Path]::GetTempPath()) ("mcinit-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp -Force | Out-Null

try {
    $zip = Join-Path $temp $asset
    Write-Step "Downloading $asset"
    $previousProgress = $ProgressPreference
    $ProgressPreference = 'SilentlyContinue'
    try {
        Invoke-WebRequest -Uri $download -OutFile $zip -UseBasicParsing
    } finally {
        $ProgressPreference = $previousProgress
    }

    $checksums = ($release.assets | Where-Object { $_.name -eq 'checksums.txt' } | Select-Object -First 1).browser_download_url
    if ($checksums) {
        $expected = ((Invoke-WebRequest -Uri $checksums -UseBasicParsing).Content -split "`n" |
            Where-Object { $_ -match [regex]::Escape($asset) } |
            Select-Object -First 1) -split '\s+' | Select-Object -First 1
        $actual = (Get-FileHash -Path $zip -Algorithm SHA256).Hash.ToLower()
        if ($expected -and $actual -ne $expected.ToLower()) {
            throw "SHA256 mismatch for $asset (expected $expected, got $actual)"
        }
        Write-Step 'SHA256 verified'
    } else {
        Write-Note 'This release has no checksums.txt, so the download could not be verified'
    }

    Write-Step "Installing into $BinDir"
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    Expand-Archive -Path $zip -DestinationPath $temp -Force

    $exe = Get-ChildItem -Path $temp -Filter 'mcinit.exe' -Recurse | Select-Object -First 1
    if (-not $exe) { throw 'The zip does not contain mcinit.exe' }
    Copy-Item -Path $exe.FullName -Destination (Join-Path $BinDir 'mcinit.exe') -Force
} finally {
    Remove-Item -Path $temp -Recurse -Force -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($null -eq $userPath) { $userPath = '' }
$alreadyOnPath = @($userPath -split ';' | Where-Object { $_.TrimEnd('\') -ieq $BinDir.TrimEnd('\') }).Count -gt 0

if ($alreadyOnPath) {
    Write-Step 'Already on the PATH'
} else {
    $updated = if ([string]::IsNullOrWhiteSpace($userPath)) { $BinDir } else { "$($userPath.TrimEnd(';'));$BinDir" }
    [Environment]::SetEnvironmentVariable('Path', $updated, 'User')
    $env:Path = "$env:Path;$BinDir"
    Write-Step 'Added to the user PATH'
}

Write-Host ''
Write-Host "  ${Accent}Done${Reset} $Dim->$Reset ${Accent}mcinit new my-server${Reset}"
Write-Host ''
if (-not $alreadyOnPath) {
    Write-Note 'If another terminal cannot find it, close and reopen that terminal.'
    Write-Host ''
}

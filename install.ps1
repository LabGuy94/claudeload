param(
  [string]$Version = "latest",
  [string]$InstallDir = "$env:LOCALAPPDATA\claudeload\bin",
  [switch]$AddToPath
)

$ErrorActionPreference = "Stop"

function Fail($msg) {
  Write-Error $msg
  exit 1
}

function Get-OsArch {
  $arch = $env:PROCESSOR_ARCHITECTURE
  switch ($arch) {
    "AMD64" { return "windows_amd64" }
    "ARM64" { return "windows_arm64" }
    default { Fail "Unsupported architecture: $arch" }
  }
}

function Get-ReleaseInfo {
  $api = "https://api.github.com/repos/LabGuy94/claudeload/releases"
  if ($Version -eq "latest") {
    $url = "$api/latest"
  } else {
    $tag = $Version
    if ($tag -notmatch '^v') { $tag = "v$tag" }
    $url = "$api/tags/$tag"
  }
  $resp = Invoke-RestMethod -Uri $url -Headers @{ "User-Agent" = "claudeload-installer" }
  return $resp
}

function Download-Asset($release, $assetName, $dest) {
  $asset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
  if (-not $asset) { Fail "Asset not found: $assetName" }
  Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $dest
}

function Ensure-Dir($dir) {
  if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }
}

function Add-PathUser($dir) {
  $current = [Environment]::GetEnvironmentVariable("Path", "User")
  if ($current -and $current.Split(";") -contains $dir) { return }
  $newPath = if ($current) { "$current;$dir" } else { $dir }
  [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
}

$osarch = Get-OsArch
$release = Get-ReleaseInfo

$ver = $release.tag_name
$verNoV = $ver -replace '^v', ''
$archive = "claudeload_${verNoV}_${osarch}.zip"

$temp = New-Item -ItemType Directory -Path ([IO.Path]::Combine([IO.Path]::GetTempPath(), "claudeload-install")) -Force
$zipPath = Join-Path $temp.FullName $archive

Write-Host "Downloading $archive..."
Download-Asset $release $archive $zipPath

Ensure-Dir $InstallDir

Write-Host "Extracting to $InstallDir..."
Expand-Archive -Path $zipPath -DestinationPath $InstallDir -Force

$bin = Join-Path $InstallDir "claudeload.exe"
if (-not (Test-Path $bin)) { Fail "claudeload.exe not found after extraction." }

if ($AddToPath) {
  Add-PathUser $InstallDir
  Write-Host "Added $InstallDir to user PATH. Open a new terminal to use claudeload."
} else {
  Write-Host "Install complete. Run: $bin"
}

Write-Host "Version: $ver"

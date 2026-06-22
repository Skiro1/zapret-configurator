param(
    [string]$Mode = "standard"
)

$ErrorActionPreference = 'Stop'
$rootDir = Split-Path -Parent $PSScriptRoot
$utilsDir = Join-Path $PSScriptRoot 'utils'
$resultsDir = Join-Path $utilsDir 'test results'
if (-not (Test-Path $resultsDir)) { New-Item -ItemType Directory -Path $resultsDir | Out-Null }

function Get-TargetsFromFile {
    $file = Join-Path $utilsDir 'targets.txt'
    if (-not (Test-Path $file)) {
        return @(
            [pscustomobject]@{ Name='Discord'; Url='https://discord.com'; Ping='discord.com' },
            [pscustomobject]@{ Name='YouTube'; Url='https://www.youtube.com'; Ping='youtube.com' },
            [pscustomobject]@{ Name='Google'; Url='https://www.google.com'; Ping='google.com' },
            [pscustomobject]@{ Name='Cloudflare'; Url='https://www.cloudflare.com'; Ping='cloudflare.com' }
        )
    }

    $targets = @()
    foreach ($line in Get-Content $file) {
        $t = $line.Trim()
        if (-not $t -or $t.StartsWith('#')) { continue }
        if ($t -notmatch '^(?<name>[A-Za-z0-9_]+)\s*=\s*"(?<value>.+)"\s*$') { continue }
        $value = $Matches.value
        if ($value -like 'PING:*') {
            $host = $value.Substring(5).Trim()
            $targets += [pscustomobject]@{ Name=$Matches.name; Url=$null; Ping=$host }
        } else {
            $host = ($value -replace '^https?://','' -replace '/.*$','')
            $targets += [pscustomobject]@{ Name=$Matches.name; Url=$value; Ping=$host }
        }
    }
    return $targets
}

function Test-Config {
    param(
        [string]$BatPath,
        [array]$Targets,
        [switch]$UseDpi
    )

    $result = [ordered]@{ Config = [IO.Path]::GetFileName($BatPath); Pass = 0; Fail = 0; Detail = @() }
    $p = Start-Process -FilePath cmd.exe -ArgumentList '/c', $BatPath -WorkingDirectory (Split-Path $BatPath) -WindowStyle Hidden -PassThru
    Start-Sleep -Seconds 5

    foreach ($t in $Targets) {
        if ($UseDpi) {
            $payload = [byte[]]::new(16384)
            [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($payload)
            $tmp = New-TemporaryFile
            [IO.File]::WriteAllBytes($tmp, $payload)
            try {
                $args = @('--range','0-16383','-m','5','-s','-X','POST','--data-binary',"@$tmp",'https://'+$t.Ping)
                $out = & curl.exe @args 2>&1
                if ($LASTEXITCODE -eq 0) { $result.Pass++ } else { $result.Fail++ }
                $result.Detail += "DPI $($t.Name): $LASTEXITCODE"
            } finally {
                Remove-Item $tmp -ErrorAction SilentlyContinue
            }
            continue
        }

        try {
            $http = & curl.exe --http1.1 -m 5 -o NUL -s -w '%{http_code}' $t.Url 2>$null
            $tls12 = & curl.exe --tlsv1.2 --tls-max 1.2 -m 5 -o NUL -s -w '%{http_code}' $t.Url 2>$null
            $tls13 = & curl.exe --tlsv1.3 --tls-max 1.3 -m 5 -o NUL -s -w '%{http_code}' $t.Url 2>$null
            if (($http -match '^[23]\d\d$') -or ($tls12 -match '^[23]\d\d$') -or ($tls13 -match '^[23]\d\d$')) {
                $result.Pass++
            } else {
                $result.Fail++
            }
            if ($t.Ping) { Test-Connection -ComputerName $t.Ping -Count 1 -Quiet | Out-Null }
            $result.Detail += "$($t.Name): http=$http tls12=$tls12 tls13=$tls13"
        } catch {
            $result.Fail++
            $result.Detail += "$($t.Name): FAIL"
        }
    }

    try { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue } catch {}
    return [pscustomobject]$result
}

if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host 'Run PowerShell as Administrator.' -ForegroundColor Red
    exit 1
}

if (-not (Get-Command curl.exe -ErrorAction SilentlyContinue)) {
    Write-Host 'curl.exe not found.' -ForegroundColor Red
    exit 1
}

if (Get-Service -Name 'zapret2' -ErrorAction SilentlyContinue) {
    Write-Host 'zapret2 service is installed. Remove it before testing.' -ForegroundColor Yellow
}

$targets = Get-TargetsFromFile
$batFiles = Get-ChildItem -LiteralPath $rootDir -Filter '*.bat' | Where-Object { $_.Name -notlike 'service*' } | Sort-Object Name
if (-not $batFiles) {
    Write-Host 'No config .bat files found.' -ForegroundColor Yellow
    exit 1
}

$selected = $batFiles
if ($Mode -eq 'select') {
    for ($i = 0; $i -lt $batFiles.Count; $i++) { Write-Host ('[{0}] {1}' -f ($i + 1), $batFiles[$i].Name) }
    $choice = Read-Host 'Enter numbers separated by comma'
    $idx = @()
    foreach ($part in ($choice -split ',')) {
        $n = 0
        if ([int]::TryParse($part.Trim(), [ref]$n) -and $n -ge 1 -and $n -le $batFiles.Count) { $idx += ($n - 1) }
    }
    if ($idx.Count -gt 0) { $selected = $batFiles[$idx] }
}

$useDpi = $Mode -eq 'dpi'
$stamp = Get-Date -Format 'yyyyMMdd_HHmmss'
$reportFile = Join-Path $resultsDir "zapret2_$stamp.txt"

"Mode: $Mode" | Out-File $reportFile -Encoding UTF8
foreach ($bat in $selected) {
    $r = Test-Config -BatPath $bat.FullName -Targets $targets -UseDpi:$useDpi
    "[$($r.Config)] pass=$($r.Pass) fail=$($r.Fail)" | Out-File $reportFile -Append -Encoding UTF8
    foreach ($d in $r.Detail) { "  $d" | Out-File $reportFile -Append -Encoding UTF8 }
}

Write-Host "Report saved to $reportFile" -ForegroundColor Green

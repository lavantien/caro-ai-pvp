$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent | Split-Path -Parent
$sidecars = Get-ChildItem $PSScriptRoot -Filter 'allow-*.txt' | Sort-Object Name
$existing = Get-Content (Join-Path $root '.mutate-allow')
$seen = @{}
foreach ($e in $existing) {
    $key = ($e -split ' # ')[0].Trim()
    $seen[$key] = $true
}
$out = New-Object System.Collections.Generic.List[string]
$fail = 0
foreach ($f in $sidecars) {
    foreach ($raw in Get-Content $f.FullName) {
        $line = $raw.TrimEnd()
        if ($line -eq '') { continue }
        if ($line -notmatch '^(\S+?):(\d+):(\d+) (.+?) # (.+)$') {
            Write-Output "PARSE-FAIL $($f.Name): $line"; $fail++; continue
        }
        $rel = $Matches[1]; $ln = [int]$Matches[2]; $col = [int]$Matches[3]
        $desc = $Matches[4]; $proof = $Matches[5]
        $shift = 0
        if ($rel -eq 'internal/server/ponder.go' -and $ln -ge 238) { $shift = 1 }
        if ($rel -eq 'internal/server/httpapi.go') { $shift = -3 }
        $ln2 = $ln + $shift
        $src = Get-Content (Join-Path $root ($rel -replace '/', '\')) -ErrorAction SilentlyContinue
        if (-not $src) { Write-Output "NOFILE ${rel}"; $fail++; continue }
        $text = $src[$ln2 - 1]
        if ($null -eq $text) { Write-Output "NOLINE ${rel}:${ln2} (was ${ln})"; $fail++; continue }
        if ($desc -match '^replace (.+?) with ') {
            $tok = $Matches[1]
            if ($col - 1 -ge $text.Length) {
                Write-Output "COL-OVERRUN ${rel}:${ln2}:${col} line='$($text.Trim())'"
                $fail++; continue
            }
            $got = $text.Substring($col - 1, [Math]::Min($tok.Length, $text.Length - $col + 1))
            if ($got -ne $tok) {
                Write-Output "TOKEN-MISMATCH ${rel}:${ln2}:${col} want '$tok' got '$got' line='$($text.Trim())'"
                $fail++; continue
            }
        }
        $key = "${rel}:${ln2}:${col} ${desc}"
        if ($seen.ContainsKey($key)) { Write-Output "DUP $key"; $fail++; continue }
        $seen[$key] = $true
        $out.Add("${key} # ${proof}")
    }
}
Write-Output "verified entries: $($out.Count), failures: $fail"
if ($fail -eq 0) {
    $newEntry = 'internal/server/store.go:146:6 negate condition # equivalent: under modernc.org/sqlite the TRUNCATE checkpoint reports busy through its result row and never errors Exec, so ckErr is nil at the gate, and no public-seam input makes the pool Close return non-nil either (a held Conn or Tx marks the pool dead without erroring Close), so both branches store the same value in s.closeErr for every reachable input; the busy path is pinned by TestCloseKeepsCheckpointErrorWhenPoolCloseSucceeds'
    $text = (Get-Content (Join-Path $root 'internal\server\store.go'))[145]
    $condCol = $text.IndexOf('err == nil') + 1
    if ($condCol -ne 6) { Write-Output "STORE-COORD mismatch cond at col $condCol"; exit 1 }
    if ($seen.ContainsKey('internal/server/store.go:146:6 negate condition')) { Write-Output 'DUP store entry'; exit 1 }
    $out.Add($newEntry)
    Add-Content (Join-Path $root '.mutate-allow') ''
    Add-Content (Join-Path $root '.mutate-allow') $out
    Write-Output "appended $($out.Count) entries to .mutate-allow"
}

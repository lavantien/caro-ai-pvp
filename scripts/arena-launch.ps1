function Invoke-ArenaProcess {
	param(
		[string]$FilePath,
		[string[]]$ArgumentList,
		[string]$WorkingDirectory,
		[string]$OutPath,
		[string]$ErrPath
	)
	$out = [System.IO.StreamWriter]::new($OutPath, $false)
	$err = [System.IO.StreamWriter]::new($ErrPath, $false)
	$proc = $null
	$outJob = $null
	$errJob = $null
	try {
		$psi = [System.Diagnostics.ProcessStartInfo]::new()
		$psi.FileName = $FilePath
		foreach ($a in $ArgumentList) { [void]$psi.ArgumentList.Add($a) }
		$psi.WorkingDirectory = $WorkingDirectory
		$psi.UseShellExecute = $false
		$psi.RedirectStandardOutput = $true
		$psi.RedirectStandardError = $true
		$psi.CreateNoWindow = $true
		$proc = [System.Diagnostics.Process]::new()
		$proc.StartInfo = $psi
		if (-not $proc.Start()) { throw "arena launch failed: $FilePath" }
		$outJob = Start-ThreadJob {
			param($p, $w)
			$w.AutoFlush = $true
			try { while ($null -ne ($l = $p.StandardOutput.ReadLine())) { $w.WriteLine($l) } }
			finally { $w.Dispose() }
		} -ArgumentList $proc, $out
		$errJob = Start-ThreadJob {
			param($p, $w)
			$w.AutoFlush = $true
			try { while ($null -ne ($l = $p.StandardError.ReadLine())) { $w.WriteLine($l) } }
			finally { $w.Dispose() }
		} -ArgumentList $proc, $err
		$proc.WaitForExit()
		$outJob, $errJob | Wait-Job | Out-Null
		foreach ($j in @($outJob, $errJob)) {
			if ($j.State -eq 'Failed') { throw "arena relay reader failed: $($j.ChildJobs[0].JobStateInfo.Reason)" }
		}
		return $proc.ExitCode
	} finally {
		if ($null -ne $outJob) { $outJob | Remove-Job -Force -Confirm:$false }
		if ($null -ne $errJob) { $errJob | Remove-Job -Force -Confirm:$false }
		if ($null -ne $proc) { $proc.Dispose() }
		$out.Dispose()
		$err.Dispose()
	}
}

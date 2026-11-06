function Invoke-ArenaProcess {
	param(
		[string]$FilePath,
		[string[]]$ArgumentList,
		[string]$WorkingDirectory,
		[string]$OutPath,
		[string]$ErrPath
	)
	$cmd = '"' + $FilePath + '"'
	foreach ($a in $ArgumentList) { $cmd += ' "' + $a + '"' }
	$cmd += ' > "' + $OutPath + '" 2> "' + $ErrPath + '"'
	$psi = [System.Diagnostics.ProcessStartInfo]::new()
	$psi.FileName = $env:ComSpec
	$psi.Arguments = '/s /c "' + $cmd + '"'
	$psi.WorkingDirectory = $WorkingDirectory
	$psi.UseShellExecute = $false
	$psi.CreateNoWindow = $true
	$proc = [System.Diagnostics.Process]::new()
	$proc.StartInfo = $psi
	if (-not $proc.Start()) { throw "arena launch failed: $FilePath" }
	$proc.WaitForExit()
	$code = $proc.ExitCode
	$proc.Dispose()
	return $code
}

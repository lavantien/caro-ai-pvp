# Manages the Windows Firewall inbound allow rules for the caro server ports.
# Needs an elevated shell (run as administrator): make firewall
# Usage: pwsh -File scripts/firewall.ps1 -Ports 38063,38069 [-Remove]

param(
	[Parameter(Mandatory = $true)]
	[int[]]$Ports,
	[switch]$Remove
)

$names = @('CaroAI-HTTP', 'CaroAI-Debug')
if ($Ports.Count -ne $names.Count) {
	throw "expected $($names.Count) ports (http, debug), got $($Ports.Count)"
}

for ($i = 0; $i -lt $names.Count; $i++) {
	$name = $names[$i]
	netsh advfirewall firewall delete rule name="$name" 2>&1 | Out-Null
	if ($Remove) {
		Write-Host "removed $name"
		continue
	}
	netsh advfirewall firewall add rule name="$name" dir=in action=allow protocol=TCP localport="$($Ports[$i])" 2>&1 | Out-Null
	if ($LASTEXITCODE -ne 0) {
		throw "netsh failed for $name (exit $LASTEXITCODE), is the shell elevated?"
	}
	Write-Host "added $name tcp/$($Ports[$i])"
}

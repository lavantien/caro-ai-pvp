#!/usr/bin/env bash
# Writes a shields.io endpoint JSON badge file.
# Usage: coverage-badge.sh <label> <percentage> <output-file>
set -euo pipefail

label="$1"
pct="$2"
outfile="$3"

color=$(awk "BEGIN{
	if($pct>=90)print \"#673ab7\";
	else if($pct>=80)print \"#00bcd4\";
	else if($pct>=70)print \"green\";
	else if($pct>=60)print \"orange\";
	else if($pct>=50)print \"yellow\";
	else if($pct>=40)print \"red\";
	else if($pct>=30)print \"#e32636\";
	else if($pct>=20)print \"#b31b1b\";
	else print \"#7a0016\"
}")

mkdir -p "$(dirname "$outfile")"
printf '{"schemaVersion":1,"label":"%s","message":"%s%%","color":"%s"}\n' \
	"$label" "$pct" "$color" > "$outfile"
echo "$label coverage: $pct%"

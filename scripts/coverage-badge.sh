#!/usr/bin/env bash
# Writes a shields.io endpoint JSON badge file.
# Usage: coverage-badge.sh <label> <percentage> <output-file>
set -euo pipefail

label="$1"
pct="$2"
outfile="$3"

color=$(awk "BEGIN{if($pct>=80)print \"brightgreen\";else if($pct>=60)print \"yellow\";else print \"red\"}")

mkdir -p "$(dirname "$outfile")"
printf '{"schemaVersion":1,"label":"%s","message":"%s%%","color":"%s"}\n' \
	"$label" "$pct" "$color" > "$outfile"
echo "$label coverage: $pct%"

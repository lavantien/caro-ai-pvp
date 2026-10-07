# v0.23 master gate preregistration

Question: does the master tier (8 threads, 2 GiB table, VCF plus VCT) clear the ladder top outside the noise band, and does capability monotonicity hold at the top? Registered before any gate game runs, per the inversion research law.

## seats

master (8 cores, 2 GiB, VCF plus VCT, the new tier), master-novct (same shape with VCT stripped), hard (4 cores, 128 MiB, shipped top). All seats wire through server.NewBotSearcher. Alternating turns mean at most one seat searches at any instant, so the live thread peak per game is the seat max (8), inside MachineCores.

## schedule

4 pairs, 20 games each, red alternating per game so each seat holds red 10 times, one detached chain serialized as the machine's only engine-timing batch:

1. master vs hard at tc 1 (2+1)
2. master vs hard at tc 2 (3+2)
3. master vs master at tc 2 (the identical-engine floor that calibrates this band)
4. master vs master-novct at tc 2 (capability monotonicity at the top)

## reference bands

Identical-engine floors recorded 0 blue-seat wins at both 1+0 (40 of 40 red) and 10+5 (18 red, 2 draws, 0 blue over 20 games), so between equals the first mover decides. Pair 3 measures the same band at 3+2 before any comparison is read.

## decision rules, fixed in advance

1. Master clears the ladder top only if it beats hard outside the floor band at both 2+1 and 3+2: at least 13 of 20 overall and at least 3 blue-seat wins (equals produce about 0 blue wins; a blue-seat win is the strength signature no coin flip mints).
2. If the pair 3 floor shows blue-seat wins between equals at 3+2, rule 1's blue-win clause is read against that floor's own blue-win rate instead of the about-zero prior.
3. Capability monotonicity at the top: master must sit at or above master-novct. A master-novct win outside the floor band is an anomaly that blocks the tag and routes to investigation (the v0.20 matrix measured solver passes neutral to positive; a top inversion contradicts it).
4. A rule 1 failure means 8 threads plus a 2 GiB table does not separate above hard: record the miss, block the tier, and re-derive before shipping (the reserve-wave trigger).

## outputs

Raw pair logs under logs/archive/arena-v023-p*.log (local per the logs law), the merged table and verdict under playground/arena/out/v023-master-table.md (committed) once the chain completes.

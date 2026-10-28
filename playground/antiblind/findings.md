# Anti blindness rnd findings

Preregistration at topic-map.md, questions Q1 to Q5. All measurements from the settled forensic dataset and the replay harness in replay/, outputs under out/. Probes ran against HEAD 4939c52, the part A fail-memo stamp fix. One incidental defect found first, see Q4b.

## Q1 dataset, do flip claims differ from non flip claims

Master mate-for claim moves pooled: flip claims n 18, tt hit 14.7 permille, hashfull 2.4, fh1 50.0, depth 5.56, nodes 9.38m, ponder share 0.167. Non flip claims n 1513, tt hit 10.3, hashfull 7.0, fh1 40.7, depth 5.77, nodes 13.14m, ponder share 0.087. Flip claims carry higher table hits on emptier current generation tables, the hits come from older generation entries, and near double the ponder adoption share. Every square arm flip claim in the dataset sits on the master blue seat, 17 of 17. Artifact out/q1_flips_vs_nonflips.csv.

## Q2 does the blind claim reproduce, and how does it move with table size and warmth

Cold replay at the 18 flip positions, 300 ms budget, depth cap 8, table 0 to 768 MiB, threads 1 to 6: blind share 0.00 in every configuration, 198 rows. A cold engine never reproduces the mate-for claim, at any table size, even with more nodes than the recorded in-game claim depth. Warm replay, one carried engine per game at 120 ms per move, master shape 6 threads 768 MiB: 9 of 16 games reproduce the mate-for claim at the flip position, warm depth 3 to 6, tt hit 89 to 387 permille, hashfull 0 to 5. The claim is warmth produced, it exists only on the carried table. Blindness is monotone in warmth, not in table size. Artifacts out/replay_config.csv, out/replay_warm.csv.

## Q3 thread count alone at fixed table

No configuration goes blind cold, 1 thread or 6. Threads change completed depth at fixed budget, 4.94 to 6.00 average, and change warm hit rates, but the claim itself is warmth fed, threads only modulate how fast the warm claim is reached. Artifact out/replay_config.csv.

## Q4 the shared TT semantics and where deep entries die

From internal/engine/tt.go: store at line 153 keeps a same generation entry only when it is deeper, line 162, an old generation entry is always replaced. Probe at line 110 accepts any entry whose stored depth covers the probed depth, line 121, with no generation and no verification. bumpGen at line 101 runs once per search and once per ponder at smp.go setupJob line 287. So a mate-band entry written by a shallow completed iteration survives every later search as an old generation hit until some new write lands on its slot, and shallow depth 1 and 2 writes of each new search freely evict deep old entries. runWorker at smp.go:393 halts the root loop at the first mate-for iteration, so shallow mate entries are written by design. The measured chain: shallow mate-for completion writes a mate entry, the next search probes it as a bound cutoff from an older generation, the claim reappears warm, 9 of 16, and is refuted by a cold opponent solver in 5 of those 9, opponent VCF in 3 plies against claimed mate distances of 5 to 8 in every proven case. The surviving defect class is search side mate entry trust, not solver side staleness: the warm solver found every kill the cold solver found, 0 disagreements over 16 games of continuous instance reuse, the part A stamp fix holds under warmth.

Q4b incidental defect, hex log names. server.MLine at internal/server/stats.go:61 renders every M-line cell through geom.Default(), the square geometry, and playground/arena main.go:534 cellNameOf uses rules.AppendCellName, square only at internal/rules/coords.go:27. Every hex cell name in every arena log, move token and PV, is a square codec string, letter equals flat modulo 16, row equals flat over 16 plus 1, and never round trips through geom.ParseCell. The names are a bijective relabeling so no game data is lost, the forensic dataset inherits the codec, and the replay harness carries the decoder at replay/main.go nameToCell. The fix belongs to the parent, it touches server and arena files outside this lane.

## Q5 ranked additive measures

1. The part B refutation audit, already designed at playground/refaudit/design.md. It fires on exactly the 9 warm blind states, its parity rule out.Plies under the claimed distance demotes every one of the 5 proven wrong claims, the 4 unproven at cap stay claimed honestly. Cost one cold opponent solver pass per mate-for claim. Adopt, highest measured effect.
2. Old generation mate bound distrust, probe at tt.go refuses a cutoff from a mate-band score stored under an older generation and falls back to using the entry only as a move hint. This removes the warmth fuel the claims run on, it would have prevented all 9 warm blind states at source. Cost near zero, strength bearing, needs its own arena round. Adopt candidate.
3. Root halt defer, runWorker halts at smp.go:393 on the first mate-for iteration, one more iteration of verification before halting, or a solver check at the halt point, prevents the shallow mate writes that later poison the table. Additive compute only. Adopt candidate, folds into 1 and 2.
4. Solver instance warmth needs no measure, the stamp fix closed it, validated here.

No degradation measure studied or proposed anywhere in this campaign, varying table and threads served measurement only.

## Verdicts

Replay harness adopt, reusable bounded probe over recorded positions, replay/main.go. Dataset mining scripts adopt, extract_flips.py, mine_q1.py, mine_replay.py. Hex name decoder adopt inside the harness, the production fix routed to the parent as Q4b.

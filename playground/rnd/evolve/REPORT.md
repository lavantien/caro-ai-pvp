# evolve rnd report

Revisit of the evalfit differential evolution with modern technique, validated offline on synthetic objectives. The lab reimplements the shipped configuration exactly (rand/1/bin, F 0.7, CR 0.9, pop 48, clip bounds, accept on <=, forced crossover index) and layers switchable upgrades over it. evalfit itself is untouched; adoption is a follow-up.

## validation setup

Objectives: rastrigin and rosenbrock at D=6 for the cheap deterministic regime evalfit lives in, rosenbrockDec (decreasing-order cone, scored at the exact PAVA projection) for the constrained regime, and a game objective for the expensive noisy regime a future arena-driven weight evolution would face: true skill is a normalized rastrigin landscape, a game is one Bernoulli draw with win probability logistic in the skill gap, margins solved numerically over the bench seed set so the seed-averaged win probability of the true optimum equals the label (reference is uniform-random per seed, expected skill 0.599, solved margins 0.12267 / 0.49067 / 1.00995 for p = 0.55 / 0.70 / 0.90; the 0.9 solve lands above the analytic 0.998 because the probability clamp at 0.98 binds on high-gap seeds). LPSR is canonical worst-first removal with the incumbent always surviving. 21 seeds per cell, medians reported, targets: rastrigin 1e-6, rosenbrock 1.0, rosenbrockDec 1.1x the best of three 60k-eval reference runs, game loss 0.05 (skill 0.95). Game cells all spend the full 200k game budget, so comparisons are budget-equal.

## results

| objective | config | hits | median evals to hit | median best | avg total |
| --- | --- | --- | --- | --- | --- |
| rastrigin | base | 0/21 | 0 | 8.16132 | 19248 |
| rastrigin | shade | 21/21 | 8146 | 7.16e-07 | 8134 |
| rastrigin | shade+sur | 0/21 | 0 | 5.03e-06 | 9741 |
| rosenbrock | base | 21/21 | 6864 | 0.913574 | 6708 |
| rosenbrock | shade | 21/21 | 4813 | 0.951821 | 4745 |
| rosenbrock | shade+sur | 0/21 | 0 | 5.58306 | 2522 |
| rosenbrockDec | base | 21/21 | 384 | 0.00717 | 418 |
| rosenbrockDec | shade | 21/21 | 382 | 0.00181 | 421 |
| rosenbrockDec | shade+rep | 21/21 | 664 | 0.0074 | 714 |
| rosenbrockDec | shade+rep+sur | 17/21 | 615 | 0.00642 | 1406 |
| game55 | de+ref | 0/21 | 0 | 0.413416 | 200000 |
| game55 | shade+ref | 0/21 | 0 | 0.339805 | 200000 |
| game55 | shade+ref+crn | 0/21 | 0 | 0.276167 | 200000 |
| game55 | shade+bt | 0/21 | 0 | 0.301314 | 200000 |
| game55 | shade+btsprt | 1/21 | 97120 | 0.272203 | 195131 |
| game55 | shade+btsprt+crn | 0/21 | 0 | 0.227305 | 200048 |
| game55 | allon | 0/21 | 0 | 0.227305 | 200048 |
| game70 | de+ref | 0/21 | 0 | 0.272035 | 200000 |
| game70 | shade+ref | 0/21 | 0 | 0.207202 | 200000 |
| game70 | shade+ref+crn | 0/21 | 0 | 0.167623 | 200000 |
| game70 | shade+bt | 15/21 | 134720 | 0.0453666 | 149163 |
| game70 | shade+btsprt | 2/21 | 180983 | 0.106297 | 197642 |
| game70 | shade+btsprt+crn | 0/21 | 0 | 0.138199 | 200039 |
| game70 | allon | 0/21 | 0 | 0.138199 | 200039 |
| game90 | de+ref | 1/21 | 194880 | 0.167887 | 199756 |
| game90 | shade+ref | 4/21 | 47600 | 0.118748 | 174929 |
| game90 | shade+ref+crn | 5/21 | 45500 | 0.121319 | 168654 |
| game90 | shade+bt | 19/21 | 111680 | 0.0451849 | 117348 |
| game90 | shade+btsprt | 6/21 | 153302 | 0.0736061 | 184803 |
| game90 | shade+btsprt+crn | 2/21 | 200040 | 0.0986401 | 199868 |
| game90 | allon | 2/21 | 200040 | 0.0986401 | 199868 |

## verdicts

shade: adopt. Current-to-pbest/1 mutation with adaptive F/CR memories (Lehmer mean on successes), linear population size reduction, and a diversity-triggered restart around the incumbent. On rastrigin the shipped configuration stalls at the second ring (0/21 seeds past 8.16) while shade cracks the global optimum on every seed at a median 8146 evaluations. On rosenbrock it reaches the same target 30 percent cheaper (4813 vs 6864). Under game noise in reference mode it also beats the replica at equal budget (0.340 vs 0.413 final loss at p=0.55, 0.119 vs 0.168 at p=0.9). The 2 percent DE-vs-GD tolerance check in evalfit only gets easier to satisfy.

bradley-terry rank estimator: adopt for the game regime. Swiss-style round-robin pairings (4 opponents per individual, 20 games per pairing) with a Bradley-Terry strength fit driving rank selection. It is the only configuration that resolves the landscape at p=0.7 (15/21 at a median 134720 games) and it hits 19/21 at p=0.9 at a median 111680 games, where every reference-based configuration sits at 0 to 5/21. The noise floor at this 200k-game budget: at true p=0.55 nothing resolves for any configuration (0/21 across the board, best median loss 0.227), and p=0.7 resolves only through rank information. The floor sits between 0.55 and 0.7, not below 0.6 as the first pass claimed on mislabeled cells.

sprt early stop per pairing: drop. At p1=0.58 with 5 percent error bounds the test truncates close pairings before the strength fit can separate them, so it pays its savings back in lost ranking information: 6/21 vs 19/21 hits at p=0.9 and 2/21 vs 15/21 at p=0.7 against fixed pairings. Its only lead is the p=0.55 cell where nothing reliably hits target (1/21), which is not adoption evidence.

surrogate pre-screen: drop. A linear ridge on observed losses predicting whether a trial is worth evaluating is harmful in the deterministic regime (rosenbrock 0/21 with median 5.58 against 0.914 for the replica; rastrigin 0/21 against 21/21 for plain shade) because a linear model cannot represent the valley and the skip rule rejects good trials near the optimum. In rank mode it is inert by construction, so allon is byte-identical to shade+btsprt+crn. If surrogate screening is ever revisited it needs a nonlinear kernel and a regression on rank deltas, not a linear screen on raw losses.

constraint repair: drop. Exact PAVA projection of the genotype onto the feasible cone needs 1.7x the evaluations to hit target (664 vs 382). evalfit does not need it: the cumulative-exponential parameterization already keeps every genotype monotone by construction, which this experiment confirms is the better mechanism.

paired openings as common random numbers: drop. In reference mode it helps mildly (5/21 at a median 45500 vs 4/21 at 47600 at p=0.9) but it attaches each incumbent win to the specific luck it was measured under, and without incumbent re-evaluation that staleness is a winner-curse bias. In rank mode the shared luck stream correlates pairings and biases the Bradley-Terry fit: btsprt+crn reaches 0/21 and 2/21 where plain btsprt reaches 1/21 and 6/21. The budget it would save buys the re-evaluations it requires.

## integration sketch

For evalfit current cheap objective, adoption is a drop-in: replace the fitDE inner loop with the shade operators (the lab shade.go maps one-to-one onto evalfit evolve.go, same acceptance rule and bounds), keep GD primary with DE as challenger. Expected effect: the challenger stops stalling on any multimodal stretch of the aggregated loss and the 2 percent tolerance check keeps its meaning.

For a future arena-driven weight evolution over the 5 class weights plus scale, the adopted stack is shade plus the Bradley-Terry rank estimator with a small population. Budget anchored on the bench's own equal-budget cells rather than a generation extrapolation: median 112k games to skill 0.95 at p=0.9 and 135k at p=0.7, so a full run costs on the order of 110k to 150k games. A pop 8 configuration halves games per generation (about 960 at 3 opponents, 20 games per pairing) but not the total. At tc 1 self-play pace on this box that is on the order of 35 to 50 serialized nights, 4x the first pass's estimate, so a real run only makes sense with population, target skill, or pairing count cut, as a background ladder once the bookgen fleet and the v0.21 to v0.30 timing batches free the machine. No arena runs were made in this lane.

## artifacts

bench-out.txt holds the raw table of the final equal-budget run under the solved margins and canonical LPSR. Rebuild with go run . from this directory.

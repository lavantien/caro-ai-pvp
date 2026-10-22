# evolve rnd report

Revisit of the evalfit differential evolution with modern technique, validated offline on synthetic objectives. The lab reimplements the shipped configuration exactly (rand/1/bin, F 0.7, CR 0.9, pop 48, clip bounds, accept on <=, forced crossover index) and layers switchable upgrades over it. evalfit itself is untouched; adoption is a follow-up.

## validation setup

Objectives: rastrigin and rosenbrock at D=6 for the cheap deterministic regime evalfit lives in, rosenbrockDec (decreasing-order cone, scored at the exact PAVA projection) for the constrained regime, and a game objective for the expensive noisy regime a future arena-driven weight evolution would face: true skill is a normalized rastrigin landscape, a game is one Bernoulli draw with win probability logistic in the skill gap, and margins are tuned so the true optimum beats a fixed median-skill reference at p = 0.55, 0.7, 0.9. 21 seeds per cell, medians reported, targets: rastrigin 1e-6, rosenbrock 1.0, rosenbrockDec 1.1x the best of three 60k-eval reference runs, game loss 0.05 (skill 0.95). Game cells all spend the full 200k game budget, so comparisons are budget-equal.

## results

| objective | config | hits | median evals to hit | median best | avg total |
| --- | --- | --- | --- | --- | --- |
| rastrigin | base | 0/21 | 0 | 8.16 | 19248 |
| rastrigin | shade | 21/21 | 8304 | 8.19e-07 | 8283 |
| rastrigin | shade+sur | 3/21 | 7859 | 0.0054 | 9464 |
| rosenbrock | base | 21/21 | 6864 | 0.914 | 6708 |
| rosenbrock | shade | 21/21 | 4633 | 0.957 | 4795 |
| rosenbrock | shade+sur | 0/21 | 0 | 10.8 | 2518 |
| rosenbrockDec | base | 21/21 | 384 | 0.0072 | 418 |
| rosenbrockDec | shade | 21/21 | 382 | 0.0049 | 403 |
| rosenbrockDec | shade+rep | 21/21 | 757 | 0.0037 | 703 |
| rosenbrockDec | shade+rep+sur | 18/21 | 487 | 0.0065 | 871 |
| game55 | de+ref | 0/21 | 0 | 0.461 | 200000 |
| game55 | shade+ref | 0/21 | 0 | 0.391 | 200000 |
| game55 | shade+ref+crn | 0/21 | 0 | 0.287 | 200000 |
| game55 | shade+bt | 0/21 | 0 | 0.373 | 200003 |
| game55 | shade+btsprt | 0/21 | 0 | 0.243 | 200034 |
| game70 | de+ref | 0/21 | 0 | 0.311 | 200000 |
| game70 | shade+ref | 0/21 | 0 | 0.280 | 200000 |
| game70 | shade+ref+crn | 0/21 | 0 | 0.235 | 200000 |
| game70 | shade+bt | 13/21 | 123200 | 0.048 | 156845 |
| game70 | shade+btsprt | 0/21 | 0 | 0.141 | 200031 |
| game90 | de+ref | 0/21 | 0 | 0.192 | 200000 |
| game90 | shade+ref | 1/21 | 94660 | 0.160 | 194983 |
| game90 | shade+ref+crn | 1/21 | 89060 | 0.131 | 194717 |
| game90 | shade+bt | 20/21 | 115520 | 0.042 | 114057 |
| game90 | shade+btsprt | 3/21 | 166069 | 0.094 | 195109 |
| game90 | allon | 2/21 | 190442 | 0.095 | 197170 |

## verdicts

shade: adopt. Current-to-pbest/1 mutation with adaptive F/CR memories (Lehmer mean on successes), linear population size reduction, and a diversity-triggered restart around the incumbent. On rastrigin the shipped configuration stalls at the second ring (0/21 seeds past 8.16) while shade cracks the global optimum on every seed at a median 8304 evaluations. On rosenbrock it reaches the same target 32 percent cheaper (4633 vs 6864). Under game noise in reference mode it also beats the replica at equal budget (0.391 vs 0.461 final loss at p=0.55, 0.160 vs 0.192 at p=0.9). The 2 percent DE-vs-GD tolerance check in evalfit only gets easier to satisfy.

bradley-terry rank estimator: adopt for the game regime. Swiss-style round-robin pairings (4 opponents per individual, 20 games per pairing) with a Bradley-Terry strength fit driving rank selection. It is the only configuration that reaches skill 0.95 at all at p=0.7 (13/21) and it hits 20/21 at p=0.9 at a median 115k games, where every reference-based configuration is at 0 or 1/21. At the weakest margin nothing reaches target for any config, which is the honest statement of the noise floor: below roughly p=0.6 per game, 200k games do not resolve a 6-dim landscape.

sprt early stop per pairing: drop. At p1=0.58 with 5 percent error bounds the test truncates close pairings before the strength fit can separate them, so it pays its savings back in lost ranking information: 3/21 vs 20/21 hits at p=0.9 and 0/21 vs 13/21 at p=0.7 against fixed pairings. Its only lead is the p=0.55 cell where nothing hits target, which is not adoption evidence.

surrogate pre-screen: drop. A linear ridge on observed losses predicting whether a trial is worth evaluating is harmful in the deterministic regime (rosenbrock 0/21 with median 10.8 against 0.914 for the replica; rastrigin 3/21 against 21/21 for plain shade) because a linear model cannot represent the valley and the 75th-percentile skip rule rejects good trials near the optimum. In rank mode it is inert by construction (there is no scalar loss history to fit). The no-skip warmup guarantee worked as specified, but with 6 dims and cheap evaluations there is nothing to save. If surrogate screening is ever revisited it needs a nonlinear kernel and a regression on rank deltas, not a linear screen on raw losses.

constraint repair: drop. Exact PAVA projection of the genotype onto the feasible cone reaches a slightly better median best (0.0037 vs 0.0049) but needs 2x the evaluations to hit target (757 vs 382). evalfit does not need it: the cumulative-exponential parameterization already keeps every genotype monotone by construction, which this experiment confirms is the better mechanism.

paired openings as common random numbers: drop. In reference mode it helps mildly (0.131 vs 0.160 median loss at p=0.9) but it attaches each incumbent win to the specific luck it was measured under, and without incumbent re-evaluation that staleness is a winner-curse bias. In rank mode the shared luck stream correlates pairings and biases the Bradley-Terry fit. The budget it would save buys the re-evaluations it requires.

## integration sketch

For evalfit current cheap objective, adoption is a drop-in: replace fitDE inner loop with the shade operators (the lab shade.go maps one-to-one onto evalfit evolve.go, same acceptance rule and bounds), keep GD primary with DE as challenger. Expected effect: the challenger stops stalling on any multimodal stretch of the aggregated loss and the 2 percent tolerance check keeps its meaning.

For a future arena-driven weight evolution over the 5 class weights plus scale, the adopted stack is shade plus the Bradley-Terry rank estimator with a small population. Rough budget at pop 8, 3 opponents per individual, 20 games per pairing: about 960 games per generation (two ranking rounds), and a 6-dim near-smooth surface plausibly converges in 20 to 40 generations, so 20k to 40k games per evolution run. At tc 1 self-play pace on this box that is on the order of 10 serialized nights, which only makes sense as a background ladder once the bookgen fleet and the v0.21 to v0.30 timing batches free the machine. No arena runs were made in this lane.

## artifacts

bench-out.txt holds the raw table of the final equal-budget run. Rebuild with go run . from this directory.
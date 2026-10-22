# tune: bayesian optimization harness

Question: can sample-efficient tuning replace grid sweeps for this engine's expensive-objective surfaces, and does ant colony optimization earn a place. Answer: adopt the bayesian harness, it is the only method reaching every target and 5.5x faster than the best competitor finish. Keep grid only when the full lattice fits the budget. Drop ACO, it reaches no target at this budget.

## validation

Budget 120 evaluations, 20 seeds, grid rows are per-seed random-offset lattices, fine at 13 levels per continuous axis and coarse at the largest level count whose full lattice fits the budget (out/table.md):

- branin 0.498: bayes 22, grid(13)/grid-coarse(10)/random/aco never
- hartmann3 -3.7628: bayes 16, grid(13) 89, grid-coarse(4)/random/aco never
- noisyquadratic 0.3: bayes 14, all others never

Median best f: bayes 0.4381 / -3.8258 / 0.1162, grid(13) 0.6621 / -3.7885 / 38.65, grid-coarse 0.7674 / -3.5631 / 1.218, random 0.7820 / -3.6784 / 1.727, aco 2.4789 / -3.7379 / 0.518. Bayes is the only method reaching all three targets. On the single objective any competitor finishes, bayes is 5.5x faster (16 versus 89). What is proven about grid affordability: hartmann3's full 13-level lattice is 2197 cells against a 120 budget and clocktune fully enumerated its 108 cells, so grid fails on affordability as dims grow, no prefix failure occurred anywhere. ACO never reaches a target on any objective, its median best is mid-pack (behind grid(13) on branin and hartmann3, ahead on noisyquadratic).

## clock dry-run

Streaming suggest/observe over the real GameClock, worst drain-trajectory deviation at tc 10+5, dims Kp/Ki continuous plus Kd integer (out/demo.md). The 108-cell clocktune grid needs all 108 cells to find {Kp 0.06, Ki 0.03, Kd 0.01, 179797 ms}. Bayes reproduces that exact point and value at trial 22 of 40. The objective is monotone in Kp at increment controls, so this measures sample efficiency, not a new optimum. The optimum sits at the domain corner (Kp and Ki at their maxima), the last cell in a sequential grid's order, which maximizes the grid's disadvantage, so the 22-versus-108 gap is partly a corner artifact and the synthetic table carries the sample-efficiency claim.

## integration sketch

The arena loop swaps one closure:

    dims := []dim{{"kp", 0.01, 0.06, false}, {"ki", 0.005, 0.03, false}, {"kd", 0, 2, true}}
    o := newOptimizer(dims, seed)
    x := o.suggest(); f := arenaBatchOutcome(x); o.observe(x, f)

Persisting the observation list gives checkpoint/resume for multi-night runs.

## calibration notes

Two canonical-constant errors were caught and fixed: hartmann3's P matrix (8122 versus canonical 8828 in row 4, caught because bayes reported below the published minimum, verified against the SFU virtual library listing, pinned by brute force) and branin's c term (4/pi versus canonical 5/pi, caught in adversarial review, now pinned by a test asserting the 0.397887 minimum at the three canonical minima). Trial counts are load-immune, the bookgen fleet never competed for timing.

## verdicts

Adopt the bayesian harness for the next tuning campaign. Keep grid for budgets that cover the full lattice. Drop ACO.

# tune: bayesian optimization harness

Question: can sample-efficient tuning replace grid sweeps for this engine's expensive-objective surfaces, and does ant colony optimization earn a place. Answer: adopt the bayesian harness, expect 4-8x fewer evaluations than a grid sweep at equal outcome, keep grid only for small fully-affordable cell counts, drop ACO as dominated.

## validation

Budget 120 evaluations, 20 seeds, median trials-to-target (out/table.md):

- branin 0.498: bayes 25, grid/random/aco never
- hartmann3 -3.7628: bayes 16, grid 102, random/aco never
- noisyquadratic 0.3: bayes 14, all others never

Median best-f: bayes 0.4316 / -3.8258 / 0.1162 vs grid 2.13 / -3.79 / 44.9, random 0.66 / -3.68 / 1.66, aco 2.32 / -3.74 / 0.52. Bayes reaches every target 5-8x inside budget. Grid at partial budget samples only an odometer prefix, the exact failure mode clock sweeps hit when dims grow. ACO lands behind random on 2 of 3 objectives, its discretization caps it off optima.

## clock dry-run

Streaming suggest/observe over the real GameClock, worst drain-trajectory deviation at tc 10+5, dims Kp/Ki continuous plus Kd integer (out/demo.md). The 108-cell clocktune grid needs all 108 cells to find {Kp 0.06, Ki 0.03, Kd 0.01, 179797 ms}. Bayes reproduces that exact point and value at trial 22 of 40. The objective is monotone in Kp at increment controls, so this measures sample efficiency, not a new optimum.

## integration sketch

The arena loop swaps one closure:

    dims := []dim{{"kp", 0.01, 0.06, false}, {"ki", 0.005, 0.03, false}, {"kd", 0, 2, true}}
    o := newOptimizer(dims, seed)
    x := o.suggest(); f := arenaBatchOutcome(x); o.observe(x, f)

Persisting the observation list gives checkpoint/resume for multi-night runs.

## calibration notes

The hartmann3 P matrix was initially mistyped from memory (8122 vs canonical 8828 in row 4), caught because bayes reported below the published minimum, verified against the SFU virtual library listing, pinned by a brute-force check. Trial counts are load-immune, the bookgen fleet never competed for timing.

## verdicts

Adopt the bayes harness for the next tuning campaign. Keep grid for small fully-affordable cell counts. Drop ACO.

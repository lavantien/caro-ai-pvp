# tune lane validation

budget 120 trials per run, 20 seeds, median trials-to-target (>120 means never reached), median best value at budget

grid rows are per-seed random-offset lattices, fine at 13 levels per continuous axis and coarse at the largest level count whose full lattice fits the budget

| objective | target | method | median trials-to-target | median best f |
| --- | --- | --- | --- | --- |
| branin | 0.498 | bayes | 22 | 0.4381 |
| branin | 0.498 | grid(13) | >120 | 0.6621 |
| branin | 0.498 | grid-coarse(10) | >120 | 0.7674 |
| branin | 0.498 | random | >120 | 0.7820 |
| branin | 0.498 | aco | >120 | 2.4789 |
| hartmann3 | -3.7628 | bayes | 16 | -3.8258 |
| hartmann3 | -3.7628 | grid(13) | 89 | -3.7885 |
| hartmann3 | -3.7628 | grid-coarse(4) | >120 | -3.5631 |
| hartmann3 | -3.7628 | random | >120 | -3.6784 |
| hartmann3 | -3.7628 | aco | >120 | -3.7379 |
| noisyquadratic | 0.3 | bayes | 14 | 0.1162 |
| noisyquadratic | 0.3 | grid(13) | >120 | 38.6523 |
| noisyquadratic | 0.3 | grid-coarse(4) | >120 | 1.2180 |
| noisyquadratic | 0.3 | random | >120 | 1.7272 |
| noisyquadratic | 0.3 | aco | >120 | 0.5179 |

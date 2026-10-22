# tune lane validation

budget 120 trials per run, 20 seeds, median trials-to-target (121 means never reached), median best value at budget

| objective | target | method | median trials-to-target | median best f |
| --- | --- | --- | --- | --- |
| branin | 0.498 | bayes | 25 | 0.4316 |
| branin | 0.498 | grid | 0 | 2.1292 |
| branin | 0.498 | random | 0 | 0.6612 |
| branin | 0.498 | aco | 0 | 2.3231 |
| hartmann3 | -3.7628 | bayes | 16 | -3.8258 |
| hartmann3 | -3.7628 | grid | 102 | -3.7898 |
| hartmann3 | -3.7628 | random | 0 | -3.6784 |
| hartmann3 | -3.7628 | aco | 0 | -3.7379 |
| noisyquadratic | 0.3 | bayes | 14 | 0.1162 |
| noisyquadratic | 0.3 | grid | 0 | 44.8995 |
| noisyquadratic | 0.3 | random | 0 | 1.6575 |
| noisyquadratic | 0.3 | aco | 0 | 0.5192 |

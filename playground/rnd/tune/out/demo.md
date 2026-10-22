# tune lane clock dry-run

objective: worst drain-trajectory deviation in ms at tc 3 (10+5) over lengths [20 30 40 50 60], cost models exact, overshoot[1,1.3], slowstart 10x3

grid reference: 108 cells (the clocktune 6x6x3 grid), best {Kp 0.060, Ki 0.030, Kd 0.010} worst deviation 179797 ms

bayes stream: budget 40, best {Kp 0.0600, Ki 0.0300, Kd 0.010} worst deviation 179797 ms

trials to first value at or below the full-grid best: 22 of 40

the sweep objective is monotone in Kp at increment controls (the clocktune structural finding), so this dry-run measures sample efficiency on a real repo surface, not a new optimum

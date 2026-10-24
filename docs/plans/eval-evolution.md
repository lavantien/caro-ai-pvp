# eval weight evolution

Owner rulings 2026-10-24, fixed before any further machine time: self-play 50/50 split checks are cut from eval weight adoption entirely, the owner called them unnecessary and time-wasting for the goal, and the evolution side joins a feedback loop where candidate weight sets test fitness against each other in real games instead of by offline validation loss alone.

## what parked the protocol

The preregistered adoption chain ran four floor arms before the two deciding pairs. The fitted set covered 0.5 at both controls, 2+1 landed at decisive fraction 0.470 with CI 0.372 to 0.568 over 100 games. The scaffold set blocked at 2+1: 300 games across 15 dirs, decisive fraction 0.662, CI 0.605 to 0.719, never covering 0.5 at the extension cap, and at that decisive fraction the halfwidth needed to reach 0.5 exceeds what any volume can deliver, so more games move the interval away from 0.5 forever. The chain exited floor-failed and paused for re-registration with no deciding pair ever played, which is what makes the re-registration clean: nothing downstream of the removed rule has ever run.

## re-registered protocol

No floor, noise, or split precondition exists anywhere in weight adoption. The chain is the two deciding pairs alone:

```
evalpair-21  master vs master-w97.122.512.513.2077 at tc index 1 (2+1)
evalpair-32  master vs master-w97.122.512.513.2077 at tc index 2 (3+2)
```

20 games per pair split across both seats, both arms carry -evalweights 97,122,512,513,2077 -evalbase master so the derived seat exists in the registry, each pair appends to a resumable -pairOut log and relaunching skips completed arms. The series decides on the played games across both controls. A tie falls to validation loss, where the fitted set holds 0.65568 against the scaffold's 0.67078, the standing tie-break from the preregistration. Adoption lands as the PatternClassWeights swap at config.go:168-172 with the pair tables committed beside it as the arena evidence. The floor machinery in the arena binary stays: the hex noise gates still measure their bands through it, only weight adoption stops requiring any.

## evolution loop

The fitting program today is offline: gen.go grows a game corpus at a tier budget with geometry support, fitShade in evolve.go runs SHADE differential evolution over the 5 class weights plus the scale, and fitness is validation loss on held-out games. The ruling moves fitness into games, the loop becomes candidate populations tested against each other.

Each round: the fitter emits a candidate set from the top of the offline DE population, perturbations of the incumbent vector, and cross-round survivors. The arena plays a round-robin among the candidates plus the incumbent at a fast control with W+D scoring over both seats. Results select: winners seed the next fitting round, either as DE restart centers, restartAround already recenters the population on the best vector, or through their games joining the fit corpus rendered in the games-file format gen and fit already parse. A candidate adopts only when it beats the incumbent outside the recorded band at the gated controls 2+1 and 3+2, per the arena evidence law: offline loss and fast-control dominance are screening only, adoption is decided by the gated pairs. The incumbent updates after every adoption, so the loop always measures against the live shipped weights.

Machine law unchanged: every arena round is one engine-timing batch on the machine, serialized with the hex gates and the night batches, never concurrent. Hex rides the existing -geometry seam end to end, so the same loop produces hex candidates from a hex corpus without a second program.

## sequencing

The deciding pairs run first: they are fully specified above, cheap, and unblocked by the re-registration. The loop rounds follow behind them, one per night batch, until a candidate clears the gated pairs or the population stalls with the finding recorded.

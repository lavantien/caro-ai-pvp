# smoke21 rescale + ponder preregistration

Question: does the demand-spec tier re-scale (easy 32 MiB, medium 128 MiB, hard 1 GiB, master unchanged at 2 GiB) hold the easy < medium < hard ladder at 2+1, and what does a full-roster tournament record for the blocked master tier running at its spec shape minus the opening book (8 threads, 2 GiB, VCF+VCT, pondering)? Registered before any gate game runs, per the inversion research law.

## run

`make tourney-full ARGS="--db db/gates-v025.db --parallel 1"`, the default 8-seat roster (easy-1/2, medium-1/2, hard-1/2, master-1/2) at 2+1 bo3, every pairing meeting twice with seats swapped: 56 series, 112 to 168 games, parallel 1 forced by the ponder-aware core law (one room's worst case is master searching 8 while master ponders 8 = MachineCores). Detached as the machine's only engine-timing batch. Expected wall-clock: nights. Resume via `make tourney-resume` on any interruption shape.

## decision rules, fixed in advance

1. Ladder (gates the re-scale ship): zero cross-tier inversions among easy, medium, and hard. Every hard seat finishes above every medium seat and every medium above every easy in the final standings, and each tier's aggregate W-L-D against the tier directly below is a winning record. One inversion blocks the re-scale tag pending the 3+2 arm and investigation; the v0.19 reference at the old sizes holds the same shape exactly.
2. Master (descriptive, no ship decision, the tier is already blocked): record master's aggregate W-L-D and red/blue split against each tier and master-vs-master, plus the logstats per-tier telemetry. These are inputs to the v0.29 re-derivation, which derives its own bars; no new threshold is invented here.
3. Ponder health (gates the ponder ship): every M-line tagged [PONDER] must be a legal move, carry t under 1 second, carry alloc equal to that turn's budget draw, and derive nps from ponder nodes over ponder elapsed. Report the adoption rate per master seat. Any illegal move, engine panic, conductor reconciliation failure, or run failure blocks the ponder tag and routes to root-causing before any re-run.
4. Ledger: the rating space closes to an exact zero-sum and the logstats parse ledger accounts for every line; a violation fails the run as evidence.

## follow-up chain, queued behind this run

The 3+2 ladder arm (same shape, tc 3+2) completes the re-gate before the rescale tag decision. The master re-derivation arena pairs (master vs hard at both controls at the re-scaled sizes, master-novct perturbation, and master vs master-noponder for the ponder separation evidence) run after it, all serialized engine batches.

Addendum 2026-10-09, user instruction after the 2+1 arm: the evidence set extends with a third arm, smoke10 (1+0, the zero-increment end), queued behind the 3+2 arm. The three arms span the increment axis 0, 1s, 2s so the formal investigation sees the system under zero increment and under ample increment. The 1+0 arm is analysis-only: the v0.20 arena program established 1+0 as thread-decided, so it feeds the investigation and gates nothing. The registered decision rules above are untouched.

# nneval: nn eval and gpu feasibility

Question: can a neural-net evaluation function evaluated in batch (cuda when it pays, cpu matmul otherwise) coexist with the cpu alpha-beta search. Answer: no for this engine as shipped, both media. The gpu pays only under an architecture-class change, which is a new engine under the arena evidence law, not an offload.

## what the search needs

Fixed midgame position mirroring engine_test.go's fixture, fresh engine per repetition, fleet-loaded machine (out/search.txt):

- depth 8: 120834 nodes in 112.6 ms, 1.07 Mnps single-thread
- depth 9: 35.2 M nodes in 58.8 s, 0.60 Mnps
- standing unloaded reference: 1.4 Mnps per core, 5.6 Mnps at 4 workers

Leaf demand equals node rate to first order: one eval call per leaf (search.go:320). Today's eval is O(1) integer (eval.go:68) riding ~4 ns incremental make/unmake, so eval is ~1% of node cost and any replacement over ~100 ns/leaf becomes the dominant node cost.

## cpu pure-go matmul

Nets 256-64-32-1 (narrow) and 256-128-64-1 (wide), out/cpu.txt:

- narrow: 17.8-24 us/eval single-thread (48-56 k evals/s), best 6.5 us/eval at 8 threads batch 4096 (155 k/s)
- wide: 39.8-50 us single-thread, best 13.4 us/eval at 8 threads (75 k/s)

10-100x too slow against the 1.4 M leaf/s per core need. Batching barely helps under fleet load: only 2.5x from 8 threads.

## gpu batched eval

Weights resident on device, per iteration = synchronized input upload + fused mlp kernel + readback, gpu output cross-checked against the cpu forward pass at tol 1e-3 (out/gpu.txt, driver 617.42):

- launch floor 10.2 us sustained (7.1 us recorded 2026-10-07 in the prior gpuoffload analysis)
- h2d 23.4 GB/s at 8 MiB blocks but 4.6 GB/s at 256 KiB blocks
- sync round-trip floor ~60 us/iter (WDDM on GeForce, no TCC)
- narrow net: 0.97 M evals/s at batch 64, 3.5 M at 1024, 4.58 M at 16384
- wide net: 0.65 M / 2.1 M / 2.4 M

## verdict

Throughput exists only at batch >= 4096, which an alpha-beta dfs structurally cannot form. Each leaf's value is consumed immediately, a formable sibling batch is 10-40 leaves and pays a ~66 us sync round trip that exceeds the cpu cost of evaluating the whole batch, and queueing leaves across the 8 lazy-SMP workers drains 11 M leaves/s of production into the gpu's 4.6 M/s, collapsing nps below the shipped integer eval.

The prior gpuoffload revisit clause holds its exact shape: the gpu pays only under an architecture-class change (an mcts-style leaf queue or root-parallel batched probing, the lc0 class). That is a new engine gated by arena evidence, not an offload of the current one.

Top risks if attempted anyway: the WDDM ~60 us sync floor (a TCC or pro card changes it), 8-worker queue contention at 11 M leaves/s, thin training data (a 1500-game corpus) for a 41 kMAC net.

## queued post-fleet reruns

Unloaded search probe (expect ~1.4 Mnps), the 8-thread cpu cells, gpu floor checks.

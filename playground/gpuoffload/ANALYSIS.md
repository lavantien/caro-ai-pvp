# gpu offload assessment

Question: can the RTX 3080 (10 GB VRAM, CUDA 13.4) speed up or absorb engine computation beneficially? Asked 2026-10-07, answered by measurement below.

## environment

- CUDA toolkit 13.4 (nvcc V13.4.92). The machine PATH still resolves 12.8 ahead of 13.4 (registry re-checked 2026-10-07 after the user's edit: `CUDA\v12.8\bin` and `CUDA\v12.8\libnvvp` still precede both v13.4 entries), so the documented invocation keeps prefixing 13.4 explicitly.
- Host compilers: VS 2022 Community MSVC 14.44 linked the recorded dll run. The VS 2026 C++ workload was added 2026-10-07 (its `VC\Tools\MSVC` tree now exists), so either MSVC can host future links.
- Driver 617.14, GeForce RTX 3080, 10240 MiB.
- Integration shape: `nvcc -shared` builds a dll behind a plain C ABI, Go loads it at runtime through the Windows loader (`syscall.NewLazyDLL`), no link-time CUDA dependency. `make gpu-spike` after importing vcvars64 is the whole pipeline.

## measured floors

From `make gpu-spike`, 2026-10-07:

| cost | measured |
| --- | --- |
| kernel launch, sustained over 200000 launches | 7.069 µs |
| PCIe round trip, pinned 256 MiB blocks, 20 iterations | 23.4 GB/s combined |

## engine cost model

From the committed benches and the standing measurements in the README:

- 5.6 Mnps at 4 workers, so one searched node costs about 0.7 µs of one core.
- Make and Unmake 3.7 ns, win detection 13.4 ns, pattern lookup 0.26 ns, zero allocations in the hot path.
- Every hot-path operation is integer bitboard logic or a table lookup in local RAM. There is no floating point and no dense vector math anywhere in the search.
- TT probes hit local RAM (~100 ns class) against a lockless direct-mapped table.

## candidate mappings

| candidate | blocker | verdict |
| --- | --- | --- |
| per-node search offload | one kernel launch costs 7.1 µs, 10x the 0.7 µs an entire node costs, and alpha-beta is sequential per worker so nodes cannot batch | no |
| TT in VRAM | every probe would pay a PCIe round trip (10 µs latency class at 23.4 GB/s) against ~100 ns local, a 100x regression | no |
| pattern table precompute | 52 ms once at process start | nothing to win |
| VCF/VCT solver offload | depth-first threat search, the same branchy shape as the main search, launch floor dominates any batch it could form | no |
| rules or win detection kernels | 13.4 ns local | no |
| v0.24 bookgen corpus | the games are the same branchy search. A GPU wins on self-play only under batched or neural evaluation, an architecture change rather than an offload. CPU nights already schedule the corpus | revisit only on an architecture change |
| batched VCF/VCT proof precompute (the v0.24 solver overlay) | the first throughput-shaped candidate: thousands of independent root positions per batch amortize the 7.1 µs launch floor entirely. The blockers move to warp divergence (each thread walks its own irregular DFS with early cutoffs, so a warp serializes on divergent branches), VRAM-latency TT probes inside that DFS, and the port cost itself, a device-code rewrite of the solver for a job that runs once per book release on otherwise idle nights | no for this ladder, the first candidate to measure if book regrows become recurring or the corpus scales up |

## precedents

Stockfish dev-20260930 (checked 2026-10-07) carries no GPU anywhere: the precompiled universal binaries pick the best CPU instruction set at runtime, the notes mention GCC with balanced LTO and the SFNNv17 CPU net, and nothing in the toolchain mentions CUDA or nvcc. The strongest alpha-beta engine keeps evaluation on CPU SIMD for the same reason this assessment records, eval is called a few leaves at a time inside a latency-bound search and a PCIe round trip costs more than the whole evaluation.

lc0 v0.33.0-rc0 is the other architecture class: MCTS batches leaf evaluations into minibatches and pushes them through dense GEMM backends (cuda with CUTLASS fused attention, onnx-trt through TensorRT engines, onnx-coreml, onnx-migraphx on ROCm), and its release notes center on batch handling and network-evaluations-per-second throughput. The GPU pays there because the workload is uniform dense linear algebra in large batches.

This engine is Stockfish-shaped (alpha-beta family, per-node integer eval, latency-bound), so the assessment follows that precedent. The lc0-shaped class is exactly the architecture change the revisit trigger names.

## conclusion

Not beneficial for this engine. The measured host-side floors (7.1 µs launch, 23.4 GB/s PCIe) sit one and two orders of magnitude above the engine's per-node and per-probe costs, and the hot path is integer bitboard logic with nothing SIMT-shaped to map. The GPU becomes interesting only under an architecture-class change (a neural or batched-evaluation engine), which would be a new project gated by the arena evidence law, not a wave in this ladder.

Revisit trigger: a proposal that makes evaluation uniform and throughput-shaped (batched offline evaluation, a neural eval), never the per-move search.

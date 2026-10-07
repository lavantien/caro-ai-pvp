# gpu offload assessment

Question: can the RTX 3080 (10 GB VRAM, CUDA 13.4) speed up or absorb engine computation beneficially? Asked 2026-10-07, answered by measurement below.

## environment

- CUDA toolkit 13.4 (nvcc V13.4.92). The machine PATH still holds 12.8 ahead of 13.4 (registry edit needs elevation), so the documented invocation prefixes 13.4 explicitly.
- Host compilers: VS 2022 Community MSVC 14.44 links the dll. The VS 2026 install carries no C++ workload (no VC directory), so it hosts nothing until that workload is added.
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

## conclusion

Not beneficial for this engine. The measured host-side floors (7.1 µs launch, 23.4 GB/s PCIe) sit one and two orders of magnitude above the engine's per-node and per-probe costs, and the hot path is integer bitboard logic with nothing SIMT-shaped to map. The GPU becomes interesting only under an architecture-class change (a neural or batched-evaluation engine), which would be a new project gated by the arena evidence law, not a wave in this ladder.

Revisit trigger: a proposal that makes evaluation uniform and throughput-shaped (batched offline evaluation, a neural eval), never the per-move search.

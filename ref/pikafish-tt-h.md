# pikafish tt.h

source: https://github.com/official-pikafish/Pikafish/blob/master/src/tt.h
fetched: 2026-10-18 via zread read_file (full file, verbatim excerpts)
license: GPL-3.0 (Stockfish/Pikafish), quoted for study

## facts

- One global hash table for the engine and all threads. Racy updates between threads are deliberate: synchronizing would cost thinking time and thus Elo. Collisions can cause blunders or faulty mate reports; fixing them also loses Elo; risk shrinks with larger TT.
- TTData: move, value, eval, depth, bound, is_pv (local copy, possibly inconsistent from races).
- TTWriter: racy non-atomic writes; `void penalize(int penalty);` decrements the stored depth by a penalty (used to demote entries that misbehaved).
- TranspositionTable: resize(mbSize), multithreaded clear(), `new_search()` at each root search start (bumps entry age), `u8 generation()` for writes, hashfull(maxAge) approximating permille written during this root search, probe(key) returning (hit, TTData, TTWriter), first_entry(key) for prefetch.
- generation8 is a u8 aging counter incremented per root search.

## extraction notes

- Design stance: correctness-vs-speed trade explicitly resolved toward speed; the racy TT is an accepted engine-grade hazard, mitigated by table size, not locks.
- penalize() is a feedback channel into replacement: the search demotes TT entries that produced bad cutoffs rather than deleting them.
- new_search()/generation8 gives per-search aging so old-search entries (including ponder-era writes) decay by generation, matching our per-search TT generation bump design.

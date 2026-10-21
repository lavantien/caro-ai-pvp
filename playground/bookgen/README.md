# bookgen

Opening book corpus generator. A nested Go module that plays tier-vs-tier games, records sub-BookMaxPly positions into append-only corpus shards, solves floor-passing positions with VCF/VCT, and emits the CAROBOOK1 artifact the engine loads.

## phases

- `-phase corpus -games N -corpus PATH`: append games to one shard until it holds N games.
- `-phase overlay -corpus PATH -overlay PATH`: solve floor-passing corpus positions into the overlay sidecar.
- `-phase emit -corpus PATH -overlay PATH -out PATH -stats PATH`: aggregate the corpus, apply the overlay, publish the artifact and stats.
- `-phase merge -games N -out PATH SHARD...`: validate shard fingerprints against each other and the `-games` target, concatenate game blocks in shard order into one corpus, and refuse a kept-game total below N. Shards go last on the command line because the Go flag parser stops at the first positional argument.
- `-phase stats [-floor F] PATH...`: read-only saturation monitor over one or more corpora, printing total games, unique position keys, distinct trunks, and per-ply unique and floor-passing counts for ply 0 through 15.

## merge rules

Every shard must match the `-tier` and `-tc` flags exactly and carry a `games` target at or below the merge `-games` target, so an accumulation member at the full target and its smaller re-armed segments unify under one requested target; a tier or tc mismatch refuses with the file and field named, and a shard whose games target exceeds the merge target refuses as a wrong-generation shard. The merged header carries the `-games` target. A shard still held by a running corpus member, or ending torn, refuses. Header-only shards count as zero games and merge cleanly. Duplicate games are detected by a SHA-256 signature over the game block body, skipped in shard order, and counted in the log; the `-games` floor applies to the kept total.

Merging is idempotent with no delta: the merged file is itself a plain corpus with the target fingerprint and games renumbered from zero, so re-merging a merged file alone reproduces it byte for byte. The fingerprint's `games=` field stays the identity target for every contributor, while the true game count is the block count the file carries and the merge log reports.

## examples

```
go -C playground/bookgen build -o bin/bookgen.exe .
bin/bookgen.exe -phase merge -games 1500 -out playground/bookgen/out/book-corpus-all.txt playground/bookgen/out/book-corpus.txt playground/bookgen/out/inst2/book-corpus-b.txt playground/bookgen/out/inst3/book-corpus-c.txt
bin/bookgen.exe -phase stats -floor 20 playground/bookgen/out/book-corpus-all.txt
```

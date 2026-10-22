# solver mirror parity snapshot 2026-10-22

Fresh mirror-parity evidence at the live corpus snapshot, closing the gap where only the 11298/11298 state was committed. Node counts are deterministic and load-immune. The committed exp1 through exp4 artifacts remain the earlier corpus state that REPORT.md cites.

Corpus files at run start (fingerprints from each file's F line):

- out/book-corpus.txt, 102 games, F corpus v2 tier=master tc=3 games=1500
- inst2/book-corpus-d.txt, 46 games, F corpus v2 tier=master tc=3 games=55
- inst3/book-corpus-c.txt, 54 games, F corpus v2 tier=master tc=3 games=55

Run: `go run . -corpus <absolute comma-joined paths>` from playground/rnd/search, 202 games parsed, 30 positions, 32 tactical.

- exp1 ref total nodes 12812, 32/32 positions have four sources
- exp1 mirror total nodes 12812, verdict flips 0

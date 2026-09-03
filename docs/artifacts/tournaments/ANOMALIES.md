# Tournament artifact anomaly analysis

Analysis of the round-robin artifacts committed 2026-09-03 (v9.4.2 `initial-5`, v9.4.3 `l1v5-100`, plus both smokes). Sources: `matches.db` (via `node:sqlite`), `run.log` statlines, `summary.json`, and the backend source at commit d2db55c. Findings are ranked by impact; each names the evidence and the regression test that now guards it.

## 1. VCF solver claims forced wins that do not convert

Six games contain `[VCF]` moves carrying `search_score=30000` played by the side that then lost or drew:

| run | game | seed | loser claims | outcome |
|---|---|---|---|---|
| l1v5-100 | 9 | 20260830 | mv 33 (chain 10), mv 35 (chain 8) | L1 won at mv 65 |
| l1v5-100 | 22 | 20260843 | mv 22-28 (chain 6 to 3) | L1 won at mv 30 |
| l1v5-100 | 23 | 20260844 | mv 21-27 (chain 11 to 8) | L1 won at mv 65 |
| l1v5-100 | 67 | 20260888 | mv 35-43 (chain 12, five consecutive re-claims) | L1 won at mv 75 |
| l1v5-100 | 70 | 20260891 | mv 22 (chain 4) | L1 won at mv 30 |
| initial-5 | 28 | 20260849 | mv 31 (chain 6), mv 95-105 (chain 12 to 7) | L2 won at mv 109 |

A score of exactly 30000 is `Constants.Score.WinScore`, set only on the VCF solver's win path, so each of these moves opened a chain the solver proved as forced. In every case the chain collapsed: the next solver invocation found no win, normal search took over, and the opponent won. Game 67 is the cleanest signal: the same depth-12 claim (the `DifficultyProfile.VCFDepth` cap for L5) re-appeared after each opponent reply for five consecutive turns and never converted, and the loser's final board contains a both-ends-blocked exact five.

`MoveOrdering.WouldWin` (MovePicker.cs) and the domain `WinDetector` agree on the rules (exactly five, no overline, not both ends blocked), and `PatternWindow.FiveCompletionsInDir` also implements them correctly, so the unsoundness is in the chain logic of `Caro.Engine/Vcf.cs`. Regression fixtures: `Caro.Engine.Tests` replays the recorded positions before l1v5-100 game 67 mv 35 and initial-5 game 28 mv 95 and asserts `VCFResult.NoWin`; l1v5-100 game 45 mv 23 (seed 20260866, chain converted) is the positive control that must keep resolving.

L5 is the heaviest VCF user (445 of 2177 moves in l1v5-100, about 20 percent) while L2 uses no VCF at all. That distribution matches the ladder inversion: L2 beat L3 4-1 in initial-5, and every adjacent step involving a VCF level underperformed.

## 2. Search scores above the Infinity bound in the draw games

All three draws (l1v5-100 games 7, 17, 63) contain `exact` moves with scores 100000, 100002, ... 100008, stepping up by 2, at depth 2:

```text
M247 blue i1  d=2  n=340   nps=7K    tt=100% s=+100002 thr=8 t=0.1s alloc=6.0s
```

`Constants.Score.Infinity` is 100000, the search window bound; `MaxEval` is 25000 and the largest legitimate score is `WinScore=30000`. These values are bound leakage: `MateScore.AdjustForStore`/`AdjustForRetrieve` (Quiescence.cs) ply-adjust any score beyond `WinScore - AbsoluteMaxDepth`, and the +-Infinity window bounds satisfy that test, so stored bound values get shifted by ply on every store/retrieve round trip and drift upward across moves. The corrupt score satisfies `IsForcedWinScore`, which trips the early deepening break in ParallelSearch.cs, so L5 stopped at depth 2, spent 0.1s of a 6s allocation while believing it was winning, and drifted into a board-full draw. Regression: `Caro.Engine.Tests` asserts the mate-score adjustment never moves a value at or beyond +-WinScore, plus a recorded-game replay of the game 7 endgame asserting every returned score stays inside `[-WinScore, WinScore]`.

## 3. Draw boards saturated with illegal fives

Replaying the recorded positions of the three draws, the final boards hold 6 to 8 exact-five runs each, every one both-ends-blocked, split across both players. Both engines repeatedly completed fives the rules reject. This is the same divergence cluster as findings 1 and 2: scoring paths treat five-completion as a win while the rules require an open end.

## 4. All timeout-fallback moves are L1's

The 15 fallback moves in l1v5-100 and both in initial-5 all belong to L1. Each spent 640-850ms on 3.4k-13k nodes without completing depth 1, running 2-5x below L1's normal nps, all at move 44 or later, clustered (3 in game 49, 3 in game 95, 2 in the game 17 draw). A depth-capped level failing to finish ply 1 in ~660ms points at a quiescence or candidate-generation blowup in specific late-game shapes. Not fixed in this pass; the runner report now attributes fallback counts per level so the pattern stays visible.

## 5. The L5 leak decomposed

Of L1's 17 wins in l1v5-100: 5 followed refuted VCF claims (finding 1), the other 12 are positions where L5's own final scores show -29997/-29999, meaning its search saw the mate two moves too late against a depth-2 opponent. The 3 draws are findings 2 and 3. Time management is exonerated: L5 think/alloc p90 was 0.94, nobody's clock went below 60s, and no game ended by timeout. The prior working theory that blamed L5 time allocation was wrong.

## 6. matches.db attributes blue's winning move to red

In all 46 blue-winner games of l1v5-100, the final move row carries `player='red'` while its `difficulty` is correctly the winner's. `MovePersistence.LogAIMove` re-derives the mover as the opponent of `resp.CurrentPlayer`, which inverts when the game ends on blue's move. The API response's `lastMove.player` is built from the true mover, so `summary.json`, `report.md`, and the runner aggregates are unaffected; only per-player queries over `matches.db` are corrupted, by one row per such game. Regression: `Caro.Api.Tests` asserts a blue-winner game's final persisted row has `player='blue'`.

## 7. Draws persist as winner='abandoned'

`GameHandlers` coerces any empty/none winner on the delete path to `EndReasons.Abandoned`, so the three board-full draws carry `winner='abandoned'` in `matches.db`, indistinguishable from actually-abandoned games. Fixed to coerce only when the game is not over; a finished draw persists `'none'`. Regression: `Caro.Api.Tests` covers both the finished-draw and live-game-delete paths.

## 8. Bookkeeping notes (documented, no fix)

The seeded opening places two stones under move number 1 which are not persisted, so `games.move_count` exceeds the moves row count by exactly 2 in every game and the first persisted row is always move 2. In each draw game the final board-filling move is persisted with default stats (`d=0 n=0 t=0 alloc=0 score=0`, type `exact`). Report/summary arithmetic is unaffected because the runner counts only statline-bearing moves.

## Reproducing the queries

```sql
-- phantom VCF claims by the eventual loser
SELECT g.rowid, m.move_number, m.vcf_depth, g.winner
FROM moves m JOIN games g ON g.id = m.game_id
WHERE m.move_type = 'vcf' AND m.search_score >= 29990;

-- scores beyond the WinScore ceiling
SELECT g.rowid, m.move_number, m.search_score, m.search_depth
FROM moves m JOIN games g ON g.id = m.game_id
WHERE m.search_score > 30000;

-- fallback moves by level
SELECT difficulty, COUNT(*) FROM moves
WHERE move_type = 'timeout-fallback' GROUP BY difficulty;
```

The fixture move lists embedded in the regression tests were extracted from the `matches.db` archives with `node:sqlite`; the openings are regenerated in-process from the game seeds (20260828, 20260849, 20260866, 20260888) via the shared `Caro.Domain` opening placement.

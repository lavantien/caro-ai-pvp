# Reference engine difficulty gradients

How reference implementations weaken a bot while keeping tactical soundness, surveyed 2026-10-28 against current sources.

## Stockfish skill levels

The mechanism is move-choice degradation over a full-strength search. The Skill struct converts UCI_LimitStrength plus UCI_Elo into an internal level 0 to 19 through a cubic fit anchored to the Stash engine, covering Elo 1320 to 3190. Skill Level 20 means disabled. Source: https://github.com/official-stockfish/Stockfish/blob/master/src/search.h

When any handicap is on, Stockfish raises MultiPV to at least 4 behind the scenes, so the root always evaluates at least 4 legal moves at full strength. Pick happens per completed iteration at depth 1 plus level, so a weaker level fixes its choice from a shallower completed iteration while the search itself still runs to full depth. Source: https://github.com/official-stockfish/Stockfish/blob/master/src/search.cpp

The pick itself, Skill::pick_best, sorts root moves by score descending, computes weakness as 120 minus 2 times level, and adds a deterministic plus a random push toward worse moves, with the score gap of the candidate set capped at one pawn. The chosen move always comes from the top of a full-vision search, never from outside it. Idea credited to Heinz van Saanen. Source: https://github.com/official-stockfish/Stockfish/blob/master/src/search.cpp

No search component is ever disabled at low skill: the same TT, tablebases, and full depth run at level 0 as at level 20. Known complaints about the design are calibration and range, not blindness: levels 1 to 19 can feel similar and level 0 stays too strong for beginners. Sources: https://github.com/official-stockfish/Stockfish/issues/4431, https://github.com/official-stockfish/Stockfish/issues/3635, https://talkchess.com/viewtopic.php?t=49736

## Gomoku and renju engines

Rapfi, one of the strongest Gomoku and renju engines, publishes no strength-level or difficulty mechanism. Tournament engines of this family run at full strength, configured by time and threads, and weaken only by receiving less time. Source: https://github.com/dhbloo/rapfi

The technique space is shared across the family: depth-first proof-number search answers whether a forced win exists, alpha-beta over minimax picks moves. The forced-win prover and the move picker are separate instruments, and published play strength talk centers on time and depth, not on tiers that remove the prover. Source: https://stackoverflow.com/questions/21548865/gomoku-state-of-the-art-tech

## KataGo

Weakening is a compute budget or a different model, never a reduced reading of tactics. The GTP config caps the search with maxVisits, maxPlayouts, and maxTime, example values 500, 300, and 10.0. Source: https://github.com/lightvector/KataGo/blob/master/cpp/configs/gtp_example.cfg

Human-like weaker play ships as a dedicated model: gtp_human5k_example.cfg imitates a 5 kyu human with the human SL net, maxVisits 40, one thread, lagBuffer 1.0. Handicap support is a first-class mode with placement and detection code in the engine. Sources: https://github.com/lightvector/KataGo/blob/master/cpp/configs/gtp_human5k_example.cfg, https://github.com/lightvector/KataGo/blob/master/cpp/README.md

Strength studies treat visit count as the strength dial, for example the FAR AI evaluation running KataGo at various visit settings up to 100K. Source: https://goattack.far.ai/human-evaluation

## Lichess

Server bots run Stockfish with per-level time and depth caps, and the move selection layer interpolates between real game frequency at level 1 and expected value at level 8, producing human-like choices. The engine underneath keeps full vision. Sources: https://zread.ai/lichess-org/lila/17-fishnet-distributed-analysis, https://lichess.org/forum

## Synthesis

Reference consensus weakens bots by move choice or by compute budget at fixed vision, never by removing tactical components per level. Stockfish at any skill runs the same full search and picks suboptimally from its top candidates, KataGo lowers visits or swaps the model, Lichess caps time and randomizes choice. A tier ladder that strips the solver and the table from lower tiers inverts that principle, and the references would predict our failure mode: a tier-flat forced-line prover on the weak side against a deep-search defender whose blindness grows with TT scale and warmth. Our measured inversions, medium at 0.350 beating master harder than hard does, are the signature of capability removal, not of calibrated weakness.

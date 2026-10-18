# fairy-stockfish wiki: faq

source: https://github.com/fairy-stockfish/Fairy-Stockfish/wiki/FAQ
fetched: 2026-10-18 via WebFetch

## content as fetched

No information about opening books, pondering, or UCI_LimitStrength on this page; the only strength topic is Skill Level.

Executable choice: largeboard x86-64.exe for compatibility, largeboard bmi2 for speed, non-largeboard for boards up to 8x8 (faster).

Best performance settings: Threads and Hash are the main strength settings; analysis at maximum hardware, gameplay hash by time control with the rule of thumb 1 MB per thread per second of thinking time.

Skill Level 20 plays to full ability; levels below 20 (range -20 to +19 in UCI) are artificially weakened.

Variant support criteria: two-player, alternating turns, perfect information, complete information (no randomness), rectangular board, board size at most 12x10. Excludes four-player chess, avalanche, fog of war, dice chess, hex, go. Eligible games can be requested via issue.

Xiangqi: UCCI and UCI-cyclone supported; AXF chasing rules from 14.0.2. Janggi: four rule sets (janggi, janggitraditional, janggimodern, janggicasual) toggling bikjang and material counting.

## notes

- Confirmed negative: no engine-side opening book facility anywhere in the FAQ, README, or options surfaced so far. Books in this family live in the GUI (cutechess et al); the engine contract is position-in, bestmove-out plus optional ponder move.
- Support criteria (two-player, alternating, perfect information, rectangular, bounded size) is a clean checklist our game passes; their 12x10 bound versus our 16x16 means we could never be a variants.ini entry even in principle, a second reason our own engine exists.
- Skill Level exists as the artificial weakening lever; our no-handicap law deliberately rejects this class of knob.

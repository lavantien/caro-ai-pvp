# pikafish wiki home

source: https://www.pikafish.com/wiki/en/
fetched: 2026-10-18 via webReader (with_links_summary)

## content as fetched

About Pikafish: free, open-source Xiangqi (Chinese chess) AI engine, currently the strongest Xiangqi AI engine. Derived from the chess engine Stockfish, inherits GPL-3.0. Basic search framework is Alpha-Beta pruning, search algorithm is PVS (Principal Variation Search). Evaluation uses a neural network with the NNUE architecture. Computes on CPU, nothing to do with the graphics card.

Pikafish has no GUI; a separate GUI or an all-in-one package is needed. Official releases may not support Windows 7 or 32-bit systems.

Wiki nav: "Start here" section lists Getting Started with Xiangqi AI and Game Analysis, UCI Options and common questions (settings/troubleshooting: threads, Hash, rules), Source Code Guide then explore search, NNUE, position state and rules, engine runtime and communication.

NNUE license for pikafish.nnue weights: legal use only, no commercial use without permission, any usage of the weights constitutes agreement.

Language variants of the wiki: en, zh-cn, zh-hant, vi.

## notes

- Wiki is VitePress; subpage URL pattern is /wiki/en/<section>/<page>.html, sections seen: guide (ucci-gui), faq (test-data, vs-commercial-engines, ptnml, gui). Direct /wiki/en/code/*.html guesses 404; locate source-guide pages via the sidebar of a known page before citing.
- ptnml FAQ page: paired-game outcome merging for unbalanced-opening tests (SPRT tooling context).

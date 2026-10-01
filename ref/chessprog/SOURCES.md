# chessprog sources

Fetched 2026-10-01 from chessprogramming.org, which is serving a temporary read-only recovery of the wiki. Method for every page: browser (chrome-devtools MCP) with same-origin fetch plus DOM-to-markdown extraction, tables preserved as markdown tables.

| slug | url | method | bytes |
|---|---|---|---|
| Alpha-Beta.md | https://chessprogramming.org/Alpha-Beta | browser | 36892 |
| Bitboards.md | https://chessprogramming.org/Bitboards | browser | 23229 |
| Evaluation.md | https://chessprogramming.org/Evaluation | browser | 32503 |
| Extensions.md | https://chessprogramming.org/Extensions | browser | 12806 |
| History-Heuristic.md | https://chessprogramming.org/History_Heuristic | browser | 12954 |
| Iterative-Deepening.md | https://chessprogramming.org/Iterative_Deepening | browser | 12064 |
| Killer-Move.md | https://chessprogramming.org/Killer_Move | browser | 1926 |
| Move-Ordering.md | https://chessprogramming.org/Move_Ordering | browser | 27224 |
| Pattern-Recognition.md | https://chessprogramming.org/Pattern_Recognition | browser | 20400 |
| Principal-Variation-Search.md | https://chessprogramming.org/Principal_Variation_Search | browser | 25540 |
| Proof-Number-Search.md | https://chessprogramming.org/Proof-Number_Search | browser | 28826 |
| Quiescence-Search.md | https://chessprogramming.org/Quiescence_Search | browser | 23039 |
| Search.md | https://chessprogramming.org/Search | browser | 30509 |
| Selective-Search.md | https://chessprogramming.org/Selectivity | browser | 8142 |
| Shared-Hash-Tables.md | https://chessprogramming.org/Shared_Hash_Table | browser | 35336 |
| SIMD-and-SWAR-Techniques.md | https://chessprogramming.org/SIMD_and_SWAR_Techniques | browser | 12208 |
| Stockfish.md | https://chessprogramming.org/Stockfish | browser | 56246 |
| Threats.md | https://chessprogramming.org/Threat_Move | browser | 1388 |
| Time-Management.md | https://chessprogramming.org/Time_Management | browser | 18570 |
| Transposition-Table.md | https://chessprogramming.org/Transposition_Table | browser | 53751 |
| Zobrist-Hashing.md | https://chessprogramming.org/Zobrist_Hashing | browser | 22110 |

## Title resolutions and misses

- Selective-Search.md: /Selective_Search exists but is a periodical (computer chess magazine). The search-technique equivalent title is /Selectivity, used instead.
- Shared-Hash-Tables.md: real title is singular /Shared_Hash_Table.
- Threats.md: no /Threats page exists in the mirror. Closest real title /Threat_Move used. /Mate_Threat_Extensions also exists and is the null-move mate-threat extension variant.
- Replacement-Schemes: FAILED as a standalone page, HTTP 404, and no replacement/scheme page exists anywhere in the mirror's Special:AllPages index (4301 pages checked). Its subject is covered by the Replacement Strategies section inside Transposition-Table.md.
- Lockless-Hash-Tables: FAILED as a standalone page, HTTP 404, absent from the index. Closest real coverage is Shared-Hash-Tables.md (same URL row above), which covers the lockless XOR-tagged-entry approach for shared SMP hash tables.
- Killer-Move.md: /Killer_Move is a real but short definitional page. The deeper companion page /Killer_Heuristic exists in the mirror and was not part of the mandated page set.

## Stockfish NNUE exclusion

Stockfish.md drops every heading section and list item (selected features, see-also, publications, forum/blog links) whose text is NNUE-specific. Three factual history sentences and two link titles still mention NNUE by name; no NNUE technical content is included.

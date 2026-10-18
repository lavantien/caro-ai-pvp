# pikafish wiki: ptnml

source: https://www.pikafish.com/wiki/en/faq/ptnml.html
fetched: 2026-10-18 via WebFetch

## content as fetched

Ptnml is short for pentanomial. "Ptnml merges the two paired games of one opening into five outcomes, and is very useful for unbalanced-opening tests."

Five outcomes from two paired games (both engines play both colors of each opening):
1. two wins
2. one win and one draw
3. one win and one loss, or two draws (the tie)
4. one loss and one draw
5. two losses

A pair win means winning both games, once per color. A tie means the sides split one apiece or both games drawn. The page calls ptnml "very valuable data in paired-game testing with unbalanced openings."

SPRT specifics live on a separate FAQ page ("How do you test engines scientifically?": same conditions, varied openings in paired games, enough games with statistical tools, unbalanced openings when strengths are close).

## notes

- Our arena already runs paired red/blue games per opening; our verdicts currently tally game outcomes independently. Pentanomial merging per opening pair is the statistically correct tightening for unbalanced openings (our game has no draws except board fill, so category 3 collapses to 1-1 splits), it reduces variance exactly where our noise-band decisions are tightest (tier edges at 20 games per pair).

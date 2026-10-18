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

- Our arena runs paired red/blue games per seat duel, but the 2 games of a pair do not share an opening: each starts from an empty board, so the pairing merges color variance only, and the opening-variance cancellation of the strict ptnml construct is absent. Amendment 2026-10-18 after adversarial review of the landed pairing: original note claimed shared openings, that was wrong. Color-pair merging is still the right tightening here (first-move advantage is the dominant systematic per the red-first evidence), and full shared-opening ptnml becomes possible once the book can inject openings. Our game has no draws except board fill, so category 3 collapses to 1-1 splits.

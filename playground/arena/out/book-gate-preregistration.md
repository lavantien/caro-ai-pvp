# v0.24 book gate pre-registration

Book artifact: db/book.carobook, 22 majority entries grown from 308 master-tier self-play games at tc 3, solver overlay found 0 proofs. Seats: grandmaster (BookMaxPly 16) and master (BookMaxPly 8) against their nobook twins from playground/arena seat labels.

Arms, 20 games per pair, serialized one arena process at a time:

1. grandmaster vs grandmaster-nobook at tc 1 (2+1)
2. grandmaster vs grandmaster-nobook at tc 2 (3+2)
3. master vs master-nobook at tc 1
4. master vs master-nobook at tc 2

Then the evalfit side-gate per regate-2plus1-gate.md and regate-3plus2-gate.md: grandmaster vs the derived weights seat 137,162,483,484,1561 at tc 1 and tc 2, 20 games each.

Decision rule, fixed before any result: score each pair by the ptnml rows. A book seat below its same-tier floor noise band at that control (the committed floor-gm21, floor-gm32, and fivetier floor verdict tables) fails the gate: the builder is at fault, fix and regrow before v0.24 tags. Inside the band ships the book with the caveat recorded in the verdict table. Outside-band wins ship clean. No arm may reinterpret its band after seeing results.

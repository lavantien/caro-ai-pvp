# Design system

The v0.29 light contract. Light is the base presentation on every surface. The page is light pink with a pastel palette and dark plum ink, and no dark variant exists. The palette hub stays `internal/server/web/shell.css` `:root`. `go run ./cmd/run contrast` (playground/contrast, wired into `ci`) checks it.

## Identity

Caro is 5-in-a-row on a grid. The board lattice, the red and blue stone discs, and the amber last-move ring are the product's identity. The UI is furniture around them. Flat borders carry structure. Zero box shadows except the amber ring. No gradients, no all-caps eyebrow labels, no numbered markers, no middle-dot meta strings, no monospace for human prose.

The header brand, the word caro plus its 2 variant glyphs, is 1 unit. It never shrinks and never wraps to separate rows at any width down to 320px. It holds `nowrap` and `flex: none`, and the header wraps the nav below it instead.

The mover is never announced by a text line: no "red to move", "your move", or "blue to move" copy anywhere. The lit seat card carries whose turn it is. A surface that needs to name the mover in prose (the tournament live cards) prints the player's name, never a bare color word.

## Color tokens

The 12 shipped anchors never change value without a contrast gate rerun.

| token | value | role |
| --- | --- | --- |
| `--background` | `#f9e8f1` | page, glyph ink, button label on accent |
| `--surface` | `#fff7fb` | cards, fieldsets, board fill |
| `--border` | `#c48ba1` | every hairline |
| `--text` | `#2a2430` | copy |
| `--text-muted` | `#6f5f75` | status and meta |
| `--accent` | `#7050b8` | links, primary button fill |
| `--accent-strong` | `#5d3f9e` | primary button hover and active fill, focus ring |
| `--accent-soft` | `#ead9f7` | accent tint fill (quiet hover, live chip) |
| `--danger` | `#b02a44` | errors, armed forfeit |
| `--red` | `#c2185b` | red stones, red clock |
| `--blue` | `#1565c0` | blue stones, blue clock |
| `--mark` | `#b26a00` | amber, see the amber law |

A second surface depth step (`--surface-raised`) is absent by design: depth is 1 fill plus borders. Add it only when a surface needs to float.

### The amber law

`--mark` is the data-attention color. It marks the latest stone on the room and playback boards, the latest mini stone on the tournament live cards, the live-tournament line on the rooms grid, and the head-to-head score figures wherever they sit beside versus text. The rooms-grid line includes its link text, rendered at `--text-xl` bold so it qualifies as large text at the 3 floor. The score figures (`.score`, the series tallies, room-card series scores, run leaders) render bold at the 3 floor on both page and card surfaces. Amber carries no interaction-signal duty. The focus ring rides `--accent-strong`. Amber never decorates anything else. The old dark-theme amber `#ffb454` measured about 1.9 on the pink page and is retired. The `#b26a00` family carries the law on light.

## Type

2 voices over 2 tokens:

| token | value | voice |
| --- | --- | --- |
| `--font-human` | `system-ui, sans-serif` | everything a person reads: prose, names, headings |
| `--font-mono` | `ui-monospace, monospace` | machine data: room ids, run ids, bot names on bot-only surfaces, clocks, M-lines, move lists, playback counters |

Mixed surfaces (room page seat labels, history rows, score lines) can carry a human or a bot name, so they stay in the human voice. Only surfaces that always render bot data take the machine voice.

2 typographic laws on versus text. A variant chip renders the glyph only. The words square and hex never appear as chip text. The `title` attribute carries the name for assistive tech. On a versus line only the names carry weight: `.vname` is the bold part, the `vs` between them stays plain. A head-to-head score beside versus text is data, so it rides `.score`: bold and amber per the amber law, never body ink.

Scale, 1 step up from the v0.27 set so the board-first mobile layout reads larger:

| token | value |
| --- | --- |
| `--text-xs` | `.85rem` |
| `--text-s` | `.9rem` |
| `--text-m` | `.95rem` |
| `--text-base` | `1.05rem` |
| `--text-l` | `1.15rem` |
| `--text-xl` | `1.3rem` |
| `--text-2xl` | `1.7rem` |

The room clock keeps its `2.25rem` component metric (single-source, the room's hero data) and the square stone glyph rides `--text-l`. Both are component metrics, not scale steps.

## Spacing and radius

| token | value |
| --- | --- |
| `--space-1` | `.25rem` |
| `--space-2` | `.5rem` |
| `--space-3` | `.75rem` |
| `--space-4` | `1rem` |
| `--space-5` | `1.25rem` |
| `--space-6` | `1.5rem` |

| token | value | role |
| --- | --- | --- |
| `--radius-s` | `.3rem` | inputs, chips, log frames |
| `--radius-m` | `.45rem` | cards, fieldsets, board frame |

Values finer than `--space-1` (card line gaps, control metrics) stay literal in their single owning rule. They are hairline rhythm, not spacing steps.

## Components

Card: `.card` is the only flat card recipe: surface fill, border, `--radius-m`, grid, tight gap, `--space-2 --space-3` padding. `.room`, `.hrow`, `.tourney-run`, `.livecard`, and `.error` share the recipe through the selector list and keep only their layout extras (margins, link resets, mini board) in their own rules. The card grid makes every direct child its own row: a room card stacks its title row, its chips row, its players versus row, and its series score row in that order. The players line never rides the chips row, and the score never renders as an orphan.

Error: `.error` is the only error region, the offline banner's card shape for the error state: the card recipe with the boundary recolored `--danger` and the ink `--danger` at `--text-xl` weight 600. Every user-facing failure renders through it: login and create form errors, lobby join errors, tournament setup and close errors, the room action status failures, and the not-found page. It never appears as bare colored text again.

Chip: `.chip` is the small tag label: a `--text-muted` boundary (the boundary carries the state, so it clears the meaningful-boundary 3:1 instead of the decorative hairline floor), `--radius-s`, `--text-base` at padded density. `.chip.live` fills with `--accent-soft` and keeps its boundary: the live state reads as tint, terminal states as outline.

Stat line: `.statline` is the per-variant identity row of the home stats block: 1 line per variant in config order, square above hex, each carrying only that variant's own rating, WLD record, and level. The line leads with the variant glyph at the stone colors (square red, hex blue) so the row identifies itself without a word. The record stays the `.wld` chip linking to that variant's history. Ratings come from the latest rating event whose game belongs to a series of that variant. Levels come from series won of that variant. A variant with no games renders `RatingStart`.

Field: `.field` is the label-plus-control pair: grid, hairline gap, `--text-s`. Bare `label` elements alias the recipe so the shipped forms need no markup change. `.narrow` caps a numeric input at 7ch so the box fits its value instead of the sheet default. A segmented option that the machine budget cannot fund renders greyed at `.55` opacity with its radio disabled. It stays present in the list and unselectable, never silently absent. The tournament parallel control derives its disabled set from the same core budget math the submit path enforces. Its checked default falls to the largest feasible count when the configured default would not fund.

Buttons: `.btn` is the primary (accent fill, `--accent-strong` on hover and active, background-colored label). The element alias `button:not(.linklike):not(.btn-quiet)` maps the shipped button elements onto it. `.btn-quiet` is the quiet variant (surface fill, border, text color, `--accent-soft` on hover), carried by the class on anchors and buttons alike (`a.btn-quiet`). `.linklike` stays a plain underlined text button. `button[disabled]` dims.

Focus: interactive elements take `outline: none; box-shadow: 0 0 0 3px var(--accent-strong)` under `:focus-visible`, an interaction signal held off the amber data marks. Coarse-pointer minimum target sizes stay as shipped.

Rail: `.rail` is the only 2-column reading rail a paired surface uses, moves left at 30 percent and stats right at 70, in that order and never swapped. The moves column is a `.rail-scroll` box that scrolls on both axes. Its items are `.mvlist .mv` single moves that never wrap. The list grows to `max-content` so overflow is horizontal, not a reflow. The newest move always leads. A `.rail-top` button rides sticky at the bottom of the scroll box and jumps back to the newest move. It appears wherever the list grows live (room, playback) and is omitted from bounded previews. The stats column carries engine data (mono M-lines) or player delta lines. A truncated preview puts its ellipsis item first, because the cut falls on the newest side. The room page, playback, and every history row are the rail's surfaces.

Motion: the UI is near-zero motion. The only transitions are button and chip color fades, and they sit behind `prefers-reduced-motion: no-preference`.

## Ordering

Every list that accumulates over time renders newest first: the room grids on the home page and lobby (creation time, id descending as the tie-break), the history rows and profile recent games (already played_at descending), the tournament series list (creation time descending), the rail move lists, the room M-line panel, and every engine-stat tail. Engine-stat tails run newest line first: the ellipsis of a truncated tail falls at the bottom, and a truncated opening preview puts its ellipsis first, at the cut.

## Contrast contract

`go run ./cmd/run contrast` fails the build on any miss and recomputes every pair from `:root` at runtime. Pairs beyond the shipped set: `background` on `accent-strong` (hovered button label, 4.5), `text` on `accent-soft` (chip and quiet-hover label, 4.5), `mark` on `background` (live-tournament line at `--text-xl` bold, large text, 3), `accent-strong` on `background` (focus ring over the page, 3). Boundaries split by meaning: a boundary that carries state must clear WCAG 1.4.11's 3:1 (`text-muted` on `background`, `surface`, and `accent-soft` for the chip state boundary). Decorative boundaries (the board lattice, hairlines) stay at the 2.5 floor. Tightest shipped margins on the light palette: `text-muted` on `background` 5.00, `border` on `surface` 2.64, `mark` on `background` 3.60.

## Board

The square board frame takes `--radius-m`. Cells and stones keep the lattice and disc language (border cells, circular stones with x/o glyphs, amber latest ring, color-mixed hover ghost). The hex board is the same lattice in svg: cell polygons stroke `--border` at a non-scaling 1px, exactly the square cell's border weight. The hex frame carries no outer border at all. The lattice alone draws the board. The board-css define in `board.html` consumes the tokens only. Its markup classes are pinned by tests and do not change.

## Captures

The canonical capture contract for every README picture, referenced by the screenshots section and executed through playground/shots.

Every capture renders at the 440x956 viewport, 1 viewport for the whole set, portrait, light theme. Pictures run as numbered pairs, 2 per line in the README, the square member left and the hex member right. Each pair carries 1 caption line with the evidence ids (P01 through P26) that devlog and README reference.

Bot-room pictures come from real engine games: `playground/shots` starts real bot-vs-bot rooms at the easy tier on 1+0 bo3, and the operator polls the room detail endpoint and shoots when the stone count reaches the threshold, 12 stones square, 8 stones hex. Terminal verdicts shoot the already-loaded page after the finish endpoint fires, because a retired room 404s on a fresh load. The hex tournament live board comes from a real run.

Capture order is hex first, then square, so the shared live-boards surface renders hex-pure while only hex rooms are live. The desktop landscape set and the 390px set are retired.

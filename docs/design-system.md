# Design system

The v0.27 re-skin contract. Dark is the base presentation on every surface; no light variant exists. The palette hub stays `internal/server/web/shell.css` `:root`, machine-checked by `make darkcontrast` (playground/darkcontrast, wired into `ci`).

## Identity

Caro is five-in-a-row on a grid. The board lattice, the red and blue stone discs, and the amber last-move ring are the product's identity; the UI is furniture around them. Flat borders carry structure. Zero box shadows except the amber ring. No gradients, no all-caps eyebrow labels, no numbered markers, no middle-dot meta strings, no monospace for human prose.

## Color tokens

The ten shipped anchors never change value. Two ramp steps of the existing accent hue were added; no new hues exist.

| token | value | role |
| --- | --- | --- |
| `--background` | `#101318` | page |
| `--surface` | `#1a1f28` | cards, fieldsets, board fill |
| `--border` | `#566179` | every hairline |
| `--text` | `#e8ebf2` | copy |
| `--text-muted` | `#a4aec2` | status and meta |
| `--accent` | `#58a8ff` | links, primary button fill |
| `--accent-strong` | `#7db8ff` | primary button hover and active fill |
| `--accent-soft` | `#25384f` | accent tint fill (quiet hover, live chip) |
| `--danger` | `#ff7a6e` | errors, armed forfeit |
| `--red` | `#ef5350` | red stones, red clock |
| `--blue` | `#42a5f5` | blue stones, blue clock |
| `--mark` | `#ffb454` | amber, see the amber law |

A second surface depth step (`--surface-raised`) is deliberately absent: depth is one fill plus borders. Add it only when a surface truly needs to float.

### The amber law

`--mark` holds exactly one job per view: the live-attention marker. Latest stone on the room and playback boards, latest mini stone on the tournament live cards, the live-tournament line on the rooms grid, and the global focus ring. Amber never decorates anything else.

## Type

Two voices over two tokens:

| token | value | voice |
| --- | --- | --- |
| `--font-human` | `system-ui, sans-serif` | everything a person reads: prose, names, headings |
| `--font-mono` | `ui-monospace, monospace` | machine data: room ids, bot names on bot-only surfaces, clocks, M-lines, move lists, playback counters |

Mixed surfaces (room page seat labels, history rows, score lines) can carry a human or a bot name, so they stay in the human voice; only surfaces that always render bot data take the machine voice.

Scale (the `1.1`, `1.15`, and `1.25` legacy sizes folded to the nearest step):

| token | value | legacy folds in |
| --- | --- | --- |
| `--text-xs` | `.8rem` | bot log |
| `--text-s` | `.85rem` | labels, status line |
| `--text-m` | `.9rem` | clock labels, narrow table |
| `--text-base` | `1rem` | body, move lists, tables, chips |
| `--text-l` | `1.05rem` | `1.1` guestnote, banner, playback buttons |
| `--text-xl` | `1.2rem` | `1.15` card titles, `1.25` wld badge, h1, errors |
| `--text-2xl` | `1.5rem` | brand, player identity |

The room clock keeps its `2rem` component metric (single-source, the room's hero data), and the amber ring keeps its `3px` spread; both are component metrics, not scale steps.

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

Values finer than `--space-1` (card line gaps, control metrics) stay literal in their single owning rule; they are hairline rhythm, not spacing steps.

## Components

Card: `.card` is the one flat card recipe: surface fill, border, `--radius-m`, grid, tight gap, `--space-2 --space-3` padding. `.room`, `.hrow`, `.tourney-run`, and `.livecard` share the recipe through the selector list and keep only their layout extras (margins, link resets, mini board) in their own rules.

Chip: `.chip` is the small tag label: border, `--radius-s`, `--text-base` at padded density. `.chip.live` fills with `--accent-soft` and drops its border: the live state reads as tint, terminal states as outline.

Field: `.field` is the label-plus-control pair: grid, hairline gap, `--text-s`. Bare `label` elements alias the recipe so the shipped forms need no markup change.

Buttons: `.btn` is the primary (accent fill, `--accent-strong` on hover and active, background-colored label). `.btn-quiet` is the quiet variant (surface fill, border, text color, `--accent-soft` on hover). Element selectors `button:not(.linklike)` and `a.action` alias the two recipes for the shipped markup; `.linklike` stays a plain underlined text button. `button[disabled]` dims.

Focus: interactive elements take `outline: none; box-shadow: 0 0 0 3px var(--mark)` under `:focus-visible` (the amber law). Coarse-pointer minimum target sizes stay as shipped.

Motion: the UI is near-zero motion. The only transitions are button and chip color fades, and they sit behind `prefers-reduced-motion: no-preference`.

## Contrast contract

`make darkcontrast` fails the build on any miss. Pairs beyond the shipped set: `background` on `accent-strong` (hovered button label, 4.5), `text` on `accent-soft` (chip and quiet-hover label, 4.5), `mark` on `background` (focus ring over the page, 3).

## Board

The board frame takes `--radius-m`; cells and stones keep the lattice and disc language (border cells, circular stones, amber latest ring, color-mixed hover ghost). The board-css define in `board.html` consumes the tokens only; its markup classes are pinned by tests and do not change.

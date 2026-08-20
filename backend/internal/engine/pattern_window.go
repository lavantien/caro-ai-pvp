package engine

import (
	"caro-ai-pvp/internal/domain"
)

// Gap-aware threat primitives. A "completion" is an empty cell whose single
// fill turns the line through a player's stone into an exact five (Caro rules:
// exactly five stones, not both ends blocked, no overline extension). Split
// shapes like XX.XX and .XX.X. participate, unlike plain contiguous counting.

// lineState encodes one cell of an 11-cell line segment relative to a player.
// The window spans offsets -5..+5 so any exact-five through the center plus
// both of its end-check cells is fully visible.
const (
	lineOpp   = -1 // opponent stone or off-board
	lineEmpty = 0
	lineOwn   = 1
)

const lineCenter = 5

// extractLine reads the 11 cells centered on (x,y) along (dx,dy). The center
// is always reported as the player's own stone, so callers may query a
// hypothetical placement without mutating the board.
func extractLine(sb *SearchBoard, x, y int, player domain.Player, dx, dy int) [11]int8 {
	var line [11]int8
	for off := -5; off <= 5; off++ {
		i := off + lineCenter
		if off == 0 {
			line[i] = lineOwn
			continue
		}
		nx, ny := x+dx*off, y+dy*off
		if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize {
			line[i] = lineOpp
			continue
		}
		p := sb.PlayerAt(nx, ny)
		switch p {
		case player:
			line[i] = lineOwn
		case domain.PlayerNone:
			line[i] = lineEmpty
		default:
			line[i] = lineOpp
		}
	}
	return line
}

// spanThrough returns the maximal run of own stones containing the center,
// treating the cell at fillIdx (if empty) as own.
func spanThrough(line [11]int8, fillIdx int) (lo, hi int) {
	lo, hi = lineCenter, lineCenter
	for lo > 0 {
		c := line[lo-1]
		if c == lineOwn || (lo-1 == fillIdx && c == lineEmpty) {
			lo--
		} else {
			break
		}
	}
	for hi < 10 {
		c := line[hi+1]
		if c == lineOwn || (hi+1 == fillIdx && c == lineEmpty) {
			hi++
		} else {
			break
		}
	}
	return lo, hi
}

// spanIsFive reports whether the span [lo,hi] is an exact five with at least
// one open end (Caro rules).
func spanIsFive(line [11]int8, lo, hi int) bool {
	if hi-lo+1 != domain.WinLength {
		return false
	}
	beforeBlocked := lo == 0 || line[lo-1] == lineOpp
	afterBlocked := hi == 10 || line[hi+1] == lineOpp
	return !beforeBlocked || !afterBlocked
}

// lineCompletions counts empty cells whose fill makes an exact five through
// the center. Only the cells adjacent to the center's maximal span can ever
// complete it, so at most two candidates are tested. Mutates nothing.
func lineCompletions(line [11]int8) int {
	// A five through the center needs the center plus three more own stones
	// already in the window (the fourth slot is the fill itself).
	own := 0
	for _, v := range line {
		if v == lineOwn {
			own++
		}
	}
	if own < domain.WinLength-1 {
		return 0
	}
	lo, hi := spanThrough(line, -1)
	comps := 0
	for i := max(lo-1, 1); i <= min(hi+1, 9); i++ {
		if line[i] != lineEmpty {
			continue
		}
		l2, h2 := spanThrough(line, i)
		if spanIsFive(line, l2, h2) {
			comps++
		}
	}
	return comps
}

// negateLine returns the same line from the opponent's perspective.
func negateLine(line [11]int8) [11]int8 {
	var out [11]int8
	for i, v := range line {
		switch v {
		case lineOwn:
			out[i] = lineOpp
		case lineOpp:
			out[i] = lineOwn
		}
	}
	return out
}

// fiveCompletionsInDir assumes (x,y) holds player's stone and returns the
// empty cells on the line whose fill makes an exact five through (x,y).
func fiveCompletionsInDir(sb *SearchBoard, x, y int, player domain.Player, dx, dy int) []domain.Position {
	line := extractLine(sb, x, y, player, dx, dy)
	var out []domain.Position
	for i := 1; i <= 9; i++ {
		if line[i] != lineEmpty {
			continue
		}
		lo, hi := spanThrough(line, i)
		if spanIsFive(line, lo, hi) {
			out = append(out, domain.Position{X: x + dx*(i - lineCenter), Y: y + dy*(i - lineCenter)})
		}
	}
	return out
}

// maxCompsAfterFill returns the largest completion count reachable by filling
// a single empty cell on the line.
func maxCompsAfterFill(line [11]int8) int {
	own := 0
	for _, v := range line {
		if v == lineOwn {
			own++
		}
	}
	if own < domain.WinLength-2 {
		return 0
	}
	best := 0
	for i := 1; i <= 9; i++ {
		if line[i] != lineEmpty {
			continue
		}
		if separatedByOpp(line, i) {
			continue
		}
		filled := line
		filled[i] = lineOwn
		if c := lineCompletions(filled); c > best {
			best = c
		}
	}
	return best
}

// separatedByOpp reports whether an opponent stone sits strictly between the
// center and cell i, making it unable to join the center's span.
func separatedByOpp(line [11]int8, i int) bool {
	if i < lineCenter {
		for j := i + 1; j < lineCenter; j++ {
			if line[j] == lineOpp {
				return true
			}
		}
	} else {
		for j := lineCenter + 1; j < i; j++ {
			if line[j] == lineOpp {
				return true
			}
		}
	}
	return false
}

// placementThreats describes what a hypothetical stone at (x,y) creates.
type placementThreats struct {
	comps [4]int // winning completions per direction after the placement
	flex3 bool   // some direction can reach a two-completion four next move
}

// placementComps computes only the per-direction completion counts. Cheap:
// no flex-three reachability scan.
func placementComps(sb *SearchBoard, x, y int, player domain.Player) [4]int {
	var comps [4]int
	for i, dir := range evalDirs {
		line := extractLine(sb, x, y, player, dir[0], dir[1])
		comps[i] = lineCompletions(line)
	}
	return comps
}

// analyzePlacement computes completions plus the flex-three flag. Callers that
// only need four-ness should use placementComps instead.
func analyzePlacement(sb *SearchBoard, x, y int, player domain.Player) placementThreats {
	pt := placementThreats{comps: placementComps(sb, x, y, player)}
	for i, dir := range evalDirs {
		if pt.comps[i] == 0 && !pt.flex3 {
			line := extractLine(sb, x, y, player, dir[0], dir[1])
			pt.flex3 = maxCompsAfterFill(line) >= 2
		}
	}
	return pt
}

func (pt placementThreats) openFour() bool {
	for _, c := range pt.comps {
		if c >= 2 {
			return true
		}
	}
	return false
}

func (pt placementThreats) four() bool {
	for _, c := range pt.comps {
		if c >= 1 {
			return true
		}
	}
	return false
}

// createsOpenFour reports whether placing player at (x,y) creates an open
// four: at least two distinct winning completions (straight .XXXX. or split
// XX.XX shapes alike).
func createsOpenFour(sb *SearchBoard, x, y int, player domain.Player) bool {
	for _, c := range placementComps(sb, x, y, player) {
		if c >= 2 {
			return true
		}
	}
	return false
}

// createsFourType reports whether placing player at (x,y) creates any four:
// a shape one move away from an exact five, gapped or straight.
func createsFourType(sb *SearchBoard, x, y int, player domain.Player) bool {
	for _, c := range placementComps(sb, x, y, player) {
		if c >= 1 {
			return true
		}
	}
	return false
}

// createsOpenThree reports whether placing player at (x,y) creates an open
// three: a shape (straight or broken) that can become an open four next move.
func createsOpenThree(sb *SearchBoard, x, y int, player domain.Player) bool {
	for _, dir := range evalDirs {
		line := extractLine(sb, x, y, player, dir[0], dir[1])
		if maxCompsAfterFill(line) >= 2 {
			return true
		}
	}
	return false
}

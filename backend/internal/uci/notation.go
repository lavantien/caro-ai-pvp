package uci

func MoveToString(x, y int) string {
	return string(rune('a'+y)) + string(rune('a'+x))
}

func ParseMove(s string) (int, int, bool) {
	if len(s) < 2 {
		return 0, 0, false
	}
	y := int(s[0] - 'a')
	x := int(s[1] - 'a')
	if x < 0 || x >= 16 || y < 0 || y >= 16 {
		return 0, 0, false
	}
	return x, y, true
}

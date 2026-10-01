package fixture

func Add(a, b int) int {
	return a + b
}

func Sub(a, b int) int {
	return a - b
}

func Cmp(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

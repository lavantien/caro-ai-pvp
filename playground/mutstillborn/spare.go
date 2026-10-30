package mutstillborn

func Spare(n int) int {
	s := make([]int, 0, 4)
	s = append(s, n)
	return len(s)
}

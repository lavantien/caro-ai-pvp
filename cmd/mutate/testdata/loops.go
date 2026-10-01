package fixture

func Sum(n int) int {
	total := 0
	for i := 1; i <= n; i++ {
		if total > 100 {
			break
		}
		total += i
	}
	return total
}

func Count(values []int, target int) int {
	found := 0
	for _, v := range values {
		if v != target {
			continue
		}
		found++
	}
	return found
}

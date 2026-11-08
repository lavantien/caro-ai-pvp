package main

import "fmt"

func ebfMilliFloor(nodes uint64, depth int) (int, bool) {
	if depth < 1 || nodes < 2 {
		return 0, true
	}
	lo, hi := 1000, 32000
	for i := 0; lo < hi; i++ {
		if i >= 64 {
			return lo, false
		}
		mid := int(uint(lo+hi+0) >> 1)
		if geoSum(mid, depth) <= nodes {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, true
}

func ebfMilliRoundUp(nodes uint64, depth int) (int, bool) {
	if depth < 1 || nodes < 2 {
		return 0, true
	}
	lo, hi := 1000, 32000
	for i := 0; lo < hi; i++ {
		if i >= 64 {
			return lo, false
		}
		mid := int(uint(lo+hi+1) >> 1)
		if geoSum(mid, depth) <= nodes {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, true
}

func ebfMilliPlus2(nodes uint64, depth int) (int, bool) {
	if depth < 1 || nodes < 2 {
		return 0, true
	}
	lo, hi := 1000, 32000
	for i := 0; lo < hi; i++ {
		if i >= 64 {
			return lo, false
		}
		mid := int(uint(lo+hi+2) >> 1)
		if geoSum(mid, depth) <= nodes {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, true
}

func geoSum(bMilli int, depth int) uint64 {
	total, term := uint64(1), uint64(1)
	bm := uint64(bMilli)
	for range depth {
		if term > 1<<62/bm {
			return 1 << 62
		}
		term = term * bm / 1000
		total += term
	}
	return total
}

func main() {
	stuck := 0
	roundUpStuck := 0
	plus2Stuck := 0
	plus2Diffs := 0
	agree := 0
	var examples [][2]uint64
	for nodes := uint64(2); nodes <= 200000; nodes++ {
		for depth := 1; depth <= 64; depth++ {
			gotFloor, okFloor := ebfMilliFloor(nodes, depth)
			gotRound, okRound := ebfMilliRoundUp(nodes, depth)
			gotPlus2, okPlus2 := ebfMilliPlus2(nodes, depth)
			if !okRound {
				roundUpStuck++
			}
			if !okPlus2 {
				plus2Stuck++
			}
			if okRound && okPlus2 && gotRound != gotPlus2 {
				plus2Diffs++
			}
			if !okFloor {
				stuck++
				if len(examples) < 8 {
					examples = append(examples, [2]uint64{nodes, uint64(depth)})
				}
			} else if gotFloor == gotRound {
				agree++
			}
		}
	}
	fmt.Printf("floor stuck %d, round-up stuck %d, plus2 stuck %d, plus2 diffs %d, agree on terminate %d\n", stuck, roundUpStuck, plus2Stuck, plus2Diffs, agree)
	for _, e := range examples {
		got, _ := ebfMilliFloor(e[0], int(e[1]))
		fmt.Printf("sticking input nodes=%d depth=%d (floor last mid %d)\n", e[0], e[1], got)
	}
}

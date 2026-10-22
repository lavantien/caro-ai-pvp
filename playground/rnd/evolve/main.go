package main

func main() {
	printRows(runBench(buildBench(decReference()*1.1+0.02), seedList()))
}

func seedList() []int64 {
	out := make([]int64, seeds)
	for i := range out {
		out[i] = int64(1000 + i)
	}
	return out
}

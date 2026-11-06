package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	lines := flag.Int("lines", 5000, "stdout lines to emit")
	every := flag.Duration("every", time.Millisecond, "pause between lines, 0 bursts")
	lineBytes := flag.Int("lineBytes", 96, "padded line length in bytes")
	stderrEvery := flag.Int("stderrEvery", 0, "emit one stderr line every n stdout lines, 0 stays silent")
	exit := flag.Int("exit", 7, "process exit code")
	progress := flag.String("progress", "", "sidecar file receiving wall timestamps per line")
	flag.Parse()

	pad := make([]byte, *lineBytes)
	for i := range pad {
		pad[i] = 'x'
	}
	var f *os.File
	if *progress != "" {
		var err error
		f, err = os.Create(*progress)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer f.Close()
	}
	start := time.Now()
	for i := 1; i <= *lines; i++ {
		line := "g1 M" + strconv.Itoa(i) + ", Red, " + string(pad)
		fmt.Println(line)
		if *stderrEvery > 0 && i%*stderrEvery == 0 {
			fmt.Fprintln(os.Stderr, "stderr note "+strconv.Itoa(i))
		}
		if f != nil {
			if _, err := fmt.Fprintf(f, "%d %s\n", i, time.Since(start)); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		}
		if *every > 0 {
			time.Sleep(*every)
		}
	}
	os.Exit(*exit)
}

package main

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
)

func main() {
	winStates := 0
	winUnder := 0
	aboveStates := 0
	aboveShape := 0
	for idx := range config.PatternTableEntries {
		own, opp, empty := 0, 0, 0
		for _, s := range pattern.Unpack(uint32(idx)) {
			switch s {
			case config.PatternStateOwn:
				own++
			case config.PatternStateOpp:
				opp++
			case config.PatternStateEmpty:
				empty++
			}
		}
		cls := pattern.Lookup(uint32(idx)).Class
		if own == 4 && opp == 0 && empty == 1 {
			winStates++
			if cls >= config.PatternClassFour {
				winUnder++
			} else {
				fmt.Printf("MISSING four class: idx %d own %d opp %d empty %d class %d\n", idx, own, opp, empty, cls)
			}
		}
		if cls >= config.PatternClassFour {
			aboveStates++
			if !(own == 4 && opp == 0 && empty == 1) {
				aboveShape++
				fmt.Printf("class >= four with other shape: idx %d own %d opp %d empty %d class %d\n", idx, own, opp, empty, cls)
			}
		}
	}
	fmt.Printf("four-shape states %d, of them class>=four %d\n", winStates, winUnder)
	fmt.Printf("class>=four states %d, of them four-shape %d\n", aboveStates, aboveStates-aboveShape)
}

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

var openings = [][]string{
	{"H8"},
	{"H8", "H9"},
	{"H8", "I9"},
	{"H8", "H6", "H5", "I6", "J6", "I7"},
	{"H8", "H6", "H5", "I6", "J6", "I7", "J8", "J5", "I8", "G8", "L8", "K8"},
}

func main() {
	boards := openings
	if len(os.Args) > 1 {
		boards = [][]string{os.Args[1:]}
	}
	for _, cells := range boards {
		b := rules.NewBoard()
		for _, c := range cells {
			cell, err := rules.ParseCell(c)
			if err != nil {
				panic(err)
			}
			b.Make(cell)
		}
		for _, kind := range []vcf.Kind{vcf.KindVCF, vcf.KindVCT} {
			s := vcf.New(kind)
			var out vcf.SolverStats
			start := time.Now()
			proved := s.Solve(b, config.SolverNodeBudget, nil, &out)
			fmt.Printf("stones=%d kind=%d proved=%v nodes=%d elapsed=%s cells=%v\n",
				len(cells), kind, proved, out.Nodes, time.Since(start).Round(time.Millisecond), cells)
		}
	}
}

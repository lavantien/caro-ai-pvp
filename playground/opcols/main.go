package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	for _, dir := range os.Args[1:] {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				fmt.Println(path, "parse error:", err)
				return nil
			}
			ast.Inspect(f, func(n ast.Node) bool {
				e, ok := n.(*ast.BinaryExpr)
				if !ok || e.Op != token.ADD {
					return true
				}
				p := fset.Position(e.OpPos)
				fmt.Printf("%s:%d:%d\n", filepath.ToSlash(path), p.Line, p.Column)
				return true
			})
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "walk", dir, err)
			os.Exit(1)
		}
	}
}

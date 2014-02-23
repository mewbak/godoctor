package cfg_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"golang-refactoring.org/go-doctor/extras/cfg"
)

func ExampleCFG() {
	src := `
        package main

        import "fmt"

        func main() {
          for {
            if 1 > 0 {
              fmt.Println("my computer works")
            } else {
              fmt.Println("something has gone terribly wrong")
            }
          }
        }
      `

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	funcOne := f.Decls[1].(*ast.FuncDecl)
	c := cfg.FuncCFG(funcOne)

	ast.Inspect(f, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.IfStmt:
			s := c.Succs(stmt)
			p := c.Preds(stmt)

			fmt.Println(len(s))
			fmt.Println(len(p))
		}
		return true
	})
	// Output:
	// 2
	// 1
}

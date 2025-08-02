// +build ignore

// This file helps identify what needs to be exported
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"
)

func main() {
	// Packages to analyze
	packages := []string{"core", "gpu", "physics", "rendering", "simulation"}
	
	for _, pkg := range packages {
		fmt.Printf("\n=== Package %s ===\n", pkg)
		files, _ := filepath.Glob(pkg + "/*.go")
		
		for _, file := range files {
			src, _ := ioutil.ReadFile(file)
			fset := token.NewFileSet()
			f, _ := parser.ParseFile(fset, file, src, 0)
			
			// Find unexported types and functions
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.TypeSpec:
					name := x.Name.Name
					if name[0] >= 'a' && name[0] <= 'z' {
						fmt.Printf("  Unexported type: %s in %s\n", name, filepath.Base(file))
					}
				case *ast.FuncDecl:
					if x.Recv == nil { // Only top-level functions
						name := x.Name.Name
						if name[0] >= 'a' && name[0] <= 'z' {
							fmt.Printf("  Unexported func: %s in %s\n", name, filepath.Base(file))
						}
					}
				}
				return true
			})
		}
	}
}
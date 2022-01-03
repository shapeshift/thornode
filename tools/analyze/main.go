package main

import (
	"errors"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

////////////////////////////////////////////////////////////////////////////////////////
// MapIteration
////////////////////////////////////////////////////////////////////////////////////////

func MapIteration(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// track lines with the expected ignore comment
	type ignorePos struct {
		file string
		line int
	}
	ignore := map[ignorePos]bool{}

	// one pass to find all comments
	inspect.Preorder([]ast.Node{(*ast.File)(nil)}, func(node ast.Node) {
		n := node.(*ast.File)
		for _, c := range n.Comments {
			if strings.Contains(c.Text(), "analyze-ignore(map-iteration)") {
				p := pass.Fset.Position(c.Pos())
				ignore[ignorePos{p.Filename, p.Line + strings.Count(c.Text(), "\n")}] = true
			}
		}
	})

	inspect.Preorder([]ast.Node{(*ast.RangeStmt)(nil)}, func(node ast.Node) {
		n := node.(*ast.RangeStmt)
		// skip if this is not a range over a map
		if !strings.HasPrefix(pass.TypesInfo.TypeOf(n.X).String(), "map") {
			return
		}

		// skip if this is a test file
		p := pass.Fset.Position(n.Pos())
		if strings.HasSuffix(p.Filename, "_test.go") {
			return
		}

		// skip if the previous line contained the ignore comment
		if ignore[ignorePos{p.Filename, p.Line}] {
			return
		}

		pass.Reportf(node.Pos(), "found map iteration")
	})

	return nil, nil
}

////////////////////////////////////////////////////////////////////////////////////////
// Main
////////////////////////////////////////////////////////////////////////////////////////

func main() {
	multichecker.Main(
		&analysis.Analyzer{
			Name:     "map_iteration",
			Doc:      "fails on uncommented map iterations",
			Requires: []*analysis.Analyzer{inspect.Analyzer},
			Run:      MapIteration,
		},
	)
}

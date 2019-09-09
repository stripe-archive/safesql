// Command safesql is a tool for performing static analysis on programs to
// ensure that SQL injection attacks are not possible. It does this by ensuring
// package database/sql is only used with compile-time constant queries.
package safesql

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

const Doc = `ensure SQL injection attacks are not possible

The safesql analysis reports calls to DB functions are only made with constant strings.`

var Analyzer = &analysis.Analyzer{
	Name: "safesql",
	Doc:  Doc,
	Run:  run,
	Requires: []*analysis.Analyzer{
		buildssa.Analyzer,
		inspect.Analyzer,
	},
	FactTypes: []analysis.Fact{new(unsafeCallFact)},
}

// unsafeCallFact represents a call to a SQL execution function that isn't a
// provably constant string.
type unsafeCallFact struct {
	Pos token.Pos
}

func (*unsafeCallFact) String() string { return "found" }
func (*unsafeCallFact) AFact()         {}

// run performs the safesql analysis on a single package; it may be called
// multiple times during a single execution of the binary, once per dependency.
func run(pass *analysis.Pass) (interface{}, error) {

	// package database/sql has a couple helper functions which are thin
	// wrappers around other sensitive functions. Instead of handling the
	// general case by tracing down callsites of wrapper functions
	// recursively, let's just allowlist these DB packages, since it
	// happens to be good enough for our use case.
	for _, sql := range sqlPackages {
		if strings.HasPrefix(pass.Pkg.Path(), sql.packageName) {
			return nil, nil
		}
	}

	log.Printf("-- %s --\n", pass.Pkg.Path())

	// TODO: we should only need one of these
	var err error
	err = CheckSafeSqlSsa(pass)
	err = CheckSafeSqlAst(pass)

	return nil, err
}

// This more closely matches the original safesql implementation, but doesn't
// actually work.  See the big comment in the middle for details
func CheckSafeSqlSsa(pass *analysis.Pass) error {
	// we listed this as a dependency above; it is guaranteed to have run
	ssaPass := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	prog := ssaPass.Pkg.Prog
	prog.Build()

	qms := make([]*QueryMethod, 0)
	for _, sql := range sqlPackages {
		var pkg *ssa.Package
		for _, usedPkg := range prog.AllPackages() {
			if usedPkg.Pkg.Path() == sql.packageName {
				pkg = usedPkg
				break
			}
		}
		// the SQL package we were worried about isn't used in this module!
		if pkg == nil {
			continue
		}
		qms = append(qms, FindQueryMethods(sql, pkg.Pkg, prog)...)
	}

	if pass.Pkg.Path() == "a_pass" {
		for _, fn := range ssaPass.SrcFuncs {
			log.Printf("srcfunc: %s", fn.Name())
		}
	}

	// the pointer.Analyze function below only works on packages with that
	// _literally_ have main functions.
	if ssaPass.Pkg.Func("main") == nil {
		return nil
	}

	res, err2 := pointer.Analyze(&pointer.Config{
		Mains:          []*ssa.Package{ssaPass.Pkg},
		BuildCallGraph: true,
		// Log:            os.Stdout,
	})
	if err2 != nil {
		fmt.Printf("error performing pointer analysis: %v\n", err2)
		os.Exit(2)
	}

	// XXX: at this point, the callgraph doesn't contain edges from our SQL
	// callsites to e.g. DB.Exec.  I think there are two explanations: 1) it
	// is a Go modules thing.  2) it is something that broke when moving away from
	// the deprecated loader package.  I am pretty sure it is the second -- I
	// rebuilt my local go as go1.11.13, ran `export GO111MODULE=off` in a terminal
	// and ran the test, and still see the same behavior below.

	// for example, when running the test, we see:
	//
	// fn main -- []*callgraph.Edge{(*callgraph.Edge)(0xc00dbb4d20)}
	//   n5:a_pass.main --> n6:a_pass.runDbQuery
	// fn runDbQuery -- []*callgraph.Edge{}
	//
	// main is shown to have a single edge, to runDbQuery, and runDbQuery has
	// no edges.  This is wrong on both accounts - main also has a call to log.Printf,
	// and runDbQuery has a call to DB.Exec.

	bad := FindNonConstCalls(res.CallGraph, qms)
	log.Printf("!! found %v non-const calls", bad)

	for _, ci := range bad {
		pos := prog.Fset.Position(ci.Pos())
		fmt.Printf("- %s\n", pos)
	}

	var err error
	if len(bad) > 0 {
		err = fmt.Errorf("found %d safesql errors", len(bad))
	}

	return err
}

// This was my first approach at a Go 1.13+ version of safesql; the problem
// here is that the AST is very high level; if you have a package-level const
// string, the functions like db.Exec will receive an identifier, not a string
// literal.  I guess we could look up the identifier, and see if it resolves
// immediately to a string literal?  That might be an easy way to match the
// current behavior, but IDK if it will be easy to extend to more things that
// act as false positives today.
func CheckSafeSqlAst(pass *analysis.Pass) error {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		&ast.CallExpr{},
	}

	nErrors := 0
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)
		fn, ok := typeutil.Callee(pass.TypesInfo, call).(*types.Func)
		if !ok {
			// log.Printf("call Fun not a Func? %#v\n", call.Fun)
			return
		}

		for _, sql := range sqlPackages {
			if fn.Pkg() != nil && fn.Pkg().Path() != sql.packageName {
				continue
			}

			sig := fn.Type().(*types.Signature)
			params := sig.Params()
			for i := 0; i < params.Len(); i++ {
				v := params.At(i)
				if _, ok := sql.paramNames[v.Name()]; !ok {
					continue
				}
				arg := call.Args[i]
				lit, ok := arg.(*ast.BasicLit)
				if !ok {
					nErrors++
					// this will trigger even for _identifiers_ that point to static strings
					pass.Reportf(arg.Pos(), "SQL query with non-static argument: %s", arg)
					continue
				}
				if lit.Kind != token.STRING {
					nErrors++
					pass.Reportf(arg.Pos(), "SQL query with non-string literal: %s", arg)
					log.Printf("bad bad")
					continue
				}
				log.Printf("all good")
			}
		}
	})

	var err error
	if nErrors != 0 {
		err = errors.New("potentially unsafe SQL queries found")
	}

	return err
}

type sqlPackage struct {
	packageName string
	paramNames  map[string]struct{}
	enable      bool
	pkg         *ssa.Package
}

var sqlPackages = []sqlPackage{
	{
		packageName: "database/sql",
		paramNames: map[string]struct{}{
			"query": {},
		},
	},
	{
		packageName: "github.com/jinzhu/gorm",
		paramNames: map[string]struct{}{
			"sql":   {},
			"query": {},
		},
	},
	{
		packageName: "github.com/jmoiron/sqlx",
		paramNames: map[string]struct{}{
			"query": {},
		},
	},
}

// QueryMethod represents a method on a type which has a string parameter named
// "query".
type QueryMethod struct {
	Func     *types.Func
	SSA      *ssa.Function
	ArgCount int
	Param    int
}

// FindQueryMethods locates all methods in the given package (assumed to be
// package database/sql) with a string parameter named "query".
func FindQueryMethods(sqlPackages sqlPackage, sql *types.Package, ssa *ssa.Program) []*QueryMethod {
	methods := make([]*QueryMethod, 0)
	scope := sql.Scope()
	for _, name := range scope.Names() {
		o := scope.Lookup(name)
		if !o.Exported() {
			continue
		}
		if _, ok := o.(*types.TypeName); !ok {
			continue
		}
		n := o.Type().(*types.Named)
		for i := 0; i < n.NumMethods(); i++ {
			m := n.Method(i)
			if !m.Exported() {
				continue
			}
			s := m.Type().(*types.Signature)
			if num, ok := FuncHasQuery(sqlPackages, s); ok {
				fn := ssa.FuncValue(m)
				methods = append(methods, &QueryMethod{
					Func:     m,
					SSA:      fn,
					ArgCount: s.Params().Len(),
					Param:    num,
				})
			}
		}
	}
	return methods
}

// FuncHasQuery returns the offset of the string parameter named "query", or
// none if no such parameter exists.
func FuncHasQuery(sqlPackages sqlPackage, s *types.Signature) (offset int, ok bool) {
	params := s.Params()
	for i := 0; i < params.Len(); i++ {
		v := params.At(i)
		if _, ok := sqlPackages.paramNames[v.Name()]; ok {
			return i, true
		}
	}
	return 0, false
}

// FindNonConstCalls returns the set of callsites of the given set of methods
// for which the "query" parameter is not a compile-time constant.
func FindNonConstCalls(cg *callgraph.Graph, qms []*QueryMethod) []ssa.CallInstruction {
	cg.DeleteSyntheticNodes()

	// package database/sql has a couple helper functions which are thin
	// wrappers around other sensitive functions. Instead of handling the
	// general case by tracing down callsites of wrapper functions
	// recursively, let's just whitelist the functions we're already
	// tracking, since it happens to be good enough for our use case.
	okFuncs := make(map[*ssa.Function]struct{}, len(qms))
	for _, m := range qms {
		okFuncs[m.SSA] = struct{}{}
	}

	for fn, node := range cg.Nodes {
		if fn.Name() == "main" || fn.Name() == "runDbQuery" {
			fmt.Printf("fn %s -- %#v\n", fn.Name(), node.Out)
			for _, out := range node.Out {
				fmt.Printf("  %s\n", out)
			}
		}
	}

	bad := make([]ssa.CallInstruction, 0)
	for _, m := range qms {
		node := cg.Nodes[m.SSA]
		if node == nil {
			continue
		}

		fmt.Printf("func %s contains callees %#v\n", m.Func, node.In)
		for _, edge := range node.In {
			fmt.Printf("found an edge\n")
			if _, ok := okFuncs[edge.Site.Parent()]; ok {
				continue
			}

			isInternalSQLPkg := false
			for _, pkg := range sqlPackages {
				if pkg.packageName == edge.Caller.Func.Pkg.Pkg.Path() {
					isInternalSQLPkg = true
					break
				}
			}
			if isInternalSQLPkg {
				continue
			}

			cc := edge.Site.Common()
			args := cc.Args
			// The first parameter is occasionally the receiver.
			if len(args) == m.ArgCount+1 {
				args = args[1:]
			} else if len(args) != m.ArgCount {
				panic("arg count mismatch")
			}
			v := args[m.Param]
			fmt.Printf("found the call!!\n")

			if _, ok := v.(*ssa.Const); !ok {
				if inter, ok := v.(*ssa.MakeInterface); ok && types.IsInterface(v.(*ssa.MakeInterface).Type()) {
					if inter.X.Referrers() == nil || inter.X.Type() != types.Typ[types.String] {
						continue
					}
				}

				bad = append(bad, edge.Site)
			}
		}
	}

	return bad
}

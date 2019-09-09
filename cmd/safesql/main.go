// Command safesql is a tool for performing static analysis on programs to
// ensure that SQL injection attacks are not possible. It does this by ensuring
// package database/sql is only used with compile-time constant queries.
package main

import (
	"github.com/bpowers/safesql"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(safesql.Analyzer)
}

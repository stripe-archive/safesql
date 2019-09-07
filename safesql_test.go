package safesql_test

import (
	"testing"

	"github.com/bpowers/safesql"
	"golang.org/x/tools/go/analysis/analysistest"
)

func init() {
	safesql.Analyzer.Flags.Set("name", "safesql")
}

func TestFromFileSystem(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, safesql.Analyzer, "a_pass")
}

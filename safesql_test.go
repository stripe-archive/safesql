package main

import (
	"go/token"
	"path"
	"reflect"
	"sort"
	"testing"
)

const testDir = "./testdata"

// TestCheckIssues attempts to see if issues are ignored or not and annotates the issues if they are ignored
func TestCheckIssues(t *testing.T) {
	tests := map[string]struct{
		tokens []token.Position
		expected []Issue
	}{
		"all_ignored": {
			tokens: []token.Position{
				token.Position{Filename:"main.go", Line: 23, Column: 5 },
				token.Position{Filename:"main.go", Line: 29, Column: 5 },
			},
			expected: []Issue{
				Issue{statement: token.Position{Filename:"main.go", Line: 23, Column: 5 }, ignored: true},
				Issue{statement: token.Position{Filename:"main.go", Line: 29, Column: 5 }, ignored: true},
			},
		},
		"ignored_back_to_back": {
			tokens: []token.Position{
				token.Position{Filename:"main.go", Line: 22, Column: 5 },
				token.Position{Filename:"main.go", Line: 23, Column: 5 },
			},
			expected: []Issue{
				Issue{statement: token.Position{Filename:"main.go", Line: 22, Column: 5 }, ignored: true},
				Issue{statement: token.Position{Filename:"main.go", Line: 23, Column: 5 }, ignored: false},
			},
		},
		"single_ignored": {
			tokens: []token.Position{
				token.Position{Filename:"main.go", Line: 23, Column: 5 },
				token.Position{Filename:"main.go", Line: 29, Column: 5 },
			},
			expected: []Issue{
				Issue{statement: token.Position{Filename:"main.go", Line: 23, Column: 5 }, ignored: true},
				Issue{statement: token.Position{Filename:"main.go", Line: 29, Column: 5 }, ignored: false},
			},
		},
		"multiple_files": {
			tokens: []token.Position{
				token.Position{Filename:"main.go", Line: 23, Column: 5 },
				token.Position{Filename:"main.go", Line: 24, Column: 5 },
				token.Position{Filename:"helpers.go", Line: 16, Column: 5 },
				token.Position{Filename:"helpers.go", Line: 17, Column: 5 },
			},
			expected: []Issue{
				Issue{statement: token.Position{Filename:"main.go", Line: 23, Column: 5 }, ignored: true},
				Issue{statement: token.Position{Filename:"main.go", Line: 24, Column: 5 }, ignored: true},
				Issue{statement: token.Position{Filename:"helpers.go", Line: 16, Column: 5 }, ignored: true},
				Issue{statement: token.Position{Filename:"helpers.go", Line: 17, Column: 5 }, ignored: true},
			},
		},
	}

	for name, expectations := range tests {
		t.Run(name, func(t *testing.T) {
			for idx, pos := range expectations.tokens {
				expectations.tokens[idx].Filename = path.Join(testDir, name, pos.Filename)
			}
			for idx, issue := range expectations.expected {
				expectations.expected[idx].statement.Filename = path.Join(testDir, name, issue.statement.Filename)
			}

			issues, err := CheckIssues(expectations.tokens)
			if err != nil {
				t.Fatal(err)
			}

			if len(issues) != len(expectations.expected) {
				t.Fatal("The expected number of issues was not found")
			}

			// sort to ensure we have the same issue order
			sort.Slice(expectations.expected, func(i, j int) bool {return expectations.expected[i].statement.Filename < expectations.expected[j].statement.Filename })
			sort.Slice(issues, func(i, j int) bool {return issues[i].statement.Filename < issues[j].statement.Filename })

			for idx, expected := range expectations.expected {
				actual := issues[idx]
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("The actual value of %v did not match the expected %v", actual, expected)
				}
			}
		})
	}
}
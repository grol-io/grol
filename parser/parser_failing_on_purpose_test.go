//go:build runfailingtests
// +build runfailingtests

package parser

import (
	"testing"

	"fortio.org/log"
	"github.com/ldemailly/gorpl/lexer"
)

// show the interface nil check bug (fixed now) - test for error.
func TestLetStatementsFormerlyCrashingNowFailingOnPurpose(t *testing.T) {
	log.SetLogLevelQuiet(log.Debug)
	log.Config.ForceColor = true
	log.SetColorMode()
	input := `
let x; = 5;
let y = 10;
let foobar = 838383;
`
	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)
	if program == nil {
		t.Fatalf("ParseProgram() returned nil")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("program.Statements does not contain 3 statements. got=%d",
			len(program.Statements))
	}

	tests := []struct {
		expectedIdentifier string
	}{
		{"x"},
		{"y"},
		{"foobar"},
	}

	for i, tt := range tests {
		stmt := program.Statements[i]
		if !testLetStatement(t, stmt, tt.expectedIdentifier) {
			return
		}
	}
}

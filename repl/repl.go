package repl

import (
	"bufio"
	"fmt"
	"io"

	"github.com/ldemailly/gorpl/lexer"
	"github.com/ldemailly/gorpl/parser"
)

const PROMPT = "$ "

func printParserErrors(out io.Writer, p *parser.Parser) bool {
	errors := p.Errors()
	if len(errors) == 0 {
		return false
	}

	fmt.Fprintf(out, "parser has %d error(s)\n", len(errors))
	for _, msg := range errors {
		fmt.Fprintf(out, "parser error: %s\n", msg)
	}
	return true
}

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)

	for {
		fmt.Fprint(out, PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		l := lexer.New(line)

		p := parser.New(l)
		program := p.ParseProgram()
		if printParserErrors(out, p) {
			continue
		}
		fmt.Println(program.String())
	}
}

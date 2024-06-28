package repl

import (
	"bufio"
	"fmt"
	"io"

	"fortio.org/log"
	"github.com/ldemailly/gorpl/eval"
	"github.com/ldemailly/gorpl/lexer"
	"github.com/ldemailly/gorpl/parser"
)

const PROMPT = "$ "

func logParserErrors(p *parser.Parser) bool {
	errors := p.Errors()
	if len(errors) == 0 {
		return false
	}

	log.Critf("parser has %d error(s)", len(errors))
	for _, msg := range errors {
		log.Errf("parser error: %s", msg)
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
		if logParserErrors(p) {
			continue
		}
		fmt.Print("== Parse ==> ")
		fmt.Println(program.String())
		fmt.Print("== Eval  ==> ")
		obj := eval.Eval(program)
		fmt.Println(obj.Inspect())
	}
}

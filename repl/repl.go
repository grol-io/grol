package repl

import (
	"bufio"
	"fmt"
	"io"

	"fortio.org/log"
	"github.com/ldemailly/gorepl/eval"
	"github.com/ldemailly/gorepl/lexer"
	"github.com/ldemailly/gorepl/object"
	"github.com/ldemailly/gorepl/parser"
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
	s := eval.NewState()
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
		obj := s.Eval(program)
		if obj.Type() == object.ERROR {
			fmt.Print(log.Colors.Red)
		} else {
			fmt.Print(log.Colors.Green)
		}
		fmt.Println(obj.Inspect())
		fmt.Print(log.ANSIColors.Reset)
	}
}

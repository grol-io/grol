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

type Options struct {
	ShowParse bool
	ShowEval  bool
	All       bool
}

func EvalAll(s *eval.State, in io.Reader, out io.Writer, options Options) {
	b, err := io.ReadAll(in)
	if err != nil {
		log.Fatalf("%v", err)
	}
	what := string(b)
	EvalOne(s, what, out, options)
}

func Interactive(in io.Reader, out io.Writer, options Options) {
	s := eval.NewState()
	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}
		l := scanner.Text()
		EvalOne(s, l, out, options)
	}
}

func EvalOne(s *eval.State, what string, out io.Writer, options Options) {
	l := lexer.New(what)
	p := parser.New(l)
	program := p.ParseProgram()
	if logParserErrors(p) {
		return
	}
	if options.ShowParse {
		fmt.Fprint(out, "== Parse ==> ")
		fmt.Fprintln(out, program.String())
	}
	if !options.ShowEval {
		return
	}
	if options.ShowParse {
		fmt.Fprint(out, "== Eval  ==> ")
	}
	obj := s.Eval(program)
	if obj.Type() == object.ERROR {
		fmt.Fprint(out, log.Colors.Red)
	} else {
		fmt.Fprint(out, log.Colors.Green)
	}
	fmt.Fprintln(out, obj.Inspect())
	fmt.Fprint(out, log.ANSIColors.Reset)
}

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

const (
	PROMPT       = "$ "
	CONTINUATION = "> "
)

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

func EvalAll(s, macroState *eval.State, in io.Reader, out io.Writer, options Options) {
	b, err := io.ReadAll(in)
	if err != nil {
		log.Fatalf("%v", err)
	}
	what := string(b)
	EvalOne(s, macroState, what, out, options)
}

func Interactive(in io.Reader, out io.Writer, options Options) {
	s := eval.NewState()
	macroState := eval.NewState()

	scanner := bufio.NewScanner(in)
	prev := ""
	prompt := PROMPT
	for {
		fmt.Fprint(out, prompt)
		scanned := scanner.Scan()
		if !scanned {
			return
		}
		l := prev + scanner.Text()
		contNeeded := EvalOne(s, macroState, l, out, options)
		if contNeeded {
			prev = l + "\n"
			prompt = CONTINUATION
		} else {
			prev = ""
			prompt = PROMPT
		}
	}
}

// Returns true in line mode if more should be fed to the parser.
func EvalOne(s, macroState *eval.State, what string, out io.Writer, options Options) bool {
	var l *lexer.Lexer
	if options.All {
		l = lexer.New(what)
	} else {
		l = lexer.NewLineMode(what)
	}
	p := parser.New(l)
	program := p.ParseProgram()
	if logParserErrors(p) {
		return false
	}
	if p.ContinuationNeeded() {
		return true
	}
	if options.ShowParse {
		fmt.Fprint(out, "== Parse ==> ")
		fmt.Fprintln(out, program.String())
	}
	macroState.DefineMacros(program)
	numMacros := macroState.Len()
	if numMacros > 0 {
		log.LogVf("Expanding, %d macros defined", numMacros)
		// This actually modifies the original program, not sure... that's good but that's why
		// expanded return value doesn't need to be used.
		_ = macroState.ExpandMacros(program)
		if options.ShowParse {
			fmt.Fprint(out, "== Macro ==> ")
			fmt.Fprintln(out, program.String())
		}
	} else {
		log.LogVf("Skipping macro expansion as none are defined")
	}
	if options.ShowParse && options.ShowEval {
		fmt.Fprint(out, "== Eval  ==> ")
	}
	obj := s.Eval(program)
	if !options.ShowEval {
		return false
	}
	if obj.Type() == object.ERROR {
		fmt.Fprint(out, log.Colors.Red)
	} else {
		fmt.Fprint(out, log.Colors.Green)
	}
	fmt.Fprintln(out, obj.Inspect())
	fmt.Fprint(out, log.ANSIColors.Reset)
	return false
}

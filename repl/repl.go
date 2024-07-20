package repl

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"fortio.org/log"
	"grol.io/grol/eval"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
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
	NoColor   bool // color controlled by log package, unless this is set to true.
}

func EvalAll(s, macroState *eval.State, in io.Reader, out io.Writer, options Options) {
	b, err := io.ReadAll(in)
	if err != nil {
		log.Fatalf("%v", err)
	}
	what := string(b)
	EvalOne(s, macroState, what, out, options)
}

// EvalString can be used from playground etc for single eval.
// returns the eval errors and an array of errors if any.
func EvalString(what string) (res string, errs []string) {
	defer func() {
		if r := recover(); r != nil {
			errs = append(errs, fmt.Sprintf("panic: %v", r))
		}
	}()
	s := eval.NewState()
	macroState := eval.NewState()
	out := &strings.Builder{}
	s.Out = out
	s.NoLog = true
	_, errs = EvalOne(s, macroState, what, out, Options{All: true, ShowEval: true, NoColor: true})
	res = out.String()
	return
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
		// errors are already logged and this is the only case that can get contNeeded (EOL instead of EOF mode)
		contNeeded, _ := EvalOne(s, macroState, l, out, options)
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
// TODO: this one size fits 3 different calls (file, interactive, bot) is getting spaghetti.
func EvalOne(s, macroState *eval.State, what string, out io.Writer, options Options) (bool, []string) {
	var l *lexer.Lexer
	if options.All {
		l = lexer.New(what)
	} else {
		l = lexer.NewLineMode(what)
	}
	p := parser.New(l)
	program := p.ParseProgram()
	if logParserErrors(p) {
		return false, p.Errors()
	}
	if p.ContinuationNeeded() {
		return true, nil
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
		return false, nil
	}
	var errs []string
	if obj.Type() == object.ERROR {
		errs = append(errs, obj.Inspect())
	}
	if !options.NoColor {
		if obj.Type() == object.ERROR {
			fmt.Fprint(out, log.Colors.Red)
		} else {
			fmt.Fprint(out, log.Colors.Green)
		}
	}
	fmt.Fprintln(out, obj.Inspect())
	if !options.NoColor {
		fmt.Fprint(out, log.Colors.Reset)
	}
	return false, errs
}

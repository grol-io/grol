package repl

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"fortio.org/log"
	"grol.io/grol/ast"
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
	ShowParse  bool
	ShowEval   bool
	All        bool
	NoColor    bool // color controlled by log package, unless this is set to true.
	FormatOnly bool
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
// also returns the normalized/reformatted input if no parsing errors
// occurred.
func EvalString(what string) (res string, errs []string, formatted string) {
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
	_, errs, formatted = EvalOne(s, macroState, what, out, Options{All: true, ShowEval: true, NoColor: true})
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
		contNeeded, _, _ := EvalOne(s, macroState, l, out, options)
		if contNeeded {
			prev = l + "\n"
			prompt = CONTINUATION
		} else {
			prev = ""
			prompt = PROMPT
		}
	}
}

// Alternate API for benchmarking and simplicity.
type Grol struct {
	State     *eval.State
	Macros    *eval.State
	PrintEval bool
	program   *ast.Statements
}

// Initialize with new empty state.
func New() *Grol {
	g := &Grol{State: eval.NewState(), Macros: eval.NewState()}
	return g
}

func (g *Grol) Parse(inp []byte) error {
	l := lexer.NewBytes(inp)
	p := parser.New(l)
	g.program = p.ParseProgram()
	if len(p.Errors()) > 0 {
		return fmt.Errorf("parse errors: %v", p.Errors())
	}
	if g.Macros == nil {
		return nil
	}
	g.Macros.DefineMacros(g.program)
	numMacros := g.Macros.Len()
	if numMacros == 0 {
		return nil
	}
	log.LogVf("Expanding, %d macros defined", numMacros)
	// This actually modifies the original program, not sure... that's good but that's why
	// expanded return value doesn't need to be used.
	_ = g.Macros.ExpandMacros(g.program)
	return nil
}

func (g *Grol) Run(out io.Writer) error {
	g.State.Out = out
	res := g.State.Eval(g.program)
	if res.Type() == object.ERROR {
		return fmt.Errorf("eval error: %v", res.Inspect())
	}
	if g.PrintEval {
		fmt.Fprint(out, res.Inspect())
	}
	return nil
}

// Returns true in line mode if more should be fed to the parser.
// TODO: this one size fits 3 different calls (file, interactive, bot) is getting spaghetti.
func EvalOne(s, macroState *eval.State, what string, out io.Writer, options Options) (bool, []string, string) {
	var l *lexer.Lexer
	if options.All {
		l = lexer.New(what)
	} else {
		l = lexer.NewLineMode(what)
	}
	p := parser.New(l)
	program := p.ParseProgram()
	if logParserErrors(p) {
		return false, p.Errors(), what
	}
	if p.ContinuationNeeded() {
		return true, nil, what
	}
	formatted := program.PrettyPrint(ast.NewPrintState()).String()
	if options.FormatOnly {
		_, _ = out.Write([]byte(formatted))
		return false, nil, formatted
	}
	if options.ShowParse {
		fmt.Fprint(out, "== Parse ==> ")
		program.PrettyPrint(&ast.PrintState{Out: out})
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
			program.PrettyPrint(&ast.PrintState{Out: out})
		}
	} else {
		log.LogVf("Skipping macro expansion as none are defined")
	}
	if options.ShowParse && options.ShowEval {
		fmt.Fprint(out, "== Eval  ==> ")
	}
	obj := s.Eval(program)
	if !options.ShowEval {
		return false, nil, formatted
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
	return false, errs, formatted
}

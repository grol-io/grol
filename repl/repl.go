package repl

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"fortio.org/log"
	"fortio.org/terminal"
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
	ShowParse   bool
	ShowEval    bool
	All         bool
	NoColor     bool // color controlled by log package, unless this is set to true.
	FormatOnly  bool
	Compact     bool
	NilAndErr   bool // Show nil and errors in normal output.
	HistoryFile string
}

func EvalAll(s *eval.State, macroState *object.Environment, in io.Reader, out io.Writer, options Options) []string {
	b, err := io.ReadAll(in)
	if err != nil {
		log.Fatalf("%v", err)
	}
	what := string(b)
	_, errs, _ := EvalOne(s, macroState, what, out, options)
	return errs
}

// Kinda ugly (global) but helpful to not change the signature of EvalString for now and
// yet allow the caller to set this (ie. the discord bot).
var CompactEvalString bool

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
	macroState := object.NewMacroEnvironment()
	out := &strings.Builder{}
	s.Out = out
	s.LogOut = out
	s.NoLog = true
	_, errs, formatted = EvalOne(s, macroState, what, out,
		Options{All: true, ShowEval: true, NoColor: true, Compact: CompactEvalString})
	res = out.String()
	return
}

func Interactive(options Options) int {
	options.NilAndErr = true
	s := eval.NewState()
	macroState := object.NewMacroEnvironment()

	prev := ""

	term, err := terminal.Open()
	if err != nil {
		return log.FErrf("Error creating readline: %v", err)
	}
	defer term.Close()
	term.LoggerSetup()
	term.SetPrompt(PROMPT)
	options.Compact = true // because terminal doesn't do well will multi-line commands.
	_ = term.SetHistoryFile(options.HistoryFile)
	for {
		rd, err := term.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// To avoid trailing prompt due to prompt refresh on the log output
				// that's a bit ugly that it's necessary. consider handling in terminal.Close
				term.SetPrompt("")
				log.Infof("Exit requested") // Don't say EOF as ^C comes through as EOF as well.
				return 0
			}
			return log.FErrf("Error reading line: %v", err)
		}
		log.Debugf("Read: %q", rd)
		l := prev + rd
		// errors are already logged and this is the only case that can get contNeeded (EOL instead of EOF mode)
		contNeeded, _, formatted := EvalOne(s, macroState, l, term.Out, options)
		if contNeeded {
			prev = l + "\n"
			term.SetPrompt(CONTINUATION)
		} else {
			if prev != "" {
				what := strings.TrimSpace(formatted)
				log.LogVf("Adding to history: %q", what)
				term.AddToHistory(what)
			}
			prev = ""
			term.SetPrompt(PROMPT)
		}
	}
}

// Alternate API for benchmarking and simplicity.
type Grol struct {
	State     *eval.State
	Macros    *object.Environment
	PrintEval bool
	program   *ast.Statements
}

// Initialize with new empty state.
func New() *Grol {
	g := &Grol{State: eval.NewState(), Macros: object.NewMacroEnvironment()}
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
	eval.DefineMacros(g.Macros, g.program)
	numMacros := g.Macros.Len()
	if numMacros == 0 {
		return nil
	}
	log.LogVf("Expanding, %d macros defined", numMacros)
	// This actually modifies the original program, not sure... that's good but that's why
	// expanded return value doesn't need to be used.
	_ = eval.ExpandMacros(g.Macros, g.program)
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
func EvalOne(s *eval.State, macroState *object.Environment, what string, out io.Writer, options Options) (bool, []string, string) {
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
	printer := ast.NewPrintState()
	printer.Compact = options.Compact
	formatted := program.PrettyPrint(printer).String()
	if options.FormatOnly {
		_, _ = out.Write([]byte(formatted))
		return false, nil, formatted
	}
	if options.ShowParse {
		fmt.Fprint(out, "== Parse ==> ", formatted)
		if options.Compact {
			fmt.Fprintln(out)
		}
	}
	eval.DefineMacros(macroState, program)
	numMacros := macroState.Len()
	if numMacros > 0 {
		log.LogVf("Expanding, %d macros defined", numMacros)
		// This actually modifies the original program, not sure... that's good but that's why
		// expanded return value doesn't need to be used.
		_ = eval.ExpandMacros(macroState, program)
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
	if !options.NilAndErr && (obj.Type() == object.NIL || obj.Type() == object.ERROR) {
		return false, errs, formatted
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

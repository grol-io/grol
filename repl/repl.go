package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal"
	"grol.io/grol/ast"
	"grol.io/grol/eval"
	"grol.io/grol/extensions"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
	"grol.io/grol/token"
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

// AutoSaveFile is the filename used to autoload/save to.
// It matches extension's LoadSaveEmptyOnly file.
const AutoSaveFile = extensions.GrolFileExtension

type Options struct {
	ShowParse   bool
	ShowEval    bool
	All         bool
	NoColor     bool // color controlled by log package, unless this is set to true.
	FormatOnly  bool
	Compact     bool
	NilAndErr   bool // Show nil and errors in normal output.
	HistoryFile string
	MaxHistory  int
	AutoLoad    bool
	AutoSave    bool
	MaxDepth    int
}

func AutoLoad(s *eval.State, options Options) error {
	if !options.AutoLoad {
		log.Debugf("Autoload disabled")
		return nil
	}
	f, err := os.Open(AutoSaveFile)
	if errors.Is(err, os.ErrNotExist) {
		log.Infof("No autoload file %s", AutoSaveFile)
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	// Read line by line because some stuff don't serialize well (eg +Inf https://github.com/grol-io/grol/issues/138)
	// and yet we should try to get back as much as possible instead of aborting.
	scanner := bufio.NewScanner(f)
	count := 0
	errorCount := 0
	var errs []error
	for scanner.Scan() {
		line := scanner.Text()
		_, err = eval.EvalString(s, line, false)
		if err == nil {
			count++
		} else {
			errorCount++
			errs = append(errs, err)
			log.Errf("Error loading autoload line %q: %v", line, err)
		}
	}
	_, numset := s.UpdateNumSet()
	log.Infof("Auto loaded %s (%d set) %d lines, %d %s",
		AutoSaveFile,
		numset,
		count,
		errorCount, cli.Plural(errorCount, "error"))
	return errors.Join(errs...)
}

func AutoSave(s *eval.State, options Options) error {
	if !options.AutoSave {
		log.Debugf("Autosave disabled")
		return nil
	}
	oldS, newS := s.UpdateNumSet()
	updates := newS - oldS
	if updates == 0 {
		log.Infof("Nothing changed, not auto saving")
		return nil
	}
	f, err := os.CreateTemp(".", ".grol*.tmp")
	if err != nil {
		return err
	}
	// Write to temp file.
	n, err := s.SaveGlobals(f)
	if err != nil {
		return err
	}
	// Rename "atomically" (not really but close enough).
	err = os.Rename(f.Name(), AutoSaveFile)
	if err != nil {
		return err
	}
	log.Infof("Auto saved %d ids/fns (%d set) to: %s", n, updates, AutoSaveFile)
	return nil
}

func EvalAll(s *eval.State, in io.Reader, out io.Writer, options Options) []string {
	b, err := io.ReadAll(in)
	if err != nil {
		log.Fatalf("%v", err)
	}
	what := string(b)
	_, _, errs, _ := EvalOne(s, what, out, options) //nolint:dogsled // as mentioned we should refactor EvalOne.
	return errs
}

// EvalString can be used from playground etc for single eval.
// returns the eval errors and an array of errors if any.
// also returns the normalized/reformatted input if no parsing errors
// occurred.
// Default options are Options{All: true, ShowEval: true, NoColor: true, Compact: CompactEvalString}.
func EvalString(what string) (string, []string, string) {
	return EvalStringWithOption(Options{All: true, ShowEval: true, NoColor: true, Compact: false}, what)
}

// EvalStringWithOption can be used from playground etc for single eval.
// returns the eval errors and an array of errors if any.
// also returns the normalized/reformatted input if no parsing errors
// occurred.
// Following options should be set (like in EvalString) to control the behavior:
//
//	All: true. ShowEval: true, NoColor: true.
//
// Options to set AutoLoad and AutoSave and Compact.
func EvalStringWithOption(o Options, what string) (res string, errs []string, formatted string) {
	s := eval.NewState()
	s.MaxDepth = o.MaxDepth
	out := &strings.Builder{}
	s.Out = out
	s.LogOut = out
	s.NoLog = true
	_ = AutoLoad(s, o) // errors already logged
	panicked := false
	_, panicked, errs, formatted = EvalOne(s, what, out, o)
	res = out.String()
	if !panicked {
		_ = AutoSave(s, o)
	}
	return
}

func Interactive(options Options) int {
	options.NilAndErr = true
	s := eval.NewState()
	s.MaxDepth = options.MaxDepth
	term, err := terminal.Open()
	if err != nil {
		return log.FErrf("Error creating readline: %v", err)
	}
	defer term.Close()
	s.Out = term.Out
	autoComplete := NewCompletion()
	tokInfo := token.Info()
	for v := range tokInfo.Keywords {
		autoComplete.Trie.Insert(v + " ")
	}
	for v := range tokInfo.Builtins {
		autoComplete.Trie.Insert(v + "(")
	}
	for k := range object.ExtraFunctions() {
		autoComplete.Trie.Insert(k + "(")
	}
	// Initial ids and functions.
	s.RegisterTrie(autoComplete.Trie)
	term.SetAutoCompleteCallback(autoComplete.AutoComplete())
	term.SetPrompt(PROMPT)
	options.Compact = true // because terminal doesn't (yet) do well will multi-line commands.
	term.NewHistory(options.MaxHistory)
	_ = term.SetHistoryFile(options.HistoryFile)
	_ = AutoLoad(s, options) // errors already logged
	// Regular expression for "!nn" to run history command nn.
	historyRegex := regexp.MustCompile(`^!(\d+)$`)
	prev := ""
	for {
		rd, err := term.ReadLine()
		if errors.Is(err, io.EOF) {
			log.Infof("Exit requested") // Don't say EOF as ^C comes through as EOF as well.
			_ = AutoSave(s, options)
			return 0
		}
		if err != nil {
			return log.FErrf("Error reading line: %v", err)
		}
		log.Debugf("Read: %q", rd)
		l := prev + rd
		if historyRegex.MatchString(l) {
			h := term.History()
			slices.Reverse(h)
			idxStr := l[1:]
			idx, _ := strconv.Atoi(idxStr)
			if idx < 1 || idx > len(h) {
				log.Errf("Invalid history index %d", idx)
				continue
			}
			l = h[idx-1]
			fmt.Fprintf(term.Out, "Repeating history %d: %s\n", idx, l)
			term.ReplaceLatest(l)
		}
		switch {
		case l == "history":
			h := term.History()
			slices.Reverse(h)
			for i, v := range h {
				fmt.Fprintf(term.Out, "%02d: %s\n", i+1, v)
			}
			continue
		case l == "help":
			fmt.Fprintln(term.Out,
				"Type 'history' to see history, '!n' to repeat history n, 'info' for language builtins, use <tab> for completion.")
			continue
		}
		// normal errors are already logged but not the panic recoveries
		// Note this is the only case that can get contNeeded (EOL instead of EOF mode)
		contNeeded, panicked, errs, formatted := EvalOne(s, l, term.Out, options)
		if contNeeded {
			prev = l + "\n"
			term.SetPrompt(CONTINUATION)
		} else {
			if prev != "" && len(formatted) > 0 {
				// In addition to raw lines, we also add the single line version to history.
				log.LogVf("Adding to history: %q", formatted)
				term.AddToHistory(formatted)
			}
			prev = ""
			term.SetPrompt(PROMPT)
		}
		if panicked {
			log.Errf("%s", errs[0]) // we know there is exactly 1 error in this case, see EvalOne defer.
		}
	}
}

// Alternate API for benchmarking and simplicity.
type Grol struct {
	State     *eval.State
	PrintEval bool
	program   *ast.Statements
}

// Initialize with new empty state.
func New() *Grol {
	g := &Grol{State: eval.NewState()}
	return g
}

func (g *Grol) Parse(inp []byte) error {
	l := lexer.NewBytes(inp)
	p := parser.New(l)
	g.program = p.ParseProgram()
	if len(p.Errors()) > 0 {
		return fmt.Errorf("parse errors: %v", p.Errors())
	}
	g.State.DefineMacros(g.program)
	numMacros := g.State.NumMacros()
	if numMacros == 0 {
		return nil
	}
	log.LogVf("Expanding, %d macros defined", numMacros)
	// This actually modifies the original program, not sure... that's good but that's why
	// expanded return value doesn't need to be used.
	_ = g.State.ExpandMacros(g.program)
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

// EvalOne returns continuation=true in line mode if more should be fed to the parser.
// errs is the list of errors, formatted is the normalized input.
// If a panic occurs, panicked is true and errs contains the one panic message.
// TODO: this one size fits 3 different calls (file, interactive, bot) is getting spaghetti.
func EvalOne(s *eval.State, what string, out io.Writer, options Options) (
	continuation, panicked bool,
	errs []string,
	formatted string,
) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			log.Critf("Caught panic: %v", r)
			if log.LogDebug() {
				log.Debugf("Dumping stack trace")
				debug.PrintStack()
			}
			errs = append(errs, fmt.Sprintf("panic: %v", r))
			return
		}
	}()
	continuation, errs, formatted = evalOne(s, what, out, options)
	return
}

func evalOne(s *eval.State, what string, out io.Writer, options Options) (bool, []string, string) {
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
	s.DefineMacros(program)
	numMacros := s.NumMacros()
	if numMacros > 0 {
		log.LogVf("Expanding, %d macros defined", numMacros)
		// This actually modifies the original program, not sure... that's good but that's why
		// expanded return value doesn't need to be used.
		_ = s.ExpandMacros(program)
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

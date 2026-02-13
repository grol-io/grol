//go:build wasm

/*
Web assembly main for grol.

Existing API (textarea/batch mode):
  - grol(input, compact) → {result, errors, formatted, image}

New xterm.js interactive mode:
  - grolStartREPL(cols, rows) → starts an interactive REPL goroutine
  - grolSetTermSize(cols, rows) → updates terminal dimensions
  - Requires JS-side stdin/stdout bridge via globalThis.fs overrides
  - Uses golang.org/x/term.Terminal for line editing (same as fortio.org/terminal internally)

NOTE: Once fortio.org/terminal adds WASM support (MakeRaw/IsTerminal stubs),
this can be simplified to just call repl.Interactive() directly.
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal"
	"fortio.org/version"
	"golang.org/x/term"
	"grol.io/grol/eval"
	"grol.io/grol/extensions"
	"grol.io/grol/object"
	"grol.io/grol/repl"
	"grol.io/grol/token"
	"grol.io/grol/trie"
)

var (
	// Can do 10k on safari but only ~3.5k on chrome before
	// Error: Maximum call stack size exceeded.
	// That means n = 3096 on pi2.gr, off by 4 for some reason
	WasmMaxDepth = 3_100

	// Set a reasonably low memory limit for wasm. 512MiB.
	WasmMemLimit = int64(512 * 1024 * 1024)

	// Low limit for page to not appear dead for too long
	WasmMaxDuration = 3 * time.Second
)

func jsEval(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 && len(args) != 2 {
		return "ERROR: number of arguments doesn't match should be string or string, bool for compact mode"
	}
	input := args[0].String()
	compact := false
	if len(args) == 2 {
		compact = args[1].Bool()
	}
	opts := repl.EvalStringOptions()
	opts.Compact = compact
	// For tinygo until recover is implemented, we would set a large value for MaxDepth to get
	// Error: Maximum call stack size exceeded.
	// instead of failing to handle our panic (!)
	// https://tinygo.org/docs/reference/lang-support/#recover-builtin
	// But enough is enough... switched back to big go for now, way too many troubles with tinygo as well
	// as not exactly responsive to PRs nor issues folks (everyone trying their best yet...).
	opts.MaxDepth = WasmMaxDepth
	opts.MaxDuration = WasmMaxDuration
	res, errs, formatted := repl.EvalStringWithOption(context.Background(), opts, input)
	result := make(map[string]any)
	if strings.HasPrefix(res, "data:") {
		// special case for data: urls, we don't want to return the data
		result["image"] = res
		res = ""
	}
	result["result"] = strings.TrimSuffix(res, "\n")
	// transfer errors to []any (!)
	anyErrs := make([]any, len(errs))
	for i, v := range errs {
		anyErrs[i] = v
	}
	result["errors"] = anyErrs
	fmted := strings.TrimSuffix(formatted, "\n")
	if fmted == "" {
		fmted = input
	}
	result["formatted"] = fmted
	return result
}

var (
	TinyGoVersion string
	grolVer       string // package-level version string for REPL welcome message
)

// --- xterm.js Interactive REPL via x/term.Terminal ---

// xtermTerminal is the x/term.Terminal used for line editing.
// It's package-level so jsSetTermSize can update it.
var (
	xtermTerminal *term.Terminal
	xtermMu       sync.Mutex
)

// jsStartREPL starts the interactive REPL loop in a goroutine.
// Called from JS after the stdin/stdout bridge is set up.
// Args: cols (int), rows (int).
func jsStartREPL(_ js.Value, args []js.Value) interface{} {
	cols, rows := 80, 24
	if len(args) >= 2 {
		cols = args[0].Int()
		rows = args[1].Int()
	}
	go wasmInteractive(cols, rows)
	return nil
}

// jsSetTermSize updates the terminal dimensions (called on resize).
func jsSetTermSize(_ js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return nil
	}
	cols := args[0].Int()
	rows := args[1].Int()
	xtermMu.Lock()
	defer xtermMu.Unlock()
	if xtermTerminal != nil {
		_ = xtermTerminal.SetSize(cols, rows)
	}
	return nil
}

// wasmInteractive runs an interactive REPL loop similar to repl.Interactive()
// but using x/term.Terminal directly for line editing (works in WASM because
// x/term.Terminal is platform-independent — it just needs an io.ReadWriter
// and produces/consumes ANSI escape sequences, which xterm.js handles perfectly).
//
// This replicates the core logic of repl.Interactive() including:
// - Line editing via ANSI escape sequences (handled by x/term.Terminal)
// - Tab completion
// - Command history
// - Multi-line continuation for incomplete input
// - Special commands: history, help, exit, !n
func wasmInteractive(cols, rows int) { //nolint:funlen,gocognit // mirrors repl.Interactive complexity
	// Set up eval state
	s := eval.NewState()
	s.MaxDepth = WasmMaxDepth

	options := repl.Options{
		ShowEval:    true,
		NilAndErr:   true,
		DualFormat:  true,
		MaxDuration: WasmMaxDuration,
	}

	// Set up autocomplete trie
	autoTrie := trie.NewTrie()
	tokInfo := token.Info()
	for v := range tokInfo.Keywords {
		autoTrie.Insert(v + " ")
	}
	for v := range tokInfo.Builtins {
		autoTrie.Insert(v + "(")
	}
	for k := range object.ExtraFunctions() {
		autoTrie.Insert(k + "(")
	}
	autoTrie.Insert("history")
	s.RegisterTrie(autoTrie)

	// Create the x/term.Terminal with stdin/stdout as the ReadWriter.
	// In WASM, these are bridged to xterm.js via globalThis.fs overrides.
	rw := struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stdout}
	t := term.NewTerminal(rw, repl.PROMPT)
	_ = t.SetSize(cols, rows)

	// Store for resize updates
	xtermMu.Lock()
	xtermTerminal = t
	xtermMu.Unlock()

	// Set up history (using fortio.org/terminal.TermHistory which implements term.History)
	history := terminal.NewHistory(terminal.DefaultHistoryCapacity)
	history.AutoHistory = false // We manage history manually like repl.Interactive does
	t.History = history

	// Set up autocomplete callback
	t.AutoCompleteCallback = func(line string, pos int, key rune) (string, int, bool) {
		if key != '\t' {
			return line, pos, false
		}
		prefix := line[:pos]
		l, commands := autoTrie.PrefixAll(prefix)
		if len(commands) == 0 {
			return line, pos, false
		}
		if len(commands) > 1 {
			fmt.Fprint(t, "\nOne of: ")
			for _, c := range commands {
				if strings.HasSuffix(c, "(") {
					fmt.Fprint(t, c, ") ")
				} else {
					fmt.Fprint(t, c)
				}
			}
			fmt.Fprintln(t)
		}
		return commands[0][:l], l, true
	}

	// Direct all eval output through the terminal (handles CRLF conversion)
	s.Out = t
	s.LogOut = t

	// Force color mode on: xterm.js supports ANSI colors but fortio.org/log's
	// auto-detection fails in WASM because IsTerminal() returns false.
	log.Config.ForceColor = true
	log.SetColorMode()

	// Set interactive=true
	_, _ = eval.EvalString(s, "interactive=true", false)
	_, _ = s.UpdateNumSet()

	// Welcome message
	fmt.Fprintf(t, "GROL %s - type 'help' for help, 'info' for builtins, <tab> for completion\n", grolVer)

	// Main REPL loop (mirrors repl.Interactive)
	prev := ""
	for {
		rd, err := t.ReadLine()
		if errors.Is(err, io.EOF) {
			log.Infof("EOF, exiting REPL")
			return
		}
		if errors.Is(err, term.ErrPasteIndicator) {
			// Paste mode, treat as normal input
			err = nil
		}
		if err != nil {
			log.Warnf("Error reading line: %v", err)
			// On interrupt/error, reset continuation
			if prev != "" {
				prev = ""
				t.SetPrompt(repl.PROMPT)
			}
			continue
		}

		log.Debugf("Read: %q", rd)

		// Handle !n history recall
		if idx, ok := extractHistoryNumber(rd); ok {
			h := getHistory(t.History)
			slices.Reverse(h)
			if idx < 1 || idx > len(h) {
				fmt.Fprintf(t, "Invalid history index %d\n", idx)
				continue
			}
			rd = h[idx-1]
			fmt.Fprintf(t, "Repeating history %d: %s\n", idx, rd)
		}
		history.UnconditionalAdd(rd)

		l := prev + rd

		// Handle special commands (only when not in continuation)
		if prev == "" {
			switch l {
			case "history":
				h := getHistory(t.History)
				slices.Reverse(h)
				for i, v := range h {
					fmt.Fprintf(t, "%02d: %s\n", i+1, v)
				}
				continue
			case "help":
				fmt.Fprintln(t,
					"Type 'history' to see history, '!n' to repeat history n,"+
						" 'info' for language builtins, use <tab> for completion.")
				continue
			case "exit":
				log.Infof("Exit requested")
				fmt.Fprintln(t, "Goodbye!")
				return
			case "clear":
				// Send ANSI clear screen sequence
				fmt.Fprint(t, "\033[2J\033[H")
				continue
			}
		}

		// Evaluate
		ctx := context.Background()
		contNeeded, _, errs, formatted := repl.EvalOne(ctx, s, l, t, options)
		if contNeeded {
			prev = l + "\n"
			t.SetPrompt(repl.CONTINUATION)
		} else {
			if prev != "" && len(formatted) > 0 {
				// Also add the single-line formatted version to history
				history.UnconditionalAdd(formatted)
			}
			prev = ""
			t.SetPrompt(repl.PROMPT)
		}
		_ = errs // errors already printed by EvalOne
		// Update autocomplete with any new identifiers
		s.RegisterTrie(autoTrie)
	}
}

// extractHistoryNumber extracts the history number from "!n" input.
func extractHistoryNumber(input string) (int, bool) {
	if len(input) > 1 && input[0] == '!' {
		if num, err := strconv.Atoi(input[1:]); err == nil {
			return num, true
		}
	}
	return 0, false
}

// getHistory returns all history entries as a slice.
func getHistory(h term.History) []string {
	res := make([]string, 0, h.Len())
	for i := range h.Len() {
		res = append(res, h.At(i))
	}
	return res
}

func main() {
	cli.Main() // just to get version etc
	_, grolVersion, _ := version.FromBuildInfoPath("grol.io/grol")
	if TinyGoVersion != "" { // tinygo doesn't have modules info in buildinfo nor tinygo install...
		grolVersion = TinyGoVersion + " " + runtime.Compiler + runtime.Version() + " " + runtime.GOARCH + " " + runtime.GOOS
		cli.LongVersion = grolVersion
		cli.ShortVersion = TinyGoVersion
	}
	grolVer = grolVersion // store for REPL init
	prev := debug.SetMemoryLimit(WasmMemLimit)
	log.Infof("Grol wasm main %s - prev memory limit %d now %d", grolVersion, prev, WasmMemLimit)
	global := js.Global()
	// Existing batch eval API (textarea mode)
	global.Set("grol", js.FuncOf(jsEval))
	global.Set("grolVersion", js.ValueOf(grolVersion))
	// Interactive REPL API (xterm.js mode)
	global.Set("grolStartREPL", js.FuncOf(jsStartREPL))
	global.Set("grolSetTermSize", js.FuncOf(jsSetTermSize))
	// IOs don't work yet https://github.com/grol-io/grol/issues/124 otherwise we'd
	// use extensions.Config and allow HasLoad HasSave.
	err := extensions.Init(nil)
	if err != nil {
		log.Critf("Error initializing extensions: %v", err)
	}
	select {}
}

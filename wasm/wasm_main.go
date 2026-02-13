//go:build wasm

/*
Web assembly main for grol.

Existing API (textarea/batch mode):
  - grol(input, compact) → {result, errors, formatted, image}

xterm.js interactive mode:
  - grolStartREPL(cols, rows) → starts an interactive REPL goroutine
  - grolSetTermSize(cols, rows) → updates terminal dimensions
  - Requires JS-side stdin/stdout bridge via globalThis.fs overrides
    (see xterm.html for the bridge implementation)
  - JS side must set globalThis.TerminalConnected = true before go.run()
    for fortio.org/terminal to detect the terminal emulator
*/

package main

import (
	"context"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal"
	"fortio.org/version"
	"grol.io/grol/eval"
	"grol.io/grol/extensions"
	"grol.io/grol/repl"
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
)

// --- xterm.js Interactive REPL ---

// wasmTerm holds the fortio.org/terminal.Terminal for resize updates.
var (
	wasmTerm *terminal.Terminal
	wasmMu   sync.Mutex
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
	// Set initial size globals for fortio.org/terminal's platformGetSize
	global := js.Global()
	global.Set("TerminalCols", cols)
	global.Set("TerminalRows", rows)
	go func() {
		options := repl.Options{
			ShowEval:    true,
			MaxDepth:    WasmMaxDepth,
			MaxDuration: WasmMaxDuration,
		}
		// Capture the terminal object via PreInput so we can update size on resize
		options.PreInput = func(s *eval.State) {
			wasmMu.Lock()
			wasmTerm = s.Term
			wasmMu.Unlock()
		}
		repl.Interactive(options)
	}()
	return nil
}

// jsSetTermSize updates the terminal dimensions (called on resize).
func jsSetTermSize(_ js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return nil
	}
	cols := args[0].Int()
	rows := args[1].Int()
	// Update JS globals so platformGetSize returns the new values
	global := js.Global()
	global.Set("TerminalCols", cols)
	global.Set("TerminalRows", rows)
	// Tell the fortio.org/terminal.Terminal to re-read the size
	wasmMu.Lock()
	t := wasmTerm
	wasmMu.Unlock()
	if t != nil {
		_ = t.UpdateSize()
	}
	return nil
}

func main() {
	cli.Main() // just to get version etc
	_, grolVersion, _ := version.FromBuildInfoPath("grol.io/grol")
	if TinyGoVersion != "" { // tinygo doesn't have modules info in buildinfo nor tinygo install...
		grolVersion = TinyGoVersion + " " + runtime.Compiler + runtime.Version() + " " + runtime.GOARCH + " " + runtime.GOOS
		cli.LongVersion = grolVersion
		cli.ShortVersion = TinyGoVersion
	}
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

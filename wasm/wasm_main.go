//go:build wasm
// +build wasm

/*
Web assembly main for grol, exposing grol (repl.EvalString for now) to JS
*/

package main

import (
	"runtime"
	"strings"
	"syscall/js"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/version"
	"grol.io/grol/extensions"
	"grol.io/grol/repl"
)

// Can do 10k on safari but only ~3.5k on chrome before
// Error: Maximum call stack size exceeded.
// That means n = 3096 on pi2.gr, off by 4 for some reason
var WasmMaxDepth = 3_100

func jsEval(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 && len(args) != 2 {
		return "ERROR: number of arguments doesn't match should be string or string, bool for compact mode"
	}
	input := args[0].String()
	compact := false
	if len(args) == 2 {
		compact = args[1].Bool()
	}
	res, errs, formatted := repl.EvalStringWithOption(
		// For tinygo until recover is implemented, we would set a large value for MaxDepth to get
		// Error: Maximum call stack size exceeded.
		// instead of failing to handle our panic (!)
		// https://tinygo.org/docs/reference/lang-support/#recover-builtin
		// But enough is enough... switched back to big go for now, way too many troubles with tinygo as well
		// as not exactly responsive to PRs nor issues folks (everyone trying their best yet...).
		repl.Options{All: true, ShowEval: true, NoColor: true, Compact: compact, MaxDepth: WasmMaxDepth},
		input,
	)
	result := make(map[string]any)
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

var TinyGoVersion string

func main() {
	cli.Main() // just to get version etc
	_, grolVersion, _ := version.FromBuildInfoPath("grol.io/grol")
	if TinyGoVersion != "" { // tinygo doesn't have modules info in buildinfo nor tinygo install...
		grolVersion = TinyGoVersion + " " + runtime.Compiler + runtime.Version() + " " + runtime.GOARCH + " " + runtime.GOOS
		cli.LongVersion = grolVersion
		cli.ShortVersion = TinyGoVersion
	}
	log.Infof("Grol wasm main %s", grolVersion)
	done := make(chan struct{}, 0)
	global := js.Global()
	global.Set("grol", js.FuncOf(jsEval))
	global.Set("grolVersion", js.ValueOf(grolVersion))
	// IOs don't work yet https://github.com/grol-io/grol/issues/124 otherwise we'd
	// use extensions.Config and allow HasLoad HasSave.
	err := extensions.Init(nil)
	if err != nil {
		log.Critf("Error initializing extensions: %v", err)
	}
	<-done
}

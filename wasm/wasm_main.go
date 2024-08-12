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

func jsEval(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 && len(args) != 2 {
		return "ERROR: number of arguments doesn't match should be string or string, bool for compact mode"
	}
	input := args[0].String()
	compact := false
	if len(args) == 2 {
		compact = args[1].Bool()
	}
	repl.CompactEvalString = compact
	res, errs, formatted := repl.EvalString(input)
	result := make(map[string]any)
	result["result"] = strings.TrimSuffix(res, "\n")
	// transfer errors to []any (!)
	anyErrs := make([]any, len(errs))
	for i, v := range errs {
		anyErrs[i] = v
	}
	result["errors"] = anyErrs
	result["formatted"] = strings.TrimSuffix(formatted, "\n")
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

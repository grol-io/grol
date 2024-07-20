//go:build wasm
// +build wasm

/*
Web assembly main for grol, exposing grol (repl.EvalString for now) to JS
*/

package main

import (
	"runtime"
	"syscall/js"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/version"
	"grol.io/grol/repl"
)

func jsEval(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 {
		return "ERROR: number of arguments doesn't match"
	}
	input := args[0].String()
	res, errs := repl.EvalString(input)
	result := make(map[string]any)
	result["result"] = res
	// transfer errors to []any (!)
	anyErrs := make([]any, len(errs))
	for i, v := range errs {
		anyErrs[i] = v
	}
	result["errors"] = anyErrs
	return result
}

var TinyGoVersion string

func main() {
	cli.Main() // just to get version etc
	_, grolVersion, _ := version.FromBuildInfoPath("grol.io/grol")
	if TinyGoVersion != "" { // tinygo doesn't have modules info in buildinfo nor tinygo install...
		grolVersion = TinyGoVersion + " " + runtime.Compiler + runtime.Version() + " " + runtime.GOARCH + " " + runtime.GOOS
	}
	log.Infof("Grol wasm main %s", grolVersion)
	done := make(chan struct{}, 0)
	global := js.Global()
	global.Set("grol", js.FuncOf(jsEval))
	global.Set("grolVersion", js.ValueOf(grolVersion))
	<-done
}

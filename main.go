// Gorepl is a simple interpreted language with a syntax similar to Go.
package main

import (
	"flag"
	"fmt"
	"os"

	"fortio.org/cli"
	"fortio.org/log"
	"grol.io/grol/eval"
	"grol.io/grol/repl"
)

func main() {
	os.Exit(Main())
}

func Main() int {
	commandFlag := flag.String("c", "", "command/inline script to run instead of interactive mode")
	showParse := flag.Bool("parse", false, "show parse tree")
	format := flag.Bool("format", false, "don't execute, just parse and re format the input")
	showEval := flag.Bool("eval", true, "show eval results")
	sharedState := flag.Bool("shared-state", false, "All files share same interpreter state (default is new state for each)")
	cli.ArgsHelp = "*.gr files to interpret or `-` for stdin without prompt or no arguments for stdin repl..."
	cli.MaxArgs = -1
	cli.Main()
	log.Printf("grol %s - welcome!", cli.LongVersion)
	options := repl.Options{
		ShowParse:  *showParse,
		ShowEval:   *showEval,
		FormatOnly: *format,
	}
	nArgs := len(flag.Args())
	if *commandFlag != "" {
		res, errs, _ := repl.EvalString(*commandFlag)
		if len(errs) > 0 {
			log.Errf("Errors: %v", errs)
		}
		fmt.Println(res)
		return len(errs)
	}
	if nArgs == 0 {
		repl.Interactive(os.Stdin, os.Stdout, options)
		return 0
	}
	options.All = true
	s := eval.NewState()
	macroState := eval.NewState()
	for _, file := range flag.Args() {
		processOneFile(file, s, macroState, options)
		if !*sharedState {
			s = eval.NewState()
			macroState = eval.NewState()
		}
	}
	log.Infof("All done")
	return 0
}

func processOneFile(file string, s, macroState *eval.State, options repl.Options) {
	if file == "-" {
		log.Infof("Running on stdin")
		repl.EvalAll(s, macroState, os.Stdin, os.Stdout, options)
		return
	}
	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Infof("Running %s", file)
	repl.EvalAll(s, macroState, f, os.Stdout, options)
	f.Close()
}

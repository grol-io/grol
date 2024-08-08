// Gorepl is a simple interpreted language with a syntax similar to Go.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"fortio.org/cli"
	"fortio.org/log"
	"grol.io/grol/eval"
	"grol.io/grol/extensions" // register extensions
	"grol.io/grol/object"
	"grol.io/grol/repl"
)

func main() {
	os.Exit(Main())
}

var hookBefore, hookAfter func() int

func Main() int {
	commandFlag := flag.String("c", "", "command/inline script to run instead of interactive mode")
	showParse := flag.Bool("parse", false, "show parse tree")
	format := flag.Bool("format", false, "don't execute, just parse and re format the input")
	compact := flag.Bool("compact", false, "When printing code, use no indentation and most compact form")
	showEval := flag.Bool("eval", true, "show eval results")
	sharedState := flag.Bool("shared-state", false, "All files share same interpreter state (default is new state for each)")
	homeDir, err := os.UserHomeDir()
	historyFileDefault := filepath.Join(homeDir, ".grol_history")
	if err != nil {
		log.Warnf("Couldn't get user home dir: %v", err)
		historyFileDefault = ""
	}
	historyFile := flag.String("history", historyFileDefault, "history file to use")

	cli.ArgsHelp = "*.gr files to interpret or `-` for stdin without prompt or no arguments for stdin repl..."
	cli.MaxArgs = -1
	cli.Main()
	log.Infof("grol %s - welcome!", cli.LongVersion)
	options := repl.Options{
		ShowParse:   *showParse,
		ShowEval:    *showEval,
		FormatOnly:  *format,
		Compact:     *compact,
		HistoryFile: *historyFile,
	}
	if hookBefore != nil {
		ret := hookBefore()
		if ret != 0 {
			return ret
		}
	}
	err = extensions.Init()
	if err != nil {
		return log.FErrf("Error initializing extensions: %v", err)
	}
	if *commandFlag != "" {
		res, errs, _ := repl.EvalString(*commandFlag)
		if len(errs) > 0 {
			log.Errf("Errors: %v", errs)
		}
		fmt.Print(res)
		return len(errs)
	}
	if len(flag.Args()) == 0 {
		return repl.Interactive(options)
	}
	options.All = true
	s := eval.NewState()
	macroState := object.NewMacroEnvironment()
	for _, file := range flag.Args() {
		ret := processOneFile(file, s, macroState, options)
		if ret != 0 {
			return ret
		}
		if !*sharedState {
			s = eval.NewState()
			macroState = object.NewMacroEnvironment()
		}
	}
	log.Infof("All done")
	if hookAfter != nil {
		return hookAfter()
	}
	return 0
}

func processOneStream(s *eval.State, macroState *object.Environment, in io.Reader, options repl.Options) int {
	errs := repl.EvalAll(s, macroState, in, os.Stdout, options)
	if len(errs) > 0 {
		log.Errf("Errors: %v", errs)
	}
	return len(errs)
}

func processOneFile(file string, s *eval.State, macroState *object.Environment, options repl.Options) int {
	if file == "-" {
		if options.FormatOnly {
			log.Infof("Formatting stdin")
		} else {
			log.Infof("Running on stdin")
		}
		return processOneStream(s, macroState, os.Stdin, options)
	}
	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("%v", err)
	}
	verb := "Running"
	if options.FormatOnly {
		verb = "Formatting"
	}
	log.Infof("%s %s", verb, file)
	code := processOneStream(s, macroState, f, options)
	f.Close()
	return code
}

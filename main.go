// Gorepl is a simple interpreted language with a syntax similar to Go.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

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

func Main() int {
	commandFlag := flag.String("c", "", "command/inline script to run instead of interactive mode")
	showParse := flag.Bool("parse", false, "show parse tree")
	format := flag.Bool("format", false, "don't execute, just parse and re format the input")
	compact := flag.Bool("compact", false, "When printing code, use no indentation and most compact form")
	showEval := flag.Bool("eval", true, "show eval results")
	sharedState := flag.Bool("shared-state", false, "All files share same interpreter state (default is new state for each)")
	cpuprofile := flag.String("profile-cpu", "", "write cpu profile to `file`")
	memprofile := flag.String("profile-mem", "", "write memory profile to `file`")

	cli.ArgsHelp = "*.gr files to interpret or `-` for stdin without prompt or no arguments for stdin repl..."
	cli.MaxArgs = -1
	cli.Main()
	log.Infof("grol %s - welcome!", cli.LongVersion)
	options := repl.Options{
		ShowParse:  *showParse,
		ShowEval:   *showEval,
		FormatOnly: *format,
		Compact:    *compact,
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			return log.FErrf("can't open file for cpu profile: %v", err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return log.FErrf("can't start cpu profile: %v", err)
		}
		log.Infof("Writing cpu profile to %s", *cpuprofile)
		defer pprof.StopCPUProfile()
	}
	err := extensions.Init()
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
		repl.Interactive(os.Stdin, os.Stdout, options)
		return 0
	}
	options.All = true
	s := eval.NewState()
	macroState := object.NewMacroEnvironment()
	for _, file := range flag.Args() {
		processOneFile(file, s, macroState, options)
		if !*sharedState {
			s = eval.NewState()
			macroState = object.NewMacroEnvironment()
		}
	}
	log.Infof("All done")
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			return log.FErrf("can't open file for mem profile: %v", err)
		}
		err = pprof.WriteHeapProfile(f)
		if err != nil {
			return log.FErrf("can't write mem profile: %v", err)
		}
		log.Infof("Wrote memory profile to %s", *memprofile)
		f.Close()
	}
	return 0
}

func processOneFile(file string, s *eval.State, macroState *object.Environment, options repl.Options) {
	if file == "-" {
		if options.FormatOnly {
			log.Infof("Formatting stdin")
		} else {
			log.Infof("Running on stdin")
		}
		repl.EvalAll(s, macroState, os.Stdin, os.Stdout, options)
		return
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
	repl.EvalAll(s, macroState, f, os.Stdout, options)
	f.Close()
}

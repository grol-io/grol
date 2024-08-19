// Gorepl is a simple interpreted language with a syntax similar to Go.
package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/struct2env"
	"fortio.org/terminal"
	"grol.io/grol/eval"
	"grol.io/grol/extensions" // register extensions
	"grol.io/grol/repl"
)

func main() {
	os.Exit(Main())
}

type Config struct {
	HistoryFile string
}

var config = Config{}

func EnvHelp(w io.Writer) {
	res, _ := struct2env.StructToEnvVars(config)
	str := struct2env.ToShellWithPrefix("GROL_", res, true)
	fmt.Fprintln(w, "# Grol environment variables:")
	fmt.Fprint(w, str)
}

var hookBefore, hookAfter func() int

func Main() int {
	commandFlag := flag.String("c", "", "command/inline script to run instead of interactive mode")
	showParse := flag.Bool("parse", false, "show parse tree")
	format := flag.Bool("format", false, "don't execute, just parse and re format the input")
	compact := flag.Bool("compact", false, "When printing code, use no indentation and most compact form")
	showEval := flag.Bool("eval", true, "show eval results")
	sharedState := flag.Bool("shared-state", false, "All files share same interpreter state (default is new state for each)")
	const historyDefault = "~/.grol_history" // virtual/token filename, will be replaced by actual home dir if not changed.
	cli.EnvHelpFuncs = append(cli.EnvHelpFuncs, EnvHelp)
	defaultHistoryFile := historyDefault
	errs := struct2env.SetFromEnv("GROL_", &config)
	if len(errs) > 0 {
		log.Errf("Error setting config from env: %v", errs)
	}
	if config.HistoryFile != "" {
		defaultHistoryFile = config.HistoryFile
	}
	historyFile := flag.String("history", defaultHistoryFile, "history `file` to use")
	maxHistory := flag.Int("max-history", terminal.DefaultHistoryCapacity, "max history `size`, use 0 to disable.")
	disableLoadSave := flag.Bool("no-load-save", false, "disable load/save of history")
	unrestrictedIOs := flag.Bool("unrestricted-io", false, "enable unrestricted io (dangerous)")
	emptyOnly := flag.Bool("empty-only", false, "only allow load()/save() to ./.gr")
	noAuto := flag.Bool("no-auto", false, "don't auto load/save the state to ./.gr")
	maxDepth := flag.Int("max-depth", eval.DefaultMaxDepth-1, "Maximum interpreter depth")
	maxLen := flag.Int("max-save-len", 4000, "Maximum len of saved identifiers, use 0 for unlimited")
	panicOk := flag.Bool("panic", false, "Don't catch panic - only for development/debugging")

	cli.ArgsHelp = "*.gr files to interpret or `-` for stdin without prompt or no arguments for stdin repl..."
	cli.MaxArgs = -1
	cli.Main()
	histFile := *historyFile
	if histFile == historyDefault {
		homeDir, err := os.UserHomeDir()
		histFile = filepath.Join(homeDir, ".grol_history")
		if err != nil {
			log.Warnf("Couldn't get user home dir: %v", err)
			histFile = ""
		}
	}
	log.Infof("grol %s - welcome!", cli.LongVersion)
	memlimit := debug.SetMemoryLimit(-1)
	if memlimit == math.MaxInt64 {
		log.Warnf("Memory limit not set, please set the GOMEMLIMIT env var; e.g. GOMEMLIMIT=1GiB")
	}
	options := repl.Options{
		ShowParse:   *showParse,
		ShowEval:    *showEval,
		FormatOnly:  *format,
		Compact:     *compact,
		HistoryFile: histFile,
		MaxHistory:  *maxHistory,
		AutoLoad:    !*noAuto,
		AutoSave:    !*noAuto,
		MaxDepth:    *maxDepth + 1,
		MaxValueLen: *maxLen,
		PanicOk:     *panicOk,
	}
	if hookBefore != nil {
		ret := hookBefore()
		if ret != 0 {
			return ret
		}
	}
	c := extensions.Config{
		HasLoad:           !*disableLoadSave,
		HasSave:           !*disableLoadSave,
		UnrestrictedIOs:   *unrestrictedIOs,
		LoadSaveEmptyOnly: *emptyOnly,
	}
	err := extensions.Init(&c)
	if err != nil {
		return log.FErrf("Error initializing extensions: %v", err)
	}
	if *commandFlag != "" {
		res, errs, _ := repl.EvalStringWithOption(options, *commandFlag)
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
	for _, file := range flag.Args() {
		ret := processOneFile(file, s, options)
		if ret != 0 {
			return ret
		}
		if !*sharedState {
			s = eval.NewState()
		}
	}
	log.Infof("All done")
	if hookAfter != nil {
		return hookAfter()
	}
	return 0
}

func processOneStream(s *eval.State, in io.Reader, options repl.Options) int {
	errs := repl.EvalAll(s, in, os.Stdout, options)
	if len(errs) > 0 {
		log.Errf("Errors: %v", errs)
	}
	return len(errs)
}

func processOneFile(file string, s *eval.State, options repl.Options) int {
	if file == "-" {
		if options.FormatOnly {
			log.Infof("Formatting stdin")
		} else {
			log.Infof("Running on stdin")
		}
		return processOneStream(s, os.Stdin, options)
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
	code := processOneStream(s, f, options)
	f.Close()
	return code
}

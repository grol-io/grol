// Gorepl is a simple interpreted language with a syntax similar to Go.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/progressbar"
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

func Main() (retcode int) { //nolint:funlen // we do have quite a lot of flags and variants.
	commandFlag := flag.String("c", "", "command/inline script to run instead of interactive mode")
	showParse := flag.Bool("parse", false, "show parse tree")
	allParens := flag.Bool("parse-debug", false, "show all parenthesis in parse tree (default is to simplify using precedence)")
	format := flag.Bool("format", false, "don't execute, just parse and reformat the input")
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
	restrictIOs := flag.Bool("restrict-io", false, "restrict IOs (safe mode)")
	emptyOnly := flag.Bool("empty-only", false, "only allow load()/save() to ./.gr")
	noAuto := flag.Bool("no-auto", false, "don't auto load/save the state to ./.gr")
	maxDepth := flag.Int("max-depth", eval.DefaultMaxDepth-1, "Maximum interpreter depth")
	maxLen := flag.Int("max-save-len", 4000, "Maximum len of saved identifiers, use 0 for unlimited")
	panicOk := flag.Bool("panic", false, "Don't catch panic - only for development/debugging")
	// Use 0 (unlimited) as default now that you can ^C to stop a script.
	maxDuration := flag.Duration("max-duration", 0, "Maximum duration for a script to run. 0 for unlimited.")
	shebangMode := flag.Bool("s", false, "#! script mode: next argument is a script file to run, rest are args to the script")
	noRegister := flag.Bool("no-register", false, "Don't use registers")
	noProgress := flag.Bool("no-progress", false, "Don't show progress bar even when processing multiple files")

	cli.ArgsHelp = "*.gr files to interpret or `-` for stdin without prompt or no arguments for stdin repl..."
	cli.MaxArgs = -1
	cli.Main()
	var histFile string
	if !*shebangMode { //nolint:nestif // shebang mode skips a few things like history, memory and welcome message.
		histFile = *historyFile
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
	}
	options := repl.Options{
		ShowParse:   *showParse || *allParens,
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
		AllParens:   *allParens,
		MaxDuration: *maxDuration,
		ShebangMode: *shebangMode,
		NoReg:       *noRegister,
	}
	if hookBefore != nil {
		retcode = hookBefore()
		if retcode != 0 {
			return retcode
		}
	}
	defer func() {
		if hookAfter != nil {
			retcode += hookAfter()
		}
		log.Infof("All done - retcode: %d", retcode)
	}()
	c := extensions.Config{
		HasLoad:           !*disableLoadSave,
		HasSave:           !*disableLoadSave,
		UnrestrictedIOs:   !*restrictIOs,
		LoadSaveEmptyOnly: *emptyOnly,
	}
	err := extensions.Init(&c)
	if err != nil {
		return log.FErrf("Error initializing extensions: %v", err)
	}
	if *commandFlag != "" {
		res, errs, _ := repl.EvalStringWithOption(context.Background(), options, *commandFlag)
		// Only parsing errors are already logged, eval errors aren't, we (re)log everything:
		numErrs := len(errs)
		if numErrs > 0 {
			log.Errf("Total %d %s:\n%s", numErrs, cli.Plural(numErrs, "error"), strings.Join(errs, "\n"))
		}
		fmt.Print(res)
		return numErrs
	}
	if len(flag.Args()) == 0 {
		return repl.Interactive(options)
	}
	options.All = true
	s := eval.NewState()
	s.NoReg = *noRegister
	if options.ShebangMode {
		script := flag.Arg(0)
		// remaining := flag.Args()[1:] // actually let's also pass the name of the script as arg[0]
		options.AutoLoad = false
		args := s.SetArgs(flag.Args())
		log.Infof("Running #! %s with args %s", script, args.Inspect())
		return processOneFile(script, s, options, false)
	}
	files := flag.Args()
	numFiles := len(files)
	// Only use the progress bar if we have more than 1 file as input. eg. in `make grol-tests`
	// and not disabled and stderr is a tty.
	// progress bar also breaks check_tests_double_format.sh so we disable it for formatting.
	usePbar := numFiles > 1 && !*noProgress && log.ConsoleLogging() && !options.FormatOnly
	var pbar *progressbar.Bar
	if usePbar {
		cfg := progressbar.DefaultConfig()
		cfg.NoPercent = true
		cfg.UpdateInterval = 0
		log.SetOutput(os.Stdout)     // recalc color mode based on whether stdout is redirected.
		cfg.NoAnsi = !log.Color      // reuse logger color/terminal detection.
		cfg.ScreenWriter = os.Stdout // lets use std for grol-tests, examples etc but not formatting.
		pbar = cfg.NewBar()
		pbarWriter := pbar.Writer()
		log.Config.ForceColor = log.Color // preserve color mode before it gets reset by output change.
		log.SetOutput(pbarWriter)
		s.Out = pbarWriter
		s.LogOut = pbarWriter
	}
	for i, file := range files {
		if usePbar {
			perc := float64(i*100.) / float64(numFiles)
			pbar.UpdateSuffix(fmt.Sprintf(" %d/%d: %s", i+1, numFiles, file))
			pbar.Progress(perc)
		}
		ret := processOneFile(file, s, options, usePbar)
		if ret != 0 {
			return ret // already logged errors.
		}
		if !*sharedState {
			ns := eval.NewState()
			ns.Out = s.Out
			ns.LogOut = s.LogOut
			s = ns
		}
	}
	if usePbar {
		pbar.UpdateSuffix(fmt.Sprintf(" %d/%d: all done!", numFiles, numFiles))
		pbar.Progress(100)
		pbar.End()
	}
	return 0
}

func processOneStream(s *eval.State, in io.Reader, options repl.Options) int {
	errs := repl.EvalAll(s, in, s.Out, options)
	switch n := len(errs); n {
	case 0:
		return 0
	case 1:
		log.Errf("Error in %s: %v", s.CurrentFile, errs[0])
		return 1
	default:
		log.Errf("Errors in %s: %v", s.CurrentFile, errs)
		return n
	}
}

func processOneFile(file string, s *eval.State, options repl.Options, usePbar bool) int {
	if file == "-" {
		if options.FormatOnly {
			log.Infof("Formatting stdin")
		} else {
			log.Infof("Running on stdin")
		}
		s.CurrentFile = "<stdin>"
		return processOneStream(s, os.Stdin, options)
	}
	s.CurrentFile = file
	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("%v", err)
	}
	verb := "Running"
	if options.FormatOnly {
		verb = "Formatting"
	}
	if !options.ShebangMode && !usePbar {
		log.Infof("%s %s", verb, file)
	}
	code := processOneStream(s, f, options)
	f.Close()
	return code
}

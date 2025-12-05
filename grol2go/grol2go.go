// Grol2go (will eventually) transpile grol scripts to Go code.
// Doesn't yet transpile, it just makes a go binary that runs the grol code.
package main

import (
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"fortio.org/cli"
	"fortio.org/log"
)

func main() {
	os.Exit(Main())
}

// mainCode is the current generated main.go file content.
const mainCode = `package main

import (
	"context"
	"fmt"
	"os"

	"grol.io/grol/eval"
	"grol.io/grol/extensions"
	"grol.io/grol/repl"
)

func errorAndExit(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintln(os.Stderr)
	os.Exit(1)
}

func main() {
	c := extensions.Config{
		UnrestrictedIOs: true,
	}
	err := extensions.Init(&c)
	if err != nil {
		errorAndExit("Error initializing extensions: %v", err)
	}
	s := eval.NewState()
	o := repl.Options{}
	_, _, errs, _ := repl.EvalOne(context.Background(), s, grolCode, os.Stdout, o)
	if len(errs) > 0 {
		errorAndExit("Errors during execution: %v", errs)
	}
}

const grolCode = ` + "`"

// Main is the primary entry point for grol2go. It reads grol source files,
// generates a Go module with embedded grol code, and runs go mod tidy.
// Returns 0 on success, or a non-zero error code on failure.
func Main() int {
	cli.MinArgs = 1
	cli.MaxArgs = -1 // unlimited
	cli.ArgsHelp = "file1.gr [file2.gr ...]"
	destFlag := flag.String("dest", ".", "destination directory for generated Go files and package")
	cli.Main()
	dest := *destFlag
	files := flag.Args()
	// Check that there is no go.mod nor main.go already in dest
	if _, err := os.Stat(filepath.Join(dest, "go.mod")); err == nil {
		return log.FErrf("Destination directory %q already contains go.mod", dest)
	}
	if _, err := os.Stat(filepath.Join(dest, "main.go")); err == nil {
		return log.FErrf("Destination directory %q already contains main.go", dest)
	}
	// Create destination directory if it does not exist
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return log.FErrf("Failed to create destination directory %q: %v", dest, err)
	}
	// go mod init in dest:
	moduleName := deriveModuleName(files[0])
	log.Infof("Compiling %d grol %s to Go in %q using module name %q",
		len(files),
		cli.Plural(len(files), "file"),
		dest,
		moduleName)
	if err := runCommand(dest, "go", "mod", "init", moduleName); err != nil {
		return log.FErrf("Failed to initialize go module: %v", err)
	}
	// Create main.go in dest
	mainFilePath := filepath.Join(dest, "main.go")
	mainFile, err := os.Create(mainFilePath)
	if err != nil {
		return log.FErrf("Failed to create main.go in %q: %v", dest, err)
	}
	defer mainFile.Close()
	_, err = mainFile.WriteString(mainCode)
	if err != nil {
		return log.FErrf("Failed to write to main.go in %q: %v", dest, err)
	}
	for _, f := range files {
		err = transpileFileToGo(f, mainFile)
		if err != nil {
			return log.FErrf("Error transpiling %q: %v", f, err)
		}
	}
	// close the backtick
	_, err = mainFile.WriteString("`\n")
	if err != nil {
		return log.FErrf("Failed to write to main.go in %q: %v", dest, err)
	}
	// Run go mod tidy in dest
	log.Infof("Running 'go mod tidy' in %q", dest)
	if err := runCommand(dest, "go", "mod", "tidy"); err != nil {
		return log.FErrf("Failed to run 'go mod tidy': %v", err)
	}
	log.Infof("Code embedding completed successfully.Run with:\ngo build %s\n./%s", dest, moduleName)
	return 0
}

func transpileFileToGo(srcFile string, mainFile *os.File) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close()
	// Implementation of the transpilation logic goes here (eventually).
	_, err = io.Copy(mainFile, f)
	if err != nil {
		return err
	}
	_, err = mainFile.WriteString("\n")
	if err != nil {
		return err
	}
	return nil
}

// deriveModuleName derives a Go module name from a grol source file path.
func deriveModuleName(srcFile string) string {
	// Use the filename without extension as module name
	base := filepath.Base(srcFile)
	ext := filepath.Ext(base)
	return base[:len(base)-len(ext)]
}

// runCommand runs a command in the specified directory.
func runCommand(dir string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec,noctx // we want to run command based on input.
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

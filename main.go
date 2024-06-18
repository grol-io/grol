package main

import (
	"os"

	"fortio.org/cli"
	"github.com/ldemailly/gorpl/repl"
)

func main() {
	cli.Main()
	repl.Start(os.Stdin, os.Stdout)
}

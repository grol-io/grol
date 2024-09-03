//go:build !tinygo && !windows
// +build !tinygo,!windows

package main_test

import (
	"os"
	"testing"

	"fortio.org/testscript"
	main "grol.io/grol"
)

// TODO: figure out how to make it work on windows - maybe need to use $exe everywhere?
func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"grol": main.Main,
	}))
}

func TestGrolCli(t *testing.T) {
	testscript.Run(t, testscript.Params{Dir: "./"})
}

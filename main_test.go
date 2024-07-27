//go:build !tinygo
// +build !tinygo

package main_test

import (
	"os"
	"testing"

	"fortio.org/testscript"
	main "grol.io/grol"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"grol": main.Main,
	}))
}

func TestGrolCli(t *testing.T) {
	testscript.Run(t, testscript.Params{Dir: "./"})
}

//go:build !no_pprof
// +build !no_pprof

package main

import (
	"flag"
	"os"
	"runtime/pprof"

	"fortio.org/log"
)

var (
	cpuprofile = flag.String("profile-cpu", "", "write cpu profile to `file`")
	memprofile = flag.String("profile-mem", "", "write memory profile to `file`")
)

func init() {
	hookBefore = pprofBeforeHook
	hookAfter = pprofAfterHook
}

func pprofBeforeHook() int {
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
	}
	return 0
}

func pprofAfterHook() int {
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
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

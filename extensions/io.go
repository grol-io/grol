package extensions

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"fortio.org/log"
	"grol.io/grol/eval"
	"grol.io/grol/object"
)

var seenEOF = object.FALSE

func createIOFunctions() {
	// This can hang so not to be used in wasm/discord/...
	ioFn := object.Extension{
		Name:     "read",
		MinArgs:  0,
		MaxArgs:  0,
		Help:     "reads one line from stdin",
		Category: object.CategoryIO,
		Callback: func(env any, _ string, _ []object.Object) object.Object {
			s := env.(*eval.State)
			// Flush output buffer before reading
			s.FlushOutput()
			if s.Term != nil {
				s.Term.Suspend()
				//nolint:fatcontext // we do need to update/reset the context and its cancel function.
				s.Context, s.Cancel = context.WithCancel(context.Background()) // no timeout.
				defer func() {
					//nolint:fatcontext // we do need to update/reset the context and its cancel function.
					s.Context, s.Cancel = s.Term.Resume(context.Background())
				}()
			}
			var linebuf strings.Builder
			// reading one byte at a time is pretty inefficient, but necessary because of the terminal raw mode switch/switchback.
			var b [1]byte
			for {
				n, err := os.Stdin.Read(b[:])
				if n == 1 {
					linebuf.WriteByte(b[0])
				}
				if errors.Is(err, io.EOF) {
					seenEOF = object.TRUE
					break
				}
				if b[0] == '\n' {
					break
				}
				if err != nil {
					log.Errf("Error reading stdin: %v", err)
					return s.Error(err)
				}
			}
			return object.String{Value: linebuf.String()}
		},
		DontCache: true,
	}
	MustCreate(ioFn)
	ioFn.Name = "eof"
	ioFn.Help = "returns true if a previous read hit the end of file for stdin"
	ioFn.Category = object.CategoryIO
	ioFn.Callback = func(_ any, _ string, _ []object.Object) object.Object {
		r := seenEOF
		seenEOF = object.FALSE
		return r
	}
	MustCreate(ioFn)
	ioFn.Name = "flush"
	ioFn.Help = "flushes output and disable caching/memoization"
	ioFn.Category = object.CategoryIO
	ioFn.Callback = func(env any, _ string, _ []object.Object) object.Object {
		s := env.(*eval.State)
		s.FlushOutput()
		return object.NULL
	}
	MustCreate(ioFn)
}

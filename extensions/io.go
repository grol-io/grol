package extensions

import (
	"errors"
	"io"
	"os"
	"strings"

	"fortio.org/log"
	"grol.io/grol/eval"
	"grol.io/grol/object"
)

var seenEOF = object.FALSE

func createIOFunctions() { //nolint:gocognit // we have multiple functions in here.
	// This can hang so not to be used in wasm/discord/...
	ioFn := object.Extension{
		Name:     "read",
		MinArgs:  0,
		MaxArgs:  2,
		Help:     "reads one line from stdin or just n characters if a number is provided, non blocking if boolean is true",
		Category: object.CategoryIO,
		ArgTypes: []object.Type{object.INTEGER, object.BOOLEAN},
		Callback: func(env any, _ string, args []object.Object) object.Object {
			s := env.(*eval.State)
			// Flush output buffer before reading
			s.FlushOutput()
			// reading one byte at a time is pretty inefficient, but necessary because of the terminal raw mode switch/switchback.
			lineMode := true
			from := io.Reader(os.Stdin)
			to := io.Discard
			if s.Term != nil {
				from = s.Term.IntrReader
				to = s.Term.Out
			}
			sz := 1
			nonBlocking := false
			if len(args) >= 1 {
				lineMode = false
				sz = int(args[0].(object.Integer).Value)
				if sz <= 0 {
					return s.Errorf("Invalid number of bytes to read: %d", sz)
				}
				if len(args) >= 2 {
					nonBlocking = args[1].(object.Boolean).Value
				}
				if s.Term == nil && nonBlocking {
					return s.Errorf("Non-blocking read is not supported on non-terminal input")
				}
			}
			var linebuf strings.Builder
			b := make([]byte, sz)
			for done := false; !done; {
				if !lineMode {
					done = true // single read when not wanting a full line.
				}
				var n int
				var err error
				if nonBlocking {
					n, err = s.Term.IntrReader.ReadNonBlocking(b)
				} else {
					n, err = from.Read(b)
				}
				if lineMode && n > 0 {
					// In raw terminal mode, the read() call will always return \r when enter is pressed, yet just in case we also check for \n.
					_, _ = to.Write(b[:n]) // echo including the \r that Out will convert to \r\n
					if b[n-1] == '\r' || b[n-1] == '\n' {
						n-- // exclude the \r or \n itself.
						done = true
					}
				}
				if n >= 1 {
					linebuf.Write(b[:n])
				}
				if errors.Is(err, io.EOF) {
					seenEOF = object.TRUE
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
	ioFn.ArgTypes = []object.Type{}
	ioFn.MinArgs = 0
	ioFn.MaxArgs = 0
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
	ioFn.Name = "term.size"
	ioFn.Help = "Returns the size of the terminal"
	ioFn.Category = object.CategoryIO
	ioFn.Callback = func(env any, _ string, _ []object.Object) object.Object {
		s := env.(*eval.State)
		if s.Term == nil {
			return object.NULL
		}
		err := s.Term.UpdateSize()
		if err != nil {
			return s.Error(err)
		}
		return object.MakeQuad(
			object.String{Value: "width"}, object.Integer{Value: int64(s.Term.Width)},
			object.String{Value: "height"}, object.Integer{Value: int64(s.Term.Height)},
		)
	}
	MustCreate(ioFn)
}

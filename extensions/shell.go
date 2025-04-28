package extensions

import (
	"bytes"
	"context"
	"os"
	"os/exec"

	"fortio.org/log"
	"grol.io/grol/eval"
	"grol.io/grol/object"
)

func createCmd(s eval.State, args []object.Object) (*exec.Cmd, *object.Error) {
	cmdArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.Type() != object.STRING {
			return nil, s.Errorfp("exec: argument %s not a string", arg.Inspect())
		}
		cmdArgs = append(cmdArgs, arg.(object.String).Value)
	}
	//nolint:gosec // we do want to run the command given by the user.
	return exec.CommandContext(s.Context, cmdArgs[0], cmdArgs[1:]...), nil
}

var (
	stdout = object.String{Value: "stdout"}
	stderr = object.String{Value: "stderr"}
)

func createShellFunctions() {
	shellFn := object.Extension{
		Name:     "exec",
		MinArgs:  1,
		MaxArgs:  -1,
		Help:     "executes a command and returns its stdout, stderr and any error",
		Category: object.CategoryIO,
		ArgTypes: []object.Type{object.STRING},
		Callback: func(env any, _ string, args []object.Object) object.Object {
			s := env.(*eval.State)
			cmd, oerr := createCmd(*s, args)
			if oerr != nil {
				return *oerr
			}
			log.Infof("Running %#v", cmd)
			var sout, serr bytes.Buffer
			cmd.Stdout = &sout
			cmd.Stderr = &serr
			if obj := s.GetPipeValue(); obj != nil {
				cmd.Stdin = bytes.NewReader(obj)
			}
			err := cmd.Run()
			// keys must be sorted. stdErr before stdOut.
			res := object.MakeQuad(stderr, object.String{Value: serr.String()},
				stdout, object.String{Value: sout.String()})
			if err != nil {
				res = res.Set(eval.ErrorKey, object.String{Value: err.Error()})
			} else {
				res = res.Set(eval.ErrorKey, object.NULL)
			}
			return res
		},
		DontCache: true,
	}
	MustCreate(shellFn)
	shellFn.Name = "run"
	shellFn.Help = "runs a command interactively"
	shellFn.Callback = func(env any, _ string, args []object.Object) object.Object {
		s := env.(*eval.State)
		if s.Term != nil {
			s.Term.Suspend()
		}
		//nolint:fatcontext // we do need to update/reset the context and its cancel function.
		s.Context, s.Cancel = context.WithCancel(context.Background()) // no timeout.
		cmd, oerr := createCmd(*s, args)
		if oerr != nil {
			return *oerr
		}
		log.Infof("Running %#v", cmd)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if s.Term != nil {
			s.Context, s.Cancel = s.Term.Resume(context.Background())
		}
		if err != nil {
			return s.Error(err)
		}
		return object.NULL
	}
	MustCreate(shellFn)
}

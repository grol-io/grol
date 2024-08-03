package object

import (
	"errors"
)

var extraFunctions map[string]Extension

// Init resets the table of extended functions to empty.
// Optional, will be called on demand the first time through CreateFunction.
func Init() {
	extraFunctions = make(map[string]Extension)
}

// CreateFunction adds a new function to the table of extended functions.
func CreateFunction(cmd Extension) error {
	if extraFunctions == nil {
		Init()
	}
	if cmd.Name == "" {
		return errors.New("empty command name")
	}
	if cmd.MaxArgs != -1 && cmd.MinArgs > cmd.MaxArgs {
		return errors.New(cmd.Name + ": min args > max args")
	}
	if len(cmd.ArgTypes) < cmd.MinArgs {
		return errors.New(cmd.Name + ": arg types < min args")
	}
	if _, ok := extraFunctions[cmd.Name]; ok {
		return errors.New(cmd.Name + ": already defined")
	}
	extraFunctions[cmd.Name] = cmd
	return nil
}

func ExtraFunctions() map[string]Extension {
	return extraFunctions
}

func Unwrap(objs []Object) []any {
	res := make([]any, len(objs))
	for i, o := range objs {
		res[i] = o.Unwrap()
	}
	return res
}

package object

import (
	"errors"
	"fmt"
	"strings"

	"grol.io/grol/lexer"
)

type ExtensionMap map[string]Extension

var (
	extraFunctions   ExtensionMap
	extraIdentifiers map[string]Object
	initDone         bool
)

// Init resets the table of extended functions to empty.
// Optional, will be called on demand the first time through CreateFunction.
func Init() {
	extraFunctions = make(ExtensionMap)
	extraIdentifiers = make(map[string]Object)
	initDone = true
}

func ValidIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for _, b := range []byte(name) {
		if !lexer.IsAlphaNum(b) {
			return false
		}
	}
	return true
}

// CreateFunction adds a new function to the table of extended functions.
func CreateFunction(cmd Extension) error {
	if !initDone {
		Init()
	}
	if cmd.Name == "" {
		return errors.New("empty command name")
	}
	// Only support 1 level of namespace for now.
	dotSplit := strings.SplitN(cmd.Name, ".", 2)
	var ns string
	name := cmd.Name
	if len(dotSplit) == 2 {
		ns, name = dotSplit[0], dotSplit[1]
		if !ValidIdentifier(ns) {
			return fmt.Errorf("namespace %q not alphanumeric", ns)
		}
	}
	if !ValidIdentifier(name) {
		return errors.New(name + ": not alphanumeric")
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
	cmd.Variadic = (cmd.MaxArgs == -1) || (cmd.MaxArgs > cmd.MinArgs)
	// If namespaced, put both at top level (for sake of baseinfo and command completion) and
	// in namespace map (for access/ref by eval). We decided to not even have namespaces map
	// after all.
	extraFunctions[cmd.Name] = cmd
	return nil
}

// Returns the table of extended functions to seed the state of an eval.
func ExtraFunctions() ExtensionMap {
	// no need to make a copy as each value need to be set to be changed (map of structs, not pointers).
	return extraFunctions
}

func IsExtraFunction(name string) bool {
	_, ok := extraFunctions[name]
	return ok
}

// Add values to top level environment, e.g "pi" -> 3.14159...
// or "printf(){print(sprintf(%s, args...))}".
func AddIdentifier(name string, value Object) {
	if !initDone {
		Init()
	}
	extraIdentifiers[name] = value
}

func isConstantAndExtraIdentifier(name string) bool {
	if !Constant(name) {
		return false
	}
	_, ok := extraIdentifiers[name]
	return ok
}

// This makes a copy of the extraIdentifiers map to serve as initial Environment without mutating the original.
// use to setup the root environment for the interpreter state.
func initialIdentifiersCopy() map[string]Object {
	if !initDone {
		Init()
	}
	// we'd use maps.Clone except for tinygo not having it.
	// https://github.com/tinygo-org/tinygo/issues/4382
	copied := make(map[string]Object, len(extraIdentifiers))
	for k, v := range extraIdentifiers {
		copied[k] = v
	}
	return copied
}

func Unwrap(objs []Object, forceStringKeys bool) []any {
	res := make([]any, len(objs))
	for i, o := range objs {
		res[i] = o.Unwrap(forceStringKeys)
	}
	return res
}

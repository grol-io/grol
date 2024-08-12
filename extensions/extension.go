// Package mappings some go built in functions to grol functions.
// Same mechanism can be used to map other go functions to grol functions and further extend the language.
package extensions

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"fortio.org/log"
	"grol.io/grol/eval"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
)

var (
	initDone  = false
	errInInit error
	// These are a bit ugly as globals, maybe lambda capture and/or receivers on config instead.
	unrestrictedIOs = false
	emptyOnly       = false
)

const GrolFileExtension = ".gr" // Also the default filename for LoadSaveEmptyOnly.

// Configure restrictions and features.
// Currently about IOs of load and save functions.
type Config struct {
	HasLoad           bool // load() only present if this is true.
	HasSave           bool // save() only present if this is true.
	LoadSaveEmptyOnly bool // Restrict load/save to a single .gr file inside the current directory.
	UnrestrictedIOs   bool // Dangerous when true: can overwrite files, read any readable file etc...
}

// Init initializes the extensions, can be called multiple time safely but should really be called only once
// before using GROL repl/eval. If the passed [Config] pointer is nil, default (safe) values are used.
func Init(c *Config) error {
	if initDone {
		return errInInit
	}
	if c == nil {
		c = &Config{}
	}
	errInInit = initInternal(c)
	initDone = true
	return errInInit
}

type OneFloatInOutFunc func(float64) float64

func initInternal(c *Config) error {
	unrestrictedIOs = c.UnrestrictedIOs
	emptyOnly = c.LoadSaveEmptyOnly
	cmd := object.Extension{
		Name:     "pow",
		MinArgs:  2,
		MaxArgs:  2,
		ArgTypes: []object.Type{object.FLOAT, object.FLOAT},
		Callback: object.ShortCallback(pow),
	}
	err := object.CreateFunction(cmd)
	if err != nil {
		return err
	}
	err = object.CreateFunction(object.Extension{
		Name:     "sprintf",
		MinArgs:  1,
		MaxArgs:  -1,
		ArgTypes: []object.Type{object.STRING},
		Callback: object.ShortCallback(sprintf),
	})
	if err != nil {
		return err
	}
	// for printf, we could expose current eval "Out", but instead let's use new variadic support and define
	// printf as print(snprintf(format,..)) that way the memoization of output also works out of the box.
	err = eval.AddEvalResult("printf", "func(format, ..){print(sprintf(format, ..))}")
	if err != nil {
		return err
	}
	err = eval.AddEvalResult("abs", "func(x){if x < 0 {-x} else {x}}")
	if err != nil {
		return err
	}
	err = eval.AddEvalResult("log2", "func(x) {ln(x)/ln(2)}")
	if err != nil {
		return err
	}
	oneFloat := object.Extension{
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.FLOAT},
	}
	for _, function := range []struct {
		fn   OneFloatInOutFunc
		name string
	}{
		{math.Sin, "sin"},
		{math.Cos, "cos"},
		{math.Tan, "tan"},
		{math.Log, "ln"}, // proper name for natural logarithm and also doesn't conflict with logger builtin.
		{math.Sqrt, "sqrt"},
		{math.Exp, "exp"},
		{math.Asin, "asin"},
		{math.Acos, "acos"},
		{math.Atan, "atan"},
		{math.Round, "round"},
		{math.Trunc, "trunc"},
		{math.Floor, "floor"},
		{math.Ceil, "ceil"},
		{math.Log10, "log10"},
	} {
		oneFloat.Callback = func(_ any, _ string, args []object.Object) object.Object {
			// Arg len check already done through MinArgs=MaxArgs=1 and
			// type through ArgTypes: []object.Type{object.FLOAT}.
			return object.Float{Value: function.fn(args[0].(object.Float).Value)}
		}
		oneFloat.Name = function.name
		err = object.CreateFunction(oneFloat)
		if err != nil {
			return err
		}
	}
	object.AddIdentifier("PI", object.Float{Value: math.Pi})
	object.AddIdentifier("E", object.Float{Value: math.E}) // using uppercase so "e" isn't taken/shadowed.
	jsonFn := object.Extension{
		Name:     "json",
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.ANY},
		Callback: object.ShortCallback(jsonSer),
	}
	err = object.CreateFunction(jsonFn)
	if err != nil {
		return err
	}
	jsonFn.Name = "eval"
	jsonFn.Callback = evalFunc
	jsonFn.ArgTypes = []object.Type{object.STRING}
	err = object.CreateFunction(jsonFn)
	if err != nil {
		return err
	}
	jsonFn.Name = "unjson"
	jsonFn.Callback = evalFunc // unjson at the moment is just (like) eval hoping that json is map/array/...
	err = object.CreateFunction(jsonFn)
	if err != nil {
		return err
	}
	loadSaveFn := object.Extension{
		MinArgs:  0, // empty only case - ie ".gr" save file.
		MaxArgs:  1,
		ArgTypes: []object.Type{object.STRING},
	}
	if c.HasSave {
		loadSaveFn.Name = "save"
		loadSaveFn.Callback = saveFunc // save to file.
		err = object.CreateFunction(loadSaveFn)
		if err != nil {
			return err
		}
	}
	if c.HasLoad {
		loadSaveFn.Name = "load"
		loadSaveFn.Callback = loadFunc // eval a file.
		err = object.CreateFunction(loadSaveFn)
		if err != nil {
			return err
		}
	}
	return nil
}

func pow(args []object.Object) object.Object {
	// Arg len check already done through MinArgs and MaxArgs
	// and so is type check through ArgTypes.
	base := args[0].(object.Float).Value
	exp := args[1].(object.Float).Value
	result := math.Pow(base, exp)
	return object.Float{Value: result}
}

func sprintf(args []object.Object) object.Object {
	res := fmt.Sprintf(args[0].(object.String).Value, object.Unwrap(args[1:])...)
	return object.String{Value: res}
}

func jsonSer(args []object.Object) object.Object {
	w := strings.Builder{}
	err := args[0].JSON(&w)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	return object.String{Value: w.String()}
}

func evalFunc(env any, name string, args []object.Object) object.Object {
	s := args[0].(object.String).Value
	res, err := eval.EvalString(env, s, name == "unjson" /* empty env */)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	return res
}

// Normalizes to alphanum.gr.
func sanitizeFileName(args []object.Object) (string, error) {
	if len(args) == 0 {
		return GrolFileExtension, nil
	}
	file := args[0].(object.String).Value
	if emptyOnly && file != "" {
		return "", fmt.Errorf("empty only mode, filename must be empty or no arguments, got: %q", file)
	}
	if unrestrictedIOs {
		log.Infof("Unrestricted IOs, not sanitizing filename: %s", file)
		return file, nil
	}
	// only alhpanumeric and _ allowed. no dots, slashes, etc.
	f := strings.TrimSuffix(file, GrolFileExtension)
	for _, r := range []byte(f) {
		if !lexer.IsAlphaNum(r) {
			return "", fmt.Errorf("invalid character in filename %q: %c", file, r)
		}
	}
	return f + GrolFileExtension, nil
}

func saveFunc(env any, _ string, args []object.Object) object.Object {
	eval := env.(*eval.State)
	file, err := sanitizeFileName(args)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	f, err := os.Create(file)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	defer f.Close()
	// Write to file.
	n, err := eval.SaveGlobals(f)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	log.Infof("Saved %d ids/fns to: %s", n, file)
	return object.Map{
		object.String{Value: "entries"}:  object.Integer{Value: int64(n)},
		object.String{Value: "filename"}: object.String{Value: file},
	}
}

func loadFunc(env any, _ string, args []object.Object) object.Object {
	file, err := sanitizeFileName(args)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	f, err := os.Open(file)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	defer f.Close()
	all, err := io.ReadAll(f)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	// Eval the content.
	res, err := eval.EvalString(env, string(all), false)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	log.Infof("Read/evaluated: %s", file)
	return res
}

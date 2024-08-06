// Package mappings some go built in functions to grol functions.
// Same mechanism can be used to map other go functions to grol functions and further extend the language.
package extensions

import (
	"encoding/json"
	"fmt"
	"math"

	"grol.io/grol/eval"
	"grol.io/grol/object"
)

var (
	initDone  = false
	errInInit error
)

func Init() error {
	if initDone {
		return errInInit
	}
	errInInit = initInternal()
	initDone = true
	return errInInit
}

type OneFloatInOutFunc func(float64) float64

func initInternal() error {
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
		MaxArgs:  2,
		ArgTypes: []object.Type{object.ANY, object.BOOLEAN},
		Callback: object.ShortCallback(jsonFunc),
	}
	err = object.CreateFunction(jsonFn)
	if err != nil {
		return err
	}
	jsonFn.Name = "eval"
	jsonFn.Callback = evalFunc
	jsonFn.ArgTypes = []object.Type{object.STRING}
	jsonFn.MaxArgs = 1
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

func convertMap(m map[any]any) map[string]any {
	result := make(map[string]any, len(m))
	for key, value := range m {
		if valueMap, ok := value.(map[any]any); ok {
			value = convertMap(valueMap)
		}
		result[fmt.Sprint(key)] = value
	}
	return result
}

func jsonFunc(args []object.Object) object.Object {
	v := args[0].Unwrap()
	if valueMap, ok := v.(map[any]any); ok {
		v = convertMap(valueMap)
	}
	doIndent := (len(args) > 1) && args[1].(object.Boolean).Value
	var b []byte
	var err error
	if doIndent {
		b, err = json.MarshalIndent(v, "", "  ")
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	return object.String{Value: string(b)}
}

func evalFunc(env any, name string, args []object.Object) object.Object {
	s := args[0].(object.String).Value
	res, err := eval.EvalString(env, s, name == "unjson" /* empty env */)
	if err != nil {
		return object.Error{Value: err.Error()}
	}
	return res
}

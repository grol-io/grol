package extensions

import (
	"math"
	"reflect"
	"runtime"
	"strings"

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

func FunctionName(f any) string {
	val := reflect.ValueOf(f)
	if val.Kind() != reflect.Func {
		return ""
	}
	fullName := runtime.FuncForPC(val.Pointer()).Name()
	return fullName
}

func ShortFunctionName(f any) string {
	fullName := FunctionName(f)
	if fullName == "" {
		return ""
	}
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot == -1 {
		return fullName
	}
	return fullName[lastDot+1:]
}

func MathFunctionName(f any) string {
	s := strings.ToLower(ShortFunctionName(f))
	if s == "log" {
		return "ln"
	}
	return s
}

func initInternal() error {
	cmd := object.Extension{
		Name:     "pow",
		MinArgs:  2,
		MaxArgs:  2,
		ArgTypes: []object.Type{object.FLOAT, object.FLOAT},
		Callback: pow,
	}
	err := object.CreateCommand(cmd)
	if err != nil {
		return err
	}
	oneFloat := object.Extension{
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.FLOAT},
	}
	for _, function := range []OneFloatInOutFunc{
		math.Sin,
		math.Cos,
		math.Tan,
		math.Log, // renamed to ln in MathFunctionName
		math.Sqrt,
		math.Exp,
		math.Asin,
		math.Acos,
		math.Atan,
		math.Round,
		math.Trunc,
	} {
		oneFloat.Callback = func(args []object.Object) object.Object {
			// Arg len check already done through MinArgs=MaxArgs=1 and
			// type through ArgTypes: []object.Type{object.FLOAT}.
			return object.Float{Value: function(args[0].(object.Float).Value)}
		}
		oneFloat.Name = MathFunctionName(function)
		err = object.CreateCommand(oneFloat)
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

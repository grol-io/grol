package extensions

import (
	"math"

	"grol.io/grol/object"
)

func init() {
	Init()
}

func Init() {
	cmd := object.Extension{
		Name:     "pow",
		MinArgs:  2,
		MaxArgs:  2,
		ArgTypes: []object.Type{object.FLOAT, object.FLOAT},
		Callback: pow,
	}
	err := object.CreateCommand(cmd)
	if err != nil {
		panic(err)
	}
	cmd = object.Extension{
		Name:     "sin",
		MinArgs:  1,
		MaxArgs:  1,
		ArgTypes: []object.Type{object.FLOAT},
		Callback: sin,
	}
	err = object.CreateCommand(cmd)
	if err != nil {
		panic(err)
	}
	cmd.Name = "cos"
	cmd.Callback = cos
	err = object.CreateCommand(cmd)
	if err != nil {
		panic(err)
	}
	cmd.Name = "tan"
	cmd.Callback = tan
	err = object.CreateCommand(cmd)
	if err != nil {
		panic(err)
	}
	cmd.Name = "ln"
	cmd.Callback = ln
	err = object.CreateCommand(cmd)
	if err != nil {
		panic(err)
	}
}

func pow(args []object.Object) object.Object {
	// Arg len check already done through MinArgs and MaxArgs
	// and so is type check through ArgTypes.
	base := args[0].(object.Float).Value
	exp := args[1].(object.Float).Value
	result := math.Pow(base, exp)
	return object.Float{Value: result}
}

func sin(args []object.Object) object.Object {
	return object.Float{Value: math.Sin(args[0].(object.Float).Value)}
}
func cos(args []object.Object) object.Object {
	return object.Float{Value: math.Cos(args[0].(object.Float).Value)}
}
func tan(args []object.Object) object.Object {
	return object.Float{Value: math.Tan(args[0].(object.Float).Value)}
}
func ln(args []object.Object) object.Object {
	return object.Float{Value: math.Log(args[0].(object.Float).Value)}
}

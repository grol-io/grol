package object_test

import (
	"testing"

	"grol.io/grol/object"
)

func TestStringMapKey(t *testing.T) {
	hello1 := object.String{Value: "Hello World"}
	hello2 := object.String{Value: "Hello World"}
	diff1 := object.String{Value: "My name is johnny"}
	diff2 := object.String{Value: "My name is johnny"}
	if &hello1 == &hello2 {
		t.Errorf("strings pointer somehow same, unexpected")
	}

	m := object.NewMap()
	m[hello1] = diff1

	v, ok := m[hello2]
	if !ok {
		t.Errorf("no value found for key %v", hello2)
	}
	if v != diff1 {
		t.Errorf("value for key %v is %v, expected %v", hello2, v, diff1)
	}
	if object.Equals(diff1, diff2) != object.TRUE {
		t.Errorf("values aren't equal, unexpected")
	}
}

func TestExtensionUsage(t *testing.T) {
	cmd := object.Extension{
		Name: "cmdname",
		ArgTypes: []object.Type{
			object.INTEGER,
			object.FLOAT,
			object.STRING,
		},
		MinArgs: 3,
		MaxArgs: 6,
	}
	actual := cmd.Inspect()
	expected := "cmdname(integer, float, string, arg4..arg6)"
	if actual != expected {
		t.Errorf("cmd.Inspect() test 3-6 args got %q, expected %q", actual, expected)
	}
	cmd.MaxArgs = -1
	actual = cmd.Inspect()
	expected = "cmdname(integer, float, string, ..)"
	if actual != expected {
		t.Errorf("cmd.Inspect() test unlimited args got %q, expected %q", actual, expected)
	}
}

// Code generated by "stringer -type=Type"; DO NOT EDIT.

package object

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[UNKNOWN-0]
	_ = x[INTEGER-1]
	_ = x[FLOAT-2]
	_ = x[BOOLEAN-3]
	_ = x[NIL-4]
	_ = x[ERROR-5]
	_ = x[RETURN-6]
	_ = x[FUNC-7]
	_ = x[STRING-8]
	_ = x[ARRAY-9]
	_ = x[MAP-10]
	_ = x[QUOTE-11]
	_ = x[MACRO-12]
	_ = x[EXTENSION-13]
	_ = x[REFERENCE-14]
	_ = x[ANY-15]
}

const _Type_name = "UNKNOWNINTEGERFLOATBOOLEANNILERRORRETURNFUNCSTRINGARRAYMAPQUOTEMACROEXTENSIONREFERENCEANY"

var _Type_index = [...]uint8{0, 7, 14, 19, 26, 29, 34, 40, 44, 50, 55, 58, 63, 68, 77, 86, 89}

func (i Type) String() string {
	if i >= Type(len(_Type_index)-1) {
		return "Type(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Type_name[_Type_index[i]:_Type_index[i+1]]
}

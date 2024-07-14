// Code generated by "stringer -type=Type"; DO NOT EDIT.

package token

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ILLEGAL-0]
	_ = x[EOF-1]
	_ = x[IDENT-2]
	_ = x[INT-3]
	_ = x[ASSIGN-4]
	_ = x[PLUS-5]
	_ = x[MINUS-6]
	_ = x[BANG-7]
	_ = x[ASTERISK-8]
	_ = x[SLASH-9]
	_ = x[PERCENT-10]
	_ = x[LT-11]
	_ = x[GT-12]
	_ = x[LTEQ-13]
	_ = x[GTEQ-14]
	_ = x[EQ-15]
	_ = x[NOTEQ-16]
	_ = x[COMMA-17]
	_ = x[SEMICOLON-18]
	_ = x[LPAREN-19]
	_ = x[RPAREN-20]
	_ = x[LBRACE-21]
	_ = x[RBRACE-22]
	_ = x[LBRACKET-23]
	_ = x[RBRACKET-24]
	_ = x[COLON-25]
	_ = x[LINECOMMENT-26]
	_ = x[STARTCOMMENT-27]
	_ = x[ENDCOMMENT-28]
	_ = x[FUNCTION-29]
	_ = x[LET-30]
	_ = x[TRUE-31]
	_ = x[FALSE-32]
	_ = x[IF-33]
	_ = x[ELSE-34]
	_ = x[RETURN-35]
	_ = x[STRING-36]
	_ = x[MACRO-37]
	_ = x[LEN-38]
	_ = x[FIRST-39]
	_ = x[REST-40]
	_ = x[PRINT-41]
	_ = x[LOG-42]
}

const _Type_name = "ILLEGALEOFIDENTINTASSIGNPLUSMINUSBANGASTERISKSLASHPERCENTLTGTLTEQGTEQEQNOTEQCOMMASEMICOLONLPARENRPARENLBRACERBRACELBRACKETRBRACKETCOLONLINECOMMENTSTARTCOMMENTENDCOMMENTFUNCTIONLETTRUEFALSEIFELSERETURNSTRINGMACROLENFIRSTRESTPRINTLOG"

var _Type_index = [...]uint8{0, 7, 10, 15, 18, 24, 28, 33, 37, 45, 50, 57, 59, 61, 65, 69, 71, 76, 81, 90, 96, 102, 108, 114, 122, 130, 135, 146, 158, 168, 176, 179, 183, 188, 190, 194, 200, 206, 211, 214, 219, 223, 228, 231}

func (i Type) String() string {
	if i >= Type(len(_Type_index)-1) {
		return "Type(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Type_name[_Type_index[i]:_Type_index[i+1]]
}
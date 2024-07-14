// Code generated by "stringer -type=Type"; DO NOT EDIT.

package token

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ILLEGAL-0]
	_ = x[EOL-1]
	_ = x[EOF-2]
	_ = x[IDENT-3]
	_ = x[INT-4]
	_ = x[FLOAT-5]
	_ = x[ASSIGN-6]
	_ = x[PLUS-7]
	_ = x[MINUS-8]
	_ = x[BANG-9]
	_ = x[ASTERISK-10]
	_ = x[SLASH-11]
	_ = x[PERCENT-12]
	_ = x[LT-13]
	_ = x[GT-14]
	_ = x[LTEQ-15]
	_ = x[GTEQ-16]
	_ = x[EQ-17]
	_ = x[NOTEQ-18]
	_ = x[COMMA-19]
	_ = x[SEMICOLON-20]
	_ = x[LPAREN-21]
	_ = x[RPAREN-22]
	_ = x[LBRACE-23]
	_ = x[RBRACE-24]
	_ = x[LBRACKET-25]
	_ = x[RBRACKET-26]
	_ = x[COLON-27]
	_ = x[LINECOMMENT-28]
	_ = x[STARTCOMMENT-29]
	_ = x[ENDCOMMENT-30]
	_ = x[FUNCTION-31]
	_ = x[LET-32]
	_ = x[TRUE-33]
	_ = x[FALSE-34]
	_ = x[IF-35]
	_ = x[ELSE-36]
	_ = x[RETURN-37]
	_ = x[STRING-38]
	_ = x[MACRO-39]
	_ = x[LEN-40]
	_ = x[FIRST-41]
	_ = x[REST-42]
	_ = x[PRINT-43]
	_ = x[LOG-44]
}

const _Type_name = "ILLEGALEOLEOFIDENTINTFLOATASSIGNPLUSMINUSBANGASTERISKSLASHPERCENTLTGTLTEQGTEQEQNOTEQCOMMASEMICOLONLPARENRPARENLBRACERBRACELBRACKETRBRACKETCOLONLINECOMMENTSTARTCOMMENTENDCOMMENTFUNCTIONLETTRUEFALSEIFELSERETURNSTRINGMACROLENFIRSTRESTPRINTLOG"

var _Type_index = [...]uint8{0, 7, 10, 13, 18, 21, 26, 32, 36, 41, 45, 53, 58, 65, 67, 69, 73, 77, 79, 84, 89, 98, 104, 110, 116, 122, 130, 138, 143, 154, 166, 176, 184, 187, 191, 196, 198, 202, 208, 214, 219, 222, 227, 231, 236, 239}

func (i Type) String() string {
	if i >= Type(len(_Type_index)-1) {
		return "Type(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Type_name[_Type_index[i]:_Type_index[i+1]]
}

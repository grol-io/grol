package token

import "testing"

func TestInterning(t *testing.T) {
	Init()
	myToken1 := &Token{Type: IDENT, literal: "myToken 1"}
	myToken2 := &Token{Type: IDENT, literal: "myToken 2"}
	myToken1Again := &Token{Type: IDENT, literal: "myToken 1"}
	if myToken1 == myToken1Again {
		t.Errorf("myToken1 and myToken1Again should not be the same pointer value")
	}
	norm1 := InternToken(myToken1)
	norm2 := InternToken(myToken2)
	norm1Again := InternToken(myToken1Again)
	if norm1 != norm1Again {
		t.Errorf("norm1 and norm1Again should be the same interned pointer")
	}
	if norm1 == norm2 {
		t.Errorf("norm1 and norm2 should not be the same interned pointer")
	}
}

func TestLookup(t *testing.T) {
	Init()
	tf := LookupIdent("func")
	if tf.Type != FUNC {
		t.Errorf("LookupIdent(func) returned %v, expected FUNC", tf)
	}
	if tf.literal != "func" {
		t.Errorf("LookupIdent(func) returned %v, expected 'func'", tf)
	}
	te := LookupIdent("error")
	if te.Type != ERROR { // ERROR is a keyword
		t.Errorf("LookupIdent(error) returned %v, expected ERROR", te)
	}
	if te.literal != "error" {
		t.Errorf("LookupIdent(error) returned %v, expected 'error'", te)
	}
	tu := LookupIdent("unknown")
	if tu.Type != IDENT {
		t.Errorf("LookupIdent(unknown) returned %v, expected IDENT", tu)
	}
	if tu.literal != "unknown" {
		t.Errorf("LookupIdent(unknown) returned %v, expected 'unknown'", tu)
	}
	tf2 := LookupIdent("func")
	if tf != tf2 {
		t.Errorf("LookupIdent(func) returned %v, expected the same as before", tf2)
	}
	tu2 := LookupIdent("unk" + "nown")
	if tu != tu2 {
		t.Errorf("LookupIdent(unknown) returned %v, expected the same as before", tu2)
	}
	tu3 := Intern(IDENT, "unknown")
	if tu != tu3 {
		t.Errorf("Intern(IDENT, 'unknown') returned %v, expected the same as before", tu3)
	}
}

func TestMultiCharTokens(t *testing.T) {
	Init()
	// Test all 2-char tokens
	tests := []struct {
		input    string
		expected Type
	}{
		{"==", EQ},
		{"!=", NOTEQ},
		{">=", GTEQ},
		{"<=", LTEQ},
	}
	for _, tt := range tests {
		tok := &Token{Type: tt.expected, literal: tt.input}
		tok2 := InternToken(tok)
		if tok == tok2 {
			t.Errorf("Intern[%s] was unexpectedly created", tt.input)
		}
		tok3 := ConstantTokenStr(tt.input)
		if tok3 != tok2 {
			t.Errorf("ConstantTokenStr[%s] was not found", tt.input)
		}
	}
}

func TestSingleCharTokens(t *testing.T) {
	Init()
	// Test some 1-char tokens (first and last from range)
	tests := []struct {
		input    byte
		expected Type
	}{
		{'=', ASSIGN},
		{':', COLON},
	}
	for _, tt := range tests {
		tok := ConstantTokenChar(tt.input)
		if tok == nil {
			t.Errorf("ConstantTokenChar[%c] was not found", tt.input)
		}
		if tok.Type != tt.expected {
			t.Errorf("ConstantTokenChar[%c] returned %v, expected %v", tt.input, tok.Type, tt.expected)
		}
		if tok.Literal() != string(tt.input) {
			t.Errorf("ConstantTokenChar[%c] returned %v, expected '%c'", tt.input, tok.Literal(), tt.input)
		}
	}
}

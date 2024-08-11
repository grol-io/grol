package token

import "fortio.org/sets"

// Info enables introspection of known keywords and tokens and operators.
type GrolInfo struct {
	// Keywords is a map of all known keywords.
	Keywords sets.Set[string]
	Builtins sets.Set[string]
	Tokens   sets.Set[string]
}

var info = GrolInfo{}

func Info() GrolInfo {
	return info
}

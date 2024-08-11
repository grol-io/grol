package repl

import (
	"fmt"
	"strings"

	"fortio.org/terminal"
	"grol.io/grol/trie"
)

type AutoComplete struct {
	Trie *trie.Trie
}

func NewCompletion() *AutoComplete {
	return &AutoComplete{trie.NewTrie()}
}

func (a *AutoComplete) AutoComplete() terminal.AutoCompleteCallback {
	return func(t *terminal.Terminal, line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		if key != '\t' {
			return // only tab for now
		}
		return a.autoCompleteCallback(t, line, pos)
	}
}

func (a *AutoComplete) autoCompleteCallback(t *terminal.Terminal, line string, pos int) (newLine string, newPos int, ok bool) {
	l, commands := a.Trie.PrefixAll(line[:pos])
	if len(commands) == 0 {
		return
	}
	if len(commands) > 1 {
		fmt.Fprint(t.Out, "One of: ")
		for _, c := range commands {
			if strings.HasSuffix(c, "(") {
				fmt.Fprint(t.Out, c, ") ")
			} else {
				fmt.Fprint(t.Out, c)
			}
		}
		fmt.Fprintln(t.Out)
	}
	return commands[0][:l], l, true
}

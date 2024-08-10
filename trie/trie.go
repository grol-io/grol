// Trie implements a byte trie data structure.
// It is fast as it uses arrays instead of maps and no bound checks.
package trie // import "grol.io/grol/trie"

type Trie struct {
	// Children of this node
	children [256]*Trie
	// This node itself is a valid leaf (end of a word) in addition having children.
	valid bool
	leaf  bool // Note really needed outside of debugging but with struct alignment it doesn't cost anything extra.
}

// Save some memory by having a shared end marker for leaves.
// Only one having "leaf" set to true.
var endMarker = &Trie{valid: true, leaf: true}

func NewTrie() *Trie {
	return &Trie{}
}

func (t *Trie) Insert(word string) {
	l := len(word)
	for i := range l {
		char := word[i]
		valid := false
		switch t.children[char] {
		case endMarker:
			// This was a valid leaf before, propagate to the new children node
			valid = true
			fallthrough
		case nil:
			if i == l-1 {
				t.children[char] = endMarker // Shared for all leaves, saves memory.
			} else {
				t.children[char] = &Trie{valid: valid}
			}
		}
		t = t.children[char]
	}
}

func (t *Trie) Contains(word string) bool {
	return t.Prefix(word).IsValid()
}

func (t *Trie) Prefix(word string) *Trie {
	for i := range len(word) {
		char := word[i]
		t = t.children[char]
		if t == nil {
			return nil
		}
	}
	return t
}

func (t *Trie) IsLeaf() bool {
	return t != nil && t.leaf
}

func (t *Trie) IsValid() bool {
	return t != nil && t.valid
}

/*
  A-B
  A-B-C

  [A] -> [B] children[C] = endMarker
*/

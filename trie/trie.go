// Trie implements a byte trie data structure.
// It is fast as it uses arrays instead of maps and no bound checks.
package trie // import "grol.io/grol/trie"

type Trie struct {
	// Children of this node
	children [256]*Trie
	// This node itself is a valid leaf (end of a word) in addition having children.
	// optimization for enumeration, not strictly needed.
	min, max byte // min starts at 255, max at 0 so they get set immediately after the first child addition.
	valid    bool
	leaf     bool // Note really needed outside of debugging but with struct alignment it doesn't cost anything extra.
}

// Save some memory by having a shared end marker for leaves.
// Only one having "leaf" set to true.
var endMarker = &Trie{valid: true, leaf: true}

func NewTrie() *Trie {
	return &Trie{min: 255, max: 0}
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
				t.children[char] = &Trie{valid: valid, min: 255, max: 0}
			}
			if char < t.min {
				t.min = char
			}
			if char > t.max {
				t.max = char
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

// All returns all the valid words from that point onwards.
// Typically called from the result of [Prefix].
// if somehow both 0 and 255 are valid yet not much in between,
// the optimization of min,max range won't do much, but for
// normal words, it should help a lot.
func (t *Trie) All(prefix string) []string {
	if t == nil {
		return nil
	}
	var res []string
	if t.valid {
		res = append(res, prefix)
	}
	if t.leaf {
		return res
	}
	for i := t.min; i <= t.max; i++ {
		if t.children[i] != nil {
			newPrefix := prefix + string(i)
			res = append(res, t.children[i].All(newPrefix)...)
		}
	}
	return res
}

/*
  A-B
  A-B-C

  [A] -> [B] children[C] = endMarker
*/

// gets inlined hopefully

func min(a, b byte) byte {
	if a < b {
		return a
	}
	return b
}

func max(a, b byte) byte {
	if a > b {
		return a
	}
	return b
}

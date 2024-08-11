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

// Returns all the matches for the given prefix and the
// length of the longest common prefix.
func (t *Trie) PrefixAll(prefix string) (int, []string) {
	return t.Prefix(prefix).All(prefix)
}

// All returns all the valid words from that point onwards.
// Typically called from the result of [Prefix].
// if somehow both 0 and 255 are valid yet not much in between,
// the optimization of min,max range won't do much, but for
// normal words, it should help a lot.
// Returns the len of the longest common prefix.
// If the input is incomplete UTF-8 sequence, use AllBytes() instead.
func (t *Trie) All(prefix string) (int, []string) {
	return t.AllBytes([]byte(prefix))
}

func (t *Trie) AllBytes(prefix []byte) (int, []string) {
	if t == nil {
		return 0, nil
	}
	var res []string
	longest := len(prefix)
	numChildren := 0
	if t.valid {
		res = append(res, string(prefix))
		numChildren++
	}
	if t.leaf {
		return longest, res
	}
	l1 := len(prefix)
	newPrefix := make([]byte, l1+1)
	copy(newPrefix, prefix)
	for i := t.min; i <= t.max; i++ {
		if t.children[i] == nil {
			continue
		}
		numChildren++
		newPrefix[l1] = i
		l, additional := t.children[i].AllBytes(newPrefix)
		if l > longest {
			longest = l
		}
		res = append(res, additional...)
		if i == 255 {
			break // Exit the loop after processing last possible byte value 255, avoid infinite loop.
		}
	}
	if numChildren > 1 {
		longest = len(prefix)
	}
	return longest, res
}

/*
  A-B
  A-B-C

  [A] -> [B] children[C] = endMarker
*/

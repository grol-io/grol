package trie_test

import (
	"testing"

	"grol.io/grol/trie"
)

func TestTrie_InsertAndContains(t *testing.T) {
	trie := trie.NewTrie()

	// Insert "ABC" and check containment
	trie.Insert("ABC")
	if !trie.Contains("ABC") {
		t.Error("Expected to find 'ABC', but it was not found.")
	}
	if trie.Contains("AB") {
		t.Error("Expected 'AB' to be not found, but it was found.")
	}
	if trie.Contains("ABCD") {
		t.Error("Expected 'ABCD' to be not found, but it was found.")
	}
	p := trie.Prefix("ABC")
	if !p.IsLeaf() {
		t.Errorf("Expected to find 'ABC' as the shared leaf node but it isn't: %+v", p)
	}
	trie.Insert("AB2")
	p = trie.Prefix("ABC")
	if !p.IsLeaf() {
		t.Errorf("Expected to find 'ABC' still as the shared leaf node but it isn't: %+v", p)
	}
	p2 := trie.Prefix("AB2")
	if p2 != p {
		t.Errorf("Expected 'ABC' and 'AB2' to share the same leaf node but they don't: %#v != %#v", p, p2)
	}
	if trie.Contains("AB") {
		t.Error("Expected 'AB' to be not found, but it was found after adding 'AB2'.")
	}
	if !trie.Contains("AB2") {
		t.Error("Expected to find 'AB2', but it was not found.")
	}
	if !trie.Contains("ABC") {
		t.Error("Expected to find 'ABC', but it was not found after adding 'AB2'.")
	}
	// Insert "ABCD" and check both "ABC" and "ABCD"
	trie.Insert("ABCD")
	if !trie.Contains("ABC") {
		t.Error("Expected to find 'ABC', but it was not found after adding 'ABCD'.")
	}
	if !trie.Contains("ABCD") {
		t.Error("Expected to find 'ABCD', but it was not found.")
	}

	// Ensure no additional levels were created
	//	if len(trie.children['a'].children['b'].children['c'].children) != 1 {
	//		t.Error("Expected no additional levels after 'c' when adding 'ABC'.")
	//	}
}

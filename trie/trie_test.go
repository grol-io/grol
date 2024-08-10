package trie_test

import (
	"reflect"
	"testing"

	"grol.io/grol/trie"
)

func TestTrie(t *testing.T) {
	trie := trie.NewTrie()
	// more like a "don't crash when empty" test
	if trie.Contains("Foo") {
		t.Error("Expected 'Foo' to be not found, but it was found.")
	}
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

	all := trie.Prefix("X").All("Y")
	if len(all) != 0 {
		t.Errorf("Expected no results for 'X' but got: %v", all)
	}
	all = trie.Prefix("A").All("xy")
	expected := []string{"xyB2", "xyBC", "xyBCD"}
	if len(all) != len(expected) {
		t.Errorf("Expected %v for 'A' but got: %v", expected, all)
	}
	if !reflect.DeepEqual(all, expected) {
		t.Errorf("Expected %v for 'A' but got: %v", expected, all)
	}
	all = trie.Prefix("ABCD").All("z")
	if len(all) != 1 {
		t.Errorf("Expected one result for all of 'ABCD' but got: %v", all)
	}
	if all[0] != "z" {
		t.Errorf("Expected 'z' for 'ABCD' but got: %v", all)
	}
}

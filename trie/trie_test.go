package trie_test

import (
	"reflect"
	"testing"

	"fortio.org/sets"
	"grol.io/grol/trie"
)

func TestTrie(t *testing.T) {
	tr := trie.NewTrie()
	// more like a "don't crash when empty" test
	if tr.Contains("Foo") {
		t.Error("Expected 'Foo' to be not found, but it was found.")
	}
	// Insert "ABC" and check containment
	tr.Insert("ABC")
	if !tr.Contains("ABC") {
		t.Error("Expected to find 'ABC', but it was not found.")
	}
	if tr.Contains("AB") {
		t.Error("Expected 'AB' to be not found, but it was found.")
	}
	if tr.Contains("ABCD") {
		t.Error("Expected 'ABCD' to be not found, but it was found.")
	}
	p := tr.Prefix("ABC")
	if !p.IsLeaf() {
		t.Errorf("Expected to find 'ABC' as the shared leaf node but it isn't: %+v", p)
	}
	tr.Insert("AB2")
	p = tr.Prefix("ABC")
	if !p.IsLeaf() {
		t.Errorf("Expected to find 'ABC' still as the shared leaf node but it isn't: %+v", p)
	}
	p2 := tr.Prefix("AB2")
	if p2 != p {
		t.Errorf("Expected 'ABC' and 'AB2' to share the same leaf node but they don't: %#v != %#v", p, p2)
	}
	if tr.Contains("AB") {
		t.Error("Expected 'AB' to be not found, but it was found after adding 'AB2'.")
	}
	if !tr.Contains("AB2") {
		t.Error("Expected to find 'AB2', but it was not found.")
	}
	if !tr.Contains("ABC") {
		t.Error("Expected to find 'ABC', but it was not found after adding 'AB2'.")
	}
	// Insert "ABCD" and check both "ABC" and "ABCD"
	tr.Insert("ABCD")
	if !tr.Contains("ABC") {
		t.Error("Expected to find 'ABC', but it was not found after adding 'ABCD'.")
	}
	if !tr.Contains("ABCD") {
		t.Error("Expected to find 'ABCD', but it was not found.")
	}

	l, all := tr.Prefix("X").All("Y")
	if len(all) != 0 {
		t.Errorf("Expected no results for 'X' but got: %v", all)
	}
	if l != 0 {
		t.Errorf("Expected 0 for 'X' but got: %v", l)
	}
	_, all = tr.Prefix("A").All("xy")
	expected := []string{"xyB2", "xyBC", "xyBCD"}
	if len(all) != len(expected) {
		t.Errorf("Expected %v for 'A' but got: %v", expected, all)
	}
	if !reflect.DeepEqual(all, expected) {
		t.Errorf("Expected %v for 'A' but got: %v", expected, all)
	}
	prefix := "ABCD"
	l, all = tr.PrefixAll(prefix)
	if len(all) != 1 {
		t.Errorf("Expected one result for all of 'ABCD' but got: %v", all)
	}
	if all[0] != prefix {
		t.Errorf("Expected 'z' for 'ABCD' but got: %v", all)
	}
	if l != 4 {
		t.Errorf("Expected 4 for 'ABCD' but got: %v", l)
	}
	l, _ = tr.PrefixAll("")
	if l != 2 {
		t.Errorf("Expected 3 for common prefix (AB) but got: %v", l)
	}
	unicode := "😀"
	tr.Insert(unicode)
	if !tr.Contains(unicode) {
		t.Errorf("Expected to find %q, but it was not found.", unicode)
	}
	_, all = tr.PrefixAll("")
	expected = []string{"AB2", "ABC", "ABCD", unicode}
	if len(all) != len(expected) {
		t.Errorf("Expected %v for %q but got: %v", expected, unicode, all)
	}
	if !reflect.DeepEqual(all, expected) {
		t.Errorf("Expected %v for %q but got: %v", expected, unicode, all)
	}
}

func TestTrieAllWithMaxByte(t *testing.T) {
	tr := trie.NewTrie()

	// Insert words with byte values from 0 to 255
	for i := range 256 {
		b := byte(i)
		str := string([]byte{b})
		if len(str) != 1 {
			t.Errorf("Unexpected string length: %d %d %q", len(str), i, str)
		}
		tr.Insert(str)
	}

	// This should include all 256 single-byte strings
	_, results := tr.All("")

	if len(results) != 256 {
		t.Errorf("Expected 256 results, got %d", len(results))
	}

	// Check if all byte values are present
	byteSet := sets.New[byte]()
	for i, result := range results {
		if len(result) != 1 {
			t.Errorf("Unexpected result length for %d: %s %d", i, result, len(result))
		}
		byteSet.Add(result[0])
	}

	if len(byteSet) != 256 {
		t.Errorf("Expected 256 unique bytes, got %d", len(byteSet))
	}
}

package watcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReferenceCache(t *testing.T) {
	c := NewReferenceCache()

	// Test adding references
	c.Add("ns1", "mw1", []string{"key1", "key2"})

	// Verify keys are mapped to mw1's namespace
	assert.ElementsMatch(t, []string{"ns1"}, c.GetNamespaces("key1"))
	assert.ElementsMatch(t, []string{"ns1"}, c.GetNamespaces("key2"))
	assert.Empty(t, c.GetNamespaces("key3"))

	// Test adding another MW referencing key1
	c.Add("ns2", "mw2", []string{"key1", "key3"})
	assert.ElementsMatch(t, []string{"ns1", "ns2"}, c.GetNamespaces("key1"))
	assert.ElementsMatch(t, []string{"ns2"}, c.GetNamespaces("key3"))

	// Test updating mw1: remove key2, add key3
	c.Add("ns1", "mw1", []string{"key1", "key3"})
	assert.ElementsMatch(t, []string{"ns1", "ns2"}, c.GetNamespaces("key1"))
	assert.Empty(t, c.GetNamespaces("key2")) // key2 should no longer point to ns1
	assert.ElementsMatch(t, []string{"ns1", "ns2"}, c.GetNamespaces("key3"))

	// Test removing mw2
	c.Remove("ns2", "mw2")
	assert.ElementsMatch(t, []string{"ns1"}, c.GetNamespaces("key1"))
	assert.ElementsMatch(t, []string{"ns1"}, c.GetNamespaces("key3"))

	// Test removing mw1
	c.Remove("ns1", "mw1")
	assert.Empty(t, c.GetNamespaces("key1"))
	assert.Empty(t, c.GetNamespaces("key3"))
}

package utils

import (
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDefaultCIMap(t *testing.T) {
	var testMap = map[string]string{
		"a": "valA",
		"B": "valB",
	}

	m1 := NewStringCIMap(testMap)
	assert.Assert(t, m1.Has("a"))
	assert.Assert(t, m1.Has("A"))
	assert.Assert(t, m1.Has("b"))
	assert.Assert(t, m1.Has("B"))
	assert.Assert(t, !m1.Has("c"))
	assert.Equal(t, m1.Get("a"), "valA")
	assert.Equal(t, m1.Get("A"), "valA")
	assert.Equal(t, m1.Get("b"), "valB")
	assert.Equal(t, m1.Get("B"), "valB")
	assert.Equal(t, m1.Get("c"), "")

	a := m1.GetLowerCaseKeys()
	slices.Sort(a)
	assert.DeepEqual(t, a, []string{"a", "b"})

	a = m1.GetKeys()
	slices.Sort(a)
	assert.DeepEqual(t, a, []string{"B", "a"})

	var testValue = map[string][]string{
		// because of map-internal hashing we don't know which one comes first
		"a": {"vala", "vala2"},
		"A": {"valA"},
	}
	m2 := NewStringCIMapFromValues(testValue)
	assert.Equal(t, strings.ToLower(m2.Get("a")), "vala")
	assert.Equal(t, strings.ToLower(m2.Get("A")), "vala")
	a = m2.GetArray("A")
	slices.Sort(a)
	assert.DeepEqual(t, a, []string{"valA", "vala", "vala2"})

	assert.Equal(t, len(m2.GetArray("c")), 0)
	assert.Assert(t, m2.GetArray("c") != nil)
}

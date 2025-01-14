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

	a = m1.GetOriginalKeys()
	slices.Sort(a)
	assert.DeepEqual(t, a, []string{"B", "a"})

	b := m1.GetOriginalCaseMapOnlyFirst()
	for expectK, expectedV := range testMap {
		assert.Equal(t, b[expectK], expectedV)
	}
}

func TestMultiValueMap(t *testing.T) {
	var testValue = map[string][]string{
		// because of map-internal hashing we don't know which one comes first
		"a":   {"vala", "vala2"},
		"A":   {"valA"},
		"CaH": {"cah4"},
		"Cah": {"cah5", "cah6", "cah7"},
		"CAH": {"cah1", "cah2", "cah3"},
		"D":   {"vald"},
	}
	m2 := NewStringCIMapFromValues(testValue)
	// prefer the one with the exact case
	assert.Equal(t, m2.Get("a"), "vala")
	assert.Equal(t, m2.Get("A"), "valA")
	a := m2.GetValues("A")
	assert.DeepEqual(t, a, []string{"valA", "vala", "vala2"})

	assert.Equal(t, len(m2.GetValues("c")), 0)
	assert.Assert(t, m2.GetValues("c") != nil)

	assert.Assert(t, m2.Has("cah"))
	assert.Equal(t, m2.GetOriginalCase("cah"), "CAH")

	assert.Equal(t, m2.GetOrDefault("d", "NOT"), "vald")

	assert.Equal(t, m2.GetOrDefault("f", "YES"), "YES")

	cahl := m2.GetLowerCaseMapJoined()
	vJoined, ok := cahl["a"]
	assert.Assert(t, ok)
	assert.Equal(t, vJoined, strings.Join(a, ","))
	vJoined, ok = cahl["cah"]
	assert.Assert(t, ok)
	assert.Equal(t, vJoined, "cah1,cah2,cah3,cah4,cah5,cah6,cah7")
	vJoined, ok = cahl["d"]
	assert.Assert(t, ok)
	assert.Equal(t, vJoined, "vald")

	keys := m2.GetLowerCaseKeys()
	slices.Sort(keys)
	assert.DeepEqual(t, keys, []string{"a", "cah", "d"})

	assert.Assert(t, !m2.IsEmpty())
	m2.Append("a", "vala3")
	assert.Equal(t, m2.Get("a"), "vala")
	assert.Equal(t, m2.Get("A"), "valA")
	assert.DeepEqual(t, m2.GetValues("a"), []string{"valA", "vala", "vala2", "vala3"})

	m2.Set("a", []string{"vala4"})
	assert.DeepEqual(t, m2.GetValues("a"), []string{"vala4"})
}

func TestMixedInsertCIMap(t *testing.T) {
	var testMap = map[string]string{
		"a": "valA",
		"B": "valB",
		"b": "valb",
	}

	m1 := NewStringCIMap(testMap)

	// b or B will not appear in map with current impl
	b := m1.GetOriginalCaseMap()
	for expectK, expectedV := range testMap {
		found := false
		for _, v := range b[expectK] {
			if v == expectedV {
				found = true
			}
		}
		assert.Assert(t, found)
	}

	m1.Append("c", "valc")
	m1.Append("C", "Valc")
	m1.Append("B", "valB2")

	assert.DeepEqual(t, m1.GetValues("c"), []string{"Valc", "valc"})
	assert.DeepEqual(t, m1.GetValues("b"), []string{"valB", "valB2", "valb"})
}

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
	assert.Equal(t, m2.Get("a"), m2.Get("A"))
	a := m2.GetValues("A")
	slices.Sort(a)
	assert.DeepEqual(t, a, []string{"valA", "vala", "vala2"})

	assert.Equal(t, len(m2.GetValues("c")), 0)
	assert.Assert(t, m2.GetValues("c") != nil)

	assert.Assert(t, m2.Has("cah"))

	assert.Equal(t, m2.GetOrDefault("d", "NOT"), "vald")

	assert.Equal(t, m2.GetOrDefault("f", "YES"), "YES")

	assert.Equal(t, m2.Has("e"), false)
	assert.Equal(t, m2.Get("e"), "")

	cahl := m2.GetLowerCaseMapJoined()
	vJoined, ok := cahl["a"]
	assert.Assert(t, ok)
	assert.Equal(t, vJoined, strings.Join(a, ","))
	vJoined, ok = cahl["d"]
	assert.Assert(t, ok)
	assert.Equal(t, vJoined, "vald")

	keys := m2.GetLowerCaseKeys()
	slices.Sort(keys)
	assert.DeepEqual(t, keys, []string{"a", "cah", "d"})

	assert.Assert(t, !m2.IsEmpty())
	m2.Append("a", "vala3")
	assert.Equal(t, m2.Get("a"), m2.Get("A"))
	a = m2.GetValues("a")
	slices.Sort(a)
	assert.DeepEqual(t, a, []string{"valA", "vala", "vala2", "vala3"})

	m2.Set("a", []string{"vala4"})
	assert.DeepEqual(t, m2.GetValues("a"), []string{"vala4"})
}

func TestMixedInsertCIMap(t *testing.T) {
	var testMap = map[string]string{
		"a": "valA",
	}

	m1 := NewStringCIMap(testMap)

	m1.Append("c", "valc")
	m1.Append("C", "Valc")
	m1.Append("B", "valB2")
	m1.Append("b", "valb")

	assert.DeepEqual(t, m1.GetValues("c"), []string{"valc", "Valc"})
	assert.DeepEqual(t, m1.GetValues("b"), []string{"valB2", "valb"})

	b := m1.GetOriginalCaseMap()
	assert.DeepEqual(t, b["B"], []string{"valB2", "valb"})

	assert.Equal(t, m1.GetOriginalCase("b"), "B")

	m1.Set("b", []string{"valB3"})
	m1.Append("B", "valB4")
	assert.DeepEqual(t, m1.GetValues("B"), []string{"valB3", "valB4"})
	assert.Equal(t, m1.GetOriginalCase("b"), "b")

	assert.Assert(t, m1.ContainsValue("b", "valB3"))
	assert.Assert(t, m1.ContainsValue("B", "valB3"))
	assert.Assert(t, !m1.ContainsValue("b", "valB2"))
	assert.Assert(t, m1.GetOriginalCaseMapJoined()["b"] == "valB3,valB4")

	assert.Equal(t, m1.Get("b"), "valB3")
	assert.Equal(t, m1.Get("B"), "valB3")
	assert.Equal(t, m1.Get("c"), "valc")
}

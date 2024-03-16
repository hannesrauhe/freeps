package utils

import "strings"

// CIMap is a struct that can be used to pass arguments to a function
type CIMap[Val any] interface {
	// Has returns true if map contains a key
	Has(key string) bool
	// Get returns the first value stored for key or the default value if key is not in map
	Get(key string) Val
	// GetOrDefault returns the first value stored for key or the given default value if key is not in map
	GetOrDefault(key string, defaultVal Val) Val
	// GetArray returns all values stored for key
	GetArray(key string) []Val

	// GetLowerCaseKeys returns the stored keys in lower case
	GetLowerCaseKeys() []string
	// GetKeys returns the stored key in original case
	GetKeys() []string

	GetOriginalCase(key string) string // problematic -> what to do if key does not exist
	GetOriginalCaseMapOnlyFirst() map[string]Val
	GetLowerCaseMapOnlyFirst() map[string]Val
	GetOriginalCaseMap() map[string][]Val
	// Size() int
}

// CIMapImpl is a struct similar to url.Values that allows case-insensitive comparisons but stores the orignal case
// it can be built from url.Values or a map[string]Val and preserves the case of the keys but allows accessing them in a case-insensitive way
// when inserting keys with different cases, they will be combined into one key with the first case used
type CIMapImpl[Val any] struct {
	OriginalMap     map[string][]Val    // map with original case keys
	lowerKeyMapping map[string][]string // map from lower case key to original case (can be multiple cases)
	defaultValue    Val
}

var _ CIMap[string] = &CIMapImpl[string]{}

func appendToMultiMap[Val any](m map[string][]Val, k string, v Val) {
	_, exists := m[k]
	if exists {
		m[k] = append(m[k], v)
	} else {
		m[k] = []Val{v}
	}

}

// NewStringCIMap creates a new CIMap struct from the given map
func NewStringCIMap(args map[string]string) CIMap[string] {
	ret := &CIMapImpl[string]{
		OriginalMap:     make(map[string][]string),
		lowerKeyMapping: make(map[string][]string),
		defaultValue:    "",
	}
	for k, v := range args {
		ret.OriginalMap[k] = []string{v}
		lk := strings.ToLower(k)
		appendToMultiMap(ret.lowerKeyMapping, lk, k)
	}
	return ret
}

// NewStringCIMapFromValues creates a new FunctionArguments struct from the given url.Values
func NewStringCIMapFromValues(args map[string][]string) CIMap[string] {
	ret := &CIMapImpl[string]{
		OriginalMap:     make(map[string][]string),
		lowerKeyMapping: make(map[string][]string),
		defaultValue:    "",
	}
	for k, v := range args {
		ret.OriginalMap[k] = v
		lk := strings.ToLower(k)
		appendToMultiMap(ret.lowerKeyMapping, lk, k)
	}
	return ret
}

// Has returns true if the given key is present in any variant
func (fa *CIMapImpl[Val]) Has(key string) bool {
	if _, ok := fa.OriginalMap[key]; ok {
		return true
	}
	// key does not exist in this case, look for any other
	lk := strings.ToLower(key)
	_, ok := fa.lowerKeyMapping[lk]
	return ok
}

func (fa *CIMapImpl[Val]) getFirst(key string) (Val, bool) {
	if v, ok := fa.OriginalMap[key]; ok {
		return v[0], true
	}
	// key does not exist in this variant, look for any other
	lk := strings.ToLower(key)
	if keyList, ok := fa.lowerKeyMapping[lk]; ok {
		v, _ := fa.OriginalMap[keyList[0]]
		return v[0], true
	}
	return fa.defaultValue, false
}

// Get returns the first value for the given key
func (fa *CIMapImpl[Val]) Get(key string) Val {
	v, _ := fa.getFirst(key)
	return v
}

// GetOrDefault returns the first value for the given key
func (fa *CIMapImpl[Val]) GetOrDefault(key string, defaultVal Val) Val {
	v, ok := fa.getFirst(key)
	if ok {
		return v
	}
	return defaultVal
}

// GetArray returns all values for the given key
func (fa *CIMapImpl[Val]) GetArray(key string) []Val {
	ret := []Val{}
	lk := strings.ToLower(key)
	for _, ak := range fa.lowerKeyMapping[lk] {
		ret = append(ret, fa.OriginalMap[ak]...)
	}
	return ret
}

// GetLowerCaseKeys returns the stored keys in lower case
func (fa *CIMapImpl[Val]) GetLowerCaseKeys() []string {
	ret := []string{}
	for k := range fa.lowerKeyMapping {
		ret = append(ret, k)
	}
	return ret
}

// GetKeys returns the stored key in original case
func (fa *CIMapImpl[Val]) GetKeys() []string {
	ret := []string{}
	for oKey, _ := range fa.OriginalMap {
		ret = append(ret, oKey)
	}
	return ret
}

// GetOriginalCase returns the key in the correct variant (if multiple, whichever comes first)
func (fa *CIMapImpl[Val]) GetOriginalCase(key string) string {
	lk := strings.ToLower(key)
	for _, v := range fa.lowerKeyMapping[lk] {
		return v
	}
	return ""
}

// GetLowerCaseMap returns a map of all keys in lower case
func (fa *CIMapImpl[Val]) GetLowerCaseMap() map[string][]Val {
	ret := make(map[string][]Val)
	for key, vList := range fa.OriginalMap {
		lk := strings.ToLower(key)
		for _, v := range vList {
			appendToMultiMap(ret, lk, v)
		}
	}
	return ret
}

// GetLowerCaseMapOnlyFirst returns a map of all keys in lower case
func (fa *CIMapImpl[Val]) GetLowerCaseMapOnlyFirst() map[string]Val {
	ret := make(map[string]Val)
	for k, v := range fa.OriginalMap {
		lk := strings.ToLower(k)
		ret[lk] = v[0]
	}
	return ret
}

// GetOriginalCaseMap returns a map of all keys in the original case (this will contain only one case-variant if multiple key with different cases were inserted)
func (fa *CIMapImpl[Val]) GetOriginalCaseMap() map[string][]Val {
	return fa.OriginalMap
}

// GetOriginalCaseMap returns a map of all keys in the original case (this will contain only one case-variant if multiple key with different cases were inserted)
func (fa *CIMapImpl[Val]) GetOriginalCaseMapOnlyFirst() map[string]Val {
	ret := make(map[string]Val)
	for k, v := range fa.OriginalMap {
		ret[k] = v[0]
	}
	return ret
}

// // Size returns the number of keys
// func (fa *CIMapImpl[Val]) Size() int {
// 	return len(fa.OriginalMap)
// }

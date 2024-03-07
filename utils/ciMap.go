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

	GetOriginalCase(key string) string
	GetOriginalCaseMapOnlyFirst() map[string]Val
	GetLowerCaseMapOnlyFirst() map[string]Val
	GetOriginalCaseMap() map[string][]Val
	Size() int
}

// CIMapImpl is a struct similar to url.Values that allows case-insensitive comparisons but stores the orignal case
// it can be built from url.Values or a map[string]Val and preserves the case of the keys but allows accessing them in a case-insensitive way
// when inserting keys with different cases, they will be combined into one key with the first case used
type CIMapImpl[Val any] struct {
	LcMap        map[string][]Val  // map with lower case keys
	KeyMapping   map[string]string // map from lower case key to original case
	DefaultValue Val
}

var _ CIMap[string] = &CIMapImpl[string]{}

// NewStringCIMap creates a new CIMap struct from the given map
func NewStringCIMap(args map[string]string) CIMap[string] {
	ret := &CIMapImpl[string]{
		LcMap:        make(map[string][]string),
		KeyMapping:   make(map[string]string),
		DefaultValue: "",
	}
	for k, v := range args {
		ret.KeyMapping[strings.ToLower(k)] = k
		if _, ok := ret.LcMap[strings.ToLower(k)]; ok {
			ret.LcMap[strings.ToLower(k)] = append(ret.LcMap[strings.ToLower(k)], v)
		} else {
			ret.LcMap[strings.ToLower(k)] = []string{v}
		}
	}
	return ret
}

// NewStringCIMapFromValues creates a new FunctionArguments struct from the given url.Values
func NewStringCIMapFromValues(args map[string][]string) CIMap[string] {
	ret := &CIMapImpl[string]{
		LcMap:        make(map[string][]string),
		KeyMapping:   make(map[string]string),
		DefaultValue: "",
	}
	for k, v := range args {
		ret.KeyMapping[k] = strings.ToLower(k)
		if _, ok := ret.LcMap[strings.ToLower(k)]; ok {
			ret.LcMap[strings.ToLower(k)] = append(ret.LcMap[strings.ToLower(k)], v...)
		} else {
			ret.LcMap[strings.ToLower(k)] = v
		}
	}
	return ret
}

// NewStringCIMapFromValues creates a new FunctionArguments struct from the given url.Values
func NewCIMapFromValues[Val any](args map[string][]Val, defaultVal Val) CIMap[Val] {
	ret := &CIMapImpl[Val]{
		LcMap:        make(map[string][]Val),
		KeyMapping:   make(map[string]string),
		DefaultValue: defaultVal,
	}
	for k, v := range args {
		ret.KeyMapping[k] = strings.ToLower(k)
		if _, ok := ret.LcMap[strings.ToLower(k)]; ok {
			ret.LcMap[strings.ToLower(k)] = append(ret.LcMap[strings.ToLower(k)], v...)
		} else {
			ret.LcMap[strings.ToLower(k)] = v
		}
	}
	return ret
}

// Has returns true if the given key is present
func (fa *CIMapImpl[Val]) Has(key string) bool {
	_, ok := fa.LcMap[strings.ToLower(key)]
	return ok
}

// Get returns the first value for the given key
func (fa *CIMapImpl[Val]) Get(key string) Val {
	if v, ok := fa.LcMap[strings.ToLower(key)]; ok {
		return v[0]
	}
	return fa.DefaultValue
}

// GetOrDefault returns the first value for the given key
func (fa *CIMapImpl[Val]) GetOrDefault(key string, defaultVal Val) Val {
	if v, ok := fa.LcMap[strings.ToLower(key)]; ok {
		return v[0]
	}
	return defaultVal
}

// GetArray returns all values for the given key
func (fa *CIMapImpl[Val]) GetArray(key string) []Val {
	if v, ok := fa.LcMap[strings.ToLower(key)]; ok {
		return v
	}
	return []Val{}
}

// GetLowerCaseKeys returns the stored keys in lower case
func (fa *CIMapImpl[Val]) GetLowerCaseKeys() []string {
	ret := []string{}
	for k := range fa.LcMap {
		ret = append(ret, k)
	}
	return ret
}

// GetKeys returns the stored key in original case
func (fa *CIMapImpl[Val]) GetKeys() []string {
	ret := []string{}
	for _, oKey := range fa.KeyMapping {
		ret = append(ret, oKey)
	}
	return ret
}

// GetOriginalCase returns the key in the correct case
func (fa *CIMapImpl[Val]) GetOriginalCase(key string) string {
	if v, ok := fa.KeyMapping[strings.ToLower(key)]; ok {
		return v
	}
	return ""
}

// GetLowerCaseMap returns a map of all keys in lower case
func (fa *CIMapImpl[Val]) GetLowerCaseMap() map[string][]Val {
	ret := make(map[string][]Val)
	for k, v := range fa.LcMap {
		ret[k] = v
	}
	return ret
}

// GetLowerCaseMapOnlyFirst returns a map of all keys in lower case
func (fa *CIMapImpl[Val]) GetLowerCaseMapOnlyFirst() map[string]Val {
	ret := make(map[string]Val)
	for k, v := range fa.LcMap {
		ret[k] = v[0]
	}
	return ret
}

// GetOriginalCaseMap returns a map of all keys in the original case (this will contain only one case-variant if multiple key with different cases were inserted)
func (fa *CIMapImpl[Val]) GetOriginalCaseMap() map[string][]Val {
	ret := make(map[string][]Val)
	for k, v := range fa.KeyMapping {
		ret[k] = fa.LcMap[v]
	}
	return ret
}

// GetOriginalCaseMap returns a map of all keys in the original case (this will contain only one case-variant if multiple key with different cases were inserted)
func (fa *CIMapImpl[Val]) GetOriginalCaseMapOnlyFirst() map[string]Val {
	ret := make(map[string]Val)
	for k, v := range fa.KeyMapping {
		ret[k] = fa.LcMap[v][0]
	}
	return ret
}

// Size returns the number of keys
func (fa *CIMapImpl[Val]) Size() int {
	return len(fa.LcMap)
}

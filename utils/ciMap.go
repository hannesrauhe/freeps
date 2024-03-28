package utils

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// CIMap is a struct that can be used to pass arguments to a function
type CIMap[Val any] interface {
	// Append adds value to the array stored under key, it stores the original case
	Append(key string, value ...Val)

	// Has returns true if map contains a key
	Has(key string) bool
	// Get returns the first value stored for key or the default value if key is not in map
	Get(key string) Val
	// GetOrDefault returns the first value stored for key or the given default value if key is not in map
	GetOrDefault(key string, defaultVal Val) Val
	// GetValues returns all values stored for key
	GetValues(key string) []Val

	// GetLowerCaseKeys returns the stored keys in lower case
	GetLowerCaseKeys() []string
	// GetOriginalKeys returns the stored key in original case
	GetOriginalKeys() []string

	GetOriginalCase(key string) string // problematic -> what to do if key does not exist

	GetOriginalCaseMap() map[string][]Val
	GetOriginalCaseMapOnlyFirst() map[string]Val
	GetOriginalCaseMapJoined() map[string]Val
	GetLowerCaseMap() map[string][]Val
	GetLowerCaseMapOnlyFirst() map[string]Val
	GetLowerCaseMapJoined() map[string]Val

	IsEmpty() bool
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

func appendToMultiMap[Val any](m map[string][]Val, k string, v ...Val) {
	_, exists := m[k]
	if exists {
		m[k] = append(m[k], v...)
	} else {
		m[k] = v
	}

}

func joinMultiMap[Val any](m map[string][]Val) map[string]string {
	retMap := map[string]string{}
	for k, vList := range m {
		if len(vList) > 1 {
			retStr := ""
			for i, v := range vList {
				if i == 0 {
					retStr = fmt.Sprintf("%v", vList[0])
				} else {
					retStr = fmt.Sprintf("%v,%v", retStr, v)
				}
			}
			retMap[k] = retStr
		} else {
			retMap[k] = fmt.Sprintf("%v", vList[0])
		}
	}
	return retMap
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
	for _, kList := range ret.lowerKeyMapping {
		slices.Sort(kList)
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
	for _, kList := range ret.lowerKeyMapping {
		slices.Sort(kList)
	}
	return ret
}

func (fa *CIMapImpl[Val]) Append(k string, v ...Val) {
	appendToMultiMap(fa.OriginalMap, k, v...)
	lk := strings.ToLower(k)
	_, hasAlready := slices.BinarySearch(fa.lowerKeyMapping[lk], k)
	if !hasAlready {
		appendToMultiMap(fa.lowerKeyMapping, lk, k)
		slices.Sort(fa.lowerKeyMapping[lk])
	}
}

// MarshalJSON provides a custom marshaller with better readable time formats
func (fa *CIMapImpl[Val]) MarshalJSON() ([]byte, error) {
	return json.Marshal(fa.OriginalMap)
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

// GetValues returns all values for the given key
func (fa *CIMapImpl[Val]) GetValues(key string) []Val {
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
func (fa *CIMapImpl[Val]) GetOriginalKeys() []string {
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
	for lk, kList := range fa.lowerKeyMapping {
		for _, k := range kList {
			appendToMultiMap(ret, lk, fa.OriginalMap[k]...)
		}
	}
	return ret
}

// GetLowerCaseMapOnlyFirst returns a map of all keys in lower case with only a single value
func (fa *CIMapImpl[Val]) GetLowerCaseMapOnlyFirst() map[string]Val {
	ret := make(map[string]Val)
	for k, v := range fa.OriginalMap {
		lk := strings.ToLower(k)
		ret[lk] = v[0]
	}
	return ret
}

// GetLowerCaseMapJoined
func (fa *CIMapImpl[Val]) GetLowerCaseMapJoined() map[string]string {
	return joinMultiMap(fa.GetLowerCaseMap())
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

// GetOriginalCaseMapJoined
func (fa *CIMapImpl[Val]) GetOriginalCaseMapJoined() map[string]string {
	return joinMultiMap(fa.GetOriginalCaseMap())
}

// IsEmpty returns true if there are no keys in the map
func (fa *CIMapImpl[Val]) IsEmpty() bool {
	return len(fa.OriginalMap) == 0
}

package base

import "strings"

// FunctionArguments is a struct that can be used to pass arguments to a function
type FunctionArguments interface {
	Has(key string) bool
	Get(key string) string
	GetArray(key string) []string
	GetCombined(key string) string
	GetOriginalCase(key string) string
	GetLowerCaseMap() map[string]string
	GetOriginalCaseMap() map[string]string
	Size() int
}

// FunctionArgumentsImpl is a struct that can be used to pass arguments to a function
// it can be built from url.Values or a map[string]string and preserves the case of the keys but allows accessing them in a case-insensitive way
// when inserting keys with different cases, they will be combined into one key with the first case used
type FunctionArgumentsImpl struct {
	LcArgs     map[string][]string
	KeyMapping map[string]string
}

// NewFunctionArguments creates a new FunctionArguments struct from the given map
func NewFunctionArguments(args map[string]string) FunctionArguments {
	ret := &FunctionArgumentsImpl{
		LcArgs:     make(map[string][]string),
		KeyMapping: make(map[string]string),
	}
	for k, v := range args {
		ret.KeyMapping[strings.ToLower(k)] = k
		if _, ok := ret.LcArgs[strings.ToLower(k)]; ok {
			ret.LcArgs[strings.ToLower(k)] = []string{v}
		} else {
			ret.LcArgs[strings.ToLower(k)] = append(ret.LcArgs[strings.ToLower(k)], v)
		}
	}
	return ret
}

// NewFunctionArgumentsFromValues creates a new FunctionArguments struct from the given url.Values
func NewFunctionArgumentsFromValues(args map[string][]string) FunctionArguments {
	ret := &FunctionArgumentsImpl{
		LcArgs:     make(map[string][]string),
		KeyMapping: make(map[string]string),
	}
	for k, v := range args {
		ret.KeyMapping[k] = strings.ToLower(k)
		if _, ok := ret.LcArgs[strings.ToLower(k)]; ok {
			ret.LcArgs[strings.ToLower(k)] = v
		} else {
			ret.LcArgs[strings.ToLower(k)] = append(ret.LcArgs[strings.ToLower(k)], v...)
		}
	}
	return ret
}

// Has returns true if the given key is present
func (fa *FunctionArgumentsImpl) Has(key string) bool {
	_, ok := fa.LcArgs[strings.ToLower(key)]
	return ok
}

// Get returns the first value for the given key
func (fa *FunctionArgumentsImpl) Get(key string) string {
	if v, ok := fa.LcArgs[strings.ToLower(key)]; ok {
		return v[0]
	}
	return ""
}

// GetArray returns all values for the given key
func (fa *FunctionArgumentsImpl) GetArray(key string) []string {
	if v, ok := fa.LcArgs[strings.ToLower(key)]; ok {
		return v
	}
	return []string{}
}

// GetCombined returns all values for the given key combined into one string
func (fa *FunctionArgumentsImpl) GetCombined(key string) string {
	if v, ok := fa.LcArgs[strings.ToLower(key)]; ok {
		return strings.Join(v, ",")
	}
	return ""
}

// GetOriginalCase returns the key in the correct case
func (fa *FunctionArgumentsImpl) GetOriginalCase(key string) string {
	if v, ok := fa.KeyMapping[strings.ToLower(key)]; ok {
		return v
	}
	return ""
}

// GetLowerCaseMap returns a map of all keys in lower case
func (fa *FunctionArgumentsImpl) GetLowerCaseMap() map[string]string {
	ret := make(map[string]string)
	for k, v := range fa.LcArgs {
		ret[k] = strings.Join(v, ",")
	}
	return ret
}

// GetOriginalCaseMap returns a map of all keys in the original case (this wil contain only one case-variant if multiple key with different cases were inserted)
func (fa *FunctionArgumentsImpl) GetOriginalCaseMap() map[string]string {
	ret := make(map[string]string)
	for k, v := range fa.KeyMapping {
		ret[v] = strings.Join(fa.LcArgs[k], ",")
	}
	return ret
}

// Size returns the number of keys
func (fa *FunctionArgumentsImpl) Size() int {
	return len(fa.LcArgs)
}

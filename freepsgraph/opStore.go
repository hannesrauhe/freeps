package freepsgraph

import (
	"net/http"
	"sync"
)

type StoreNamespace struct {
	data   map[string]*OperatorIO
	nsLock sync.Mutex
}

type InMemoryStore struct {
	namespaces map[string]*StoreNamespace
	globalLock sync.Mutex
}

type OpStore struct {
	store InMemoryStore
}

var _ FreepsOperator = &OpStore{}

// NewOpStore creates a new store operator
func NewOpStore() *OpStore {
	return &OpStore{store: InMemoryStore{namespaces: map[string]*StoreNamespace{}}}
}

// Execute gets, sets or deletes a value from the store
func (o *OpStore) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	result := map[string]map[string]*OperatorIO{}
	ns, ok := args["namespace"]
	if !ok {
		return MakeOutputError(http.StatusBadRequest, "No namespace given")
	}
	nsStore := o.store.GetNamespace(ns)
	key, ok := args["key"]
	if fn != "getAll" && !ok {
		return MakeOutputError(http.StatusBadRequest, "No key given")
	}
	output, ok := args["output"]
	if !ok {
		// default is the complete tree
		output = "hierarchy"
	}

	switch fn {
	case "getAll":
		{
			result[ns] = nsStore.GetAllValues()
		}
	case "get":
		{
			io := nsStore.GetValue(key)
			result[ns] = map[string]*OperatorIO{key: io}
		}
	case "set":
		{
			nsStore.SetValue(key, input)
			result[ns] = map[string]*OperatorIO{key: input}
		}
	case "setSimpleValue":
		{
			val, ok := args["value"]
			if !ok {
				return MakeOutputError(http.StatusBadRequest, "No value given")
			}
			io := MakePlainOutput(val)
			nsStore.SetValue(key, io)
			result[ns] = map[string]*OperatorIO{key: io}
		}
	case "equals":
		{
			val, ok := args["value"]
			if !ok {
				val = input.GetString()
			}
			io := nsStore.GetValue(key)
			if io.GetString() != val {
				return MakeOutputError(http.StatusExpectationFailed, "Values do not match")				
			}
			result[ns] = map[string]*OperatorIO{key: io}
		}
	case "del":
		{
			nsStore.DeleteValue(key)
		}
	default:
		return MakeOutputError(http.StatusBadRequest, "Unknown function")
	}


	switch(output) {
	case "arguments":
		{
			flatresult := map[string]string{}
			for k,v := range result[key] {
				flatresult[k] = v.GetString()
			}
			return MakeObjectOutput(flatresult)
		}
	case "direct":
		{
			return result[ns][key]
		}
	case "bool":
		{
			return MakePlainOutput("true")
		}
	case "empty":
		{
			return MakeEmptyOutput()
		}
	case "hierarchy":
		{
			return MakeObjectOutput(result)
		}
	}
	return MakeOutputError(http.StatusBadRequest, "Unknown output type '%v'", output)
}

// GetFunctions returns the functions of this operator
func (o *OpStore) GetFunctions() []string {
	return []string{"get", "set", "del", "setSimpleValue", "equals", "getAll"}
}

// GetPossibleArgs returns the possible arguments for a function
func (o *OpStore) GetPossibleArgs(fn string) []string {
	switch fn {
	case "get":
		return []string{"namespace", "key", "output"}
	case "getAll":
		return []string{"namespace"}
	case "set":
		return []string{"namespace", "key", "output"}
	case "del":
		return []string{"namespace", "key"}
	case "setSimpleValue":
		return []string{"namespace", "key", "value", "output"}
	case "equals":
		return []string{"namespace", "key", "value", "output"}
	}
	return []string{}
}

// GetArgSuggestions returns suggestions for arguments
func (o *OpStore) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "namespace":
		{
			ns := map[string]string{}
			for _, n := range o.store.GetNamespaces() {
				ns[n] = n
			}
			return ns
		}
	case "key":
		{
			ns := otherArgs["namespace"]
			if ns == "" {
				return map[string]string{}
			}
			nsStore := o.store.GetNamespace(ns)
			keys := map[string]string{}
			for _, k := range nsStore.GetKeys() {
				keys[k] = k
			}
			return keys
		}
	case "value":
		{
			ns := otherArgs["namespace"]
			if ns == "" {
				return map[string]string{}
			}
			key, ok := otherArgs["key"]
			if !ok {
				return map[string]string{}
			}
			io := o.store.GetNamespace(ns).GetValue(key)
			return map[string]string{io.GetString(): io.GetString()}
		}
	case "output":
		{
			return map[string]string{"direct": "direct", "arguments/simple dict": "arguments", "hierarchy/complete tree": "hierarchy", "empty":"empty", "boolean value": "bool"}
		}
	}
	return map[string]string{}
}

// GetNamespace from the store, create if it does not exist
func (s *InMemoryStore) GetNamespace(ns string) *StoreNamespace {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	nsStore, ok := s.namespaces[ns]
	if !ok {
		nsStore = &StoreNamespace{data: map[string]*OperatorIO{}, nsLock: sync.Mutex{}}
		s.namespaces[ns] = nsStore
	}
	return nsStore
}

// GetNamespaces returns all namespaces
func (s *InMemoryStore) GetNamespaces() []string {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	ns := []string{}
	for n := range s.namespaces {
		ns = append(ns, n)
	}
	return ns
}

// GetValue from the StoreNamespace
func (s *StoreNamespace) GetValue(key string) *OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	io, ok := s.data[key]
	if !ok {
		return MakeOutputError(http.StatusNotFound, "Key not found")
	}
	return io
}

// SetValue in the StoreNamespace
func (s *StoreNamespace) SetValue(key string, io *OperatorIO) {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	s.data[key] = io
}

// DeleteValue from the StoreNamespace
func (s *StoreNamespace) DeleteValue(key string) {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	delete(s.data, key)
}

// GetKeys returns all keys in the StoreNamespace
func (s *StoreNamespace) GetKeys() []string {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	keys := []string{}
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// GetAllValues from the StoreNamespace
func (s *StoreNamespace) GetAllValues() map[string]*OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	copy := map[string]*OperatorIO{}
	for k, v := range s.data {
		copy[k] = v
	}
	return copy
}

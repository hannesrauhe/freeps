package freepsgraph

import (
	"net/http"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/utils"
)

type StoreNamespace struct {
	data       map[string]*OperatorIO
	timestamps map[string]time.Time
	nsLock     sync.Mutex
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

// GetName returns the name of the operator
func (o *OpStore) GetName() string {
	return "store"
}

// Execute gets, sets or deletes a value from the store
func (o *OpStore) Execute(ctx *utils.Context, fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	result := map[string]map[string]*OperatorIO{}
	ns, ok := args["namespace"]
	if !ok {
		return MakeOutputError(http.StatusBadRequest, "No namespace given")
	}
	nsStore := o.store.GetNamespace(ns)
	keyArgName := args["keyArgName"]
	if keyArgName == "" {
		keyArgName = "key"
	}
	key, ok := args[keyArgName]
	if fn != "getAll" && fn != "setAll" && !ok {
		return MakeOutputError(http.StatusBadRequest, "No key given")
	}
	// overwrite input and function to treat setSimpleValue like set
	if fn == "setSimpleValue" {
		valueArgName := args["valueArgName"]
		if valueArgName == "" {
			valueArgName = "value"
		}
		val, ok := args[valueArgName]
		if !ok {
			return MakeOutputError(http.StatusBadRequest, "No value given")
		}
		fn = "set"
		input = MakePlainOutput(val)
	}
	output, ok := args["output"]
	if !ok {
		// default is the complete tree
		output = "hierarchy"
	}

	switch fn {
	case "getAll":
		{
			key = ""
			output = "hierarchy"
			result[ns] = nsStore.GetAllValues()
		}
	case "setAll":
		{
			key = ""
			output = "empty"
			m := map[string]interface{}{}
			err := input.ParseJSON(&m)
			if err != nil {
				return MakeOutputError(http.StatusBadRequest, "Cannot parse input: %v", err)
			}
			for inputKey, inputValue := range m {
				nsStore.SetValue(inputKey, MakeObjectOutput(inputValue))
			}
		}
	case "get":
		{
			var io *OperatorIO
			maxAgeStr, maxAgeRequest := args["maxAge"]
			if maxAgeRequest {
				maxAge, err := time.ParseDuration(maxAgeStr)
				if err != nil {
					return MakeOutputError(http.StatusBadRequest, "Cannot parse maxAge \"%v\" because of error: \"%v\"", maxAgeStr, err)
				}
				io = nsStore.GetValueBeforeExpiration(key, maxAge)
			} else {
				io = nsStore.GetValue(key)
			}
			if io.IsError() {
				return io
			}
			result[ns] = map[string]*OperatorIO{key: io}
		}
	case "set":
		{
			maxAgeStr, maxAgeRequest := args["maxAge"]
			if maxAgeRequest {
				maxAge, err := time.ParseDuration(maxAgeStr)
				if err != nil {
					return MakeOutputError(http.StatusBadRequest, "Cannot parse maxAge \"%v\" because of error: \"%v\"", maxAgeStr, err)
				}
				io := nsStore.OverwriteValueIfOlder(key, input, maxAge)
				if io.IsError() {
					return io
				}
			}
			nsStore.SetValue(key, input)
			result[ns] = map[string]*OperatorIO{key: input}
		}
	case "compareAndSwap":
		{
			val, ok := args["value"]
			if !ok {
				return MakeOutputError(http.StatusBadRequest, "No expected value given")
			}
			io := nsStore.CompareAndSwap(key, val, input)
			if io.IsError() {
				return io
			}
			result[ns] = map[string]*OperatorIO{key: input}
		}
	case "equals":
		{
			val, ok := args["value"]
			if !ok {
				val = input.GetString()
			}
			io := nsStore.GetValue(key)
			if io.IsError() {
				return io
			}
			if io.GetString() != val {
				return MakeOutputError(http.StatusExpectationFailed, "Values do not match")
			}
			result[ns] = map[string]*OperatorIO{key: io}
		}
	case "del":
		{
			nsStore.DeleteValue(key)
			return MakeEmptyOutput()
		}
	default:
		return MakeOutputError(http.StatusBadRequest, "Unknown function")
	}

	switch output {
	case "arguments":
		{
			flatresult := map[string]string{}
			for k, v := range result[ns] {
				if key == "" || key == k {
					flatresult[k] = v.GetString()
				}
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
	return []string{"get", "set", "del", "setSimpleValue", "equals", "getAll", "setAll", "compareAndSwap"}
}

// GetPossibleArgs returns the possible arguments for a function
func (o *OpStore) GetPossibleArgs(fn string) []string {
	switch fn {
	case "get":
		return []string{"namespace", "keyArgName", "key", "output", "maxAge"}
	case "getAll":
		return []string{"namespace"}
	case "setAll":
		return []string{"namespace"}
	case "set":
		return []string{"namespace", "keyArgName", "key", "output", "maxAge"}
	case "compareAndSwap":
		return []string{"namespace", "keyArgName", "key", "value", "output", "maxAge"}
	case "del":
		return []string{"namespace", "keyArgName", "key"}
	case "setSimpleValue":
		return []string{"namespace", "keyArgName", "key", "value", "output", "maxAge", "valueArgName"}
	case "equals":
		return []string{"namespace", "keyArgName", "key", "value", "output"}
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
			return map[string]string{"direct": "direct", "arguments/simple dict": "arguments", "hierarchy/complete tree": "hierarchy", "empty": "empty", "boolean value": "bool"}
		}
	case "maxAge":
		{
			return map[string]string{"1s": "1s", "10s": "10s", "100s": "100s"}
		}
	case "keyArgName":
		{
			return map[string]string{"key (default)": "key", "topic": "topic"}
		}
	case "valueArgName":
		{
			return map[string]string{"value (default)": "value"}
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
		nsStore = &StoreNamespace{data: map[string]*OperatorIO{}, timestamps: map[string]time.Time{}, nsLock: sync.Mutex{}}
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

// GetValueBeforeExpiration gets the value from the StoreNamespace, but returns error if older than maxAge
func (s *StoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) *OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	io, ok := s.data[key]
	if !ok {
		return MakeOutputError(http.StatusNotFound, "Key not found")
	}
	ts, ok := s.timestamps[key]
	if !ok {
		return MakeOutputError(http.StatusInternalServerError, "no timestamp for key")
	}
	if ts.Add(maxAge).Before(time.Now()) {
		return MakeOutputError(http.StatusGone, "key is older than %v", maxAge)
	}
	return io
}

func (s *StoreNamespace) setValueUnlocked(key string, newValue *OperatorIO) *OperatorIO {
	s.data[key] = newValue
	s.timestamps[key] = time.Now()
	return MakeEmptyOutput()
}

// SetValue in the StoreNamespace
func (s *StoreNamespace) SetValue(key string, io *OperatorIO) {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	s.setValueUnlocked(key, io)
}

// CompareAndSwap sets the value if the string representation of the already stored value is as expected
func (s *StoreNamespace) CompareAndSwap(key string, expected string, newValue *OperatorIO) *OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	oldV, exists := s.data[key]
	if !exists {
		return MakeOutputError(http.StatusNotFound, "key does not exist yet")
	}
	if oldV == nil || oldV.GetString() != expected {
		return MakeOutputError(http.StatusConflict, "old value is different from expectation")
	}
	return s.setValueUnlocked(key, newValue)
}

// OverwriteValueIfOlder sets the value only if the key does not exist or has been written before maxAge
func (s *StoreNamespace) OverwriteValueIfOlder(key string, io *OperatorIO, maxAge time.Duration) *OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	n := time.Now()
	ts, keyExists := s.timestamps[key]
	if keyExists && ts.Add(maxAge).After(n) {
		return MakeOutputError(http.StatusConflict, "%v already exists and is only %v old", key, n.Sub(ts))
	}
	return s.setValueUnlocked(key, io)
}

// DeleteValue from the StoreNamespace
func (s *StoreNamespace) DeleteValue(key string) {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	delete(s.data, key)
	delete(s.timestamps, key)
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

// Shutdown (noOp)
func (o *OpStore) Shutdown(ctx *utils.Context) {
}

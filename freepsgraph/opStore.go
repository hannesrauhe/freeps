package freepsgraph

import (
	"net/http"
)

type StoreNamespaces struct {
	inMemory map[string]*OperatorIO
}

type OpStore struct {
	namespaces map[string]*StoreNamespaces
}

var _ FreepsOperator = &OpStore{}

// NewOpStore creates a new store operator
func NewOpStore() *OpStore {
	defaultStore := &StoreNamespaces{inMemory: map[string]*OperatorIO{}}
	return &OpStore{namespaces: map[string]*StoreNamespaces{"default": defaultStore}}
}

// Execute gets, sets or deletes a value from the store
func (o *OpStore) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	ns, ok := args["namespace"]
	if !ok {
		return MakeOutputError(http.StatusBadRequest, "No namespace given")
	}
	if fn == "getAll" {
		nsStore, ok := o.namespaces[ns]
		if !ok {
			return MakeOutputError(http.StatusBadRequest, "Namespace not found")
		}
		return MakeObjectOutput(nsStore.inMemory)
	}
	key, ok := args["key"]
	if !ok {
		return MakeOutputError(http.StatusBadRequest, "No key given")
	}

	switch fn {
	case "get":
		{
			nsStore, ok := o.namespaces[ns]
			if !ok {
				return MakeOutputError(http.StatusBadRequest, "Namespace not found")
			}
			io, ok := nsStore.inMemory[key]
			if !ok {
				return MakeOutputError(http.StatusBadRequest, "Key not found")
			}
			output, ok := args["output"]
			if !ok || output == "direct" {
				return io
			}
			return MakeObjectOutput(map[string]string{key: io.GetString()})
		}
	case "set":
		{
			nsStore, ok := o.namespaces[ns]
			if !ok {
				nsStore = &StoreNamespaces{inMemory: map[string]*OperatorIO{}}
				o.namespaces[ns] = nsStore
			}
			nsStore.inMemory[key] = input
			return MakeEmptyOutput()
		}
	case "setSimpleValue":
		{
			val, ok := args["value"]
			if !ok {
				return MakeOutputError(http.StatusBadRequest, "No value given")
			}
			nsStore, ok := o.namespaces[ns]
			if !ok {
				nsStore = &StoreNamespaces{inMemory: map[string]*OperatorIO{}}
				o.namespaces[ns] = nsStore
			}
			nsStore.inMemory[key] = MakePlainOutput(val)
			return MakeEmptyOutput()
		}
	case "equals":
		{
			val, ok := args["value"]
			if !ok {
				return MakeOutputError(http.StatusBadRequest, "No value given")
			}
			nsStore, ok := o.namespaces[ns]
			if !ok {
				nsStore = &StoreNamespaces{inMemory: map[string]*OperatorIO{}}
				o.namespaces[ns] = nsStore
			}
			io, ok := nsStore.inMemory[key]
			if !ok {
				return MakeOutputError(http.StatusBadRequest, "Key not found")
			}
			if io.GetString() == val {
				return MakePlainOutput("true")
			}
			return MakeOutputError(http.StatusExpectationFailed, "Values do not match")
		}
	case "del":
		{
			nsStore, ok := o.namespaces[ns]
			if !ok {
				return MakeOutputError(http.StatusBadRequest, "Namespace not found")
			}
			delete(nsStore.inMemory, key)
			return MakeEmptyOutput()
		}
	}

	return MakeOutputError(http.StatusBadRequest, "Unknown function")
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
		return []string{"namespace", "key"}
	case "del":
		return []string{"namespace", "key"}
	case "setSimpleValue":
		return []string{"namespace", "key", "value"}
	case "equals":
		return []string{"namespace", "key", "value"}
	}
	return []string{}
}

// GetArgSuggestions returns suggestions for arguments
func (o *OpStore) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "namespace":
		{
			ns := map[string]string{}
			for n := range o.namespaces {
				ns[n] = n
			}
			return ns
		}
	case "key":
		{
			ns, ok := otherArgs["namespace"]
			if !ok {
				return map[string]string{}
			}
			nsStore, ok := o.namespaces[ns]
			if !ok {
				return map[string]string{}
			}
			keys := map[string]string{}
			for k := range nsStore.inMemory {
				keys[k] = k
			}
			return keys
		}
	case "value":
		{
			ns, ok := otherArgs["namespace"]
			if !ok {
				return map[string]string{}
			}
			nsStore, ok := o.namespaces[ns]
			if !ok {
				return map[string]string{}
			}
			key, ok := otherArgs["key"]
			if !ok {
				return map[string]string{}
			}
			io, ok := nsStore.inMemory[key]
			if !ok {
				return map[string]string{}
			}
			return map[string]string{io.GetString(): io.GetString()}
		}
	case "output":
		{
			return map[string]string{"direct": "direct", "arguments": "arguments"}
		}
	}
	return map[string]string{}
}

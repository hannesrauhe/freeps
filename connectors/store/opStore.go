package freepsstore

import (
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpStore struct {
}

var _ freepsgraph.FreepsOperator = &OpStore{}

// NewOpStore creates a new store operator
func NewOpStore() *OpStore {
	store.namespaces = map[string]*StoreNamespace{}
	return &OpStore{}
}

// GetName returns the name of the operator
func (o *OpStore) GetName() string {
	return "store"
}

// Execute gets, sets or deletes a value from the store
func (o *OpStore) Execute(ctx *utils.Context, fn string, args map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	result := map[string]map[string]*freepsgraph.OperatorIO{}
	ns, ok := args["namespace"]
	if !ok {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "No namespace given")
	}
	nsStore := store.GetNamespace(ns)
	keyArgName := args["keyArgName"]
	if keyArgName == "" {
		keyArgName = "key"
	}
	key, ok := args[keyArgName]
	if fn != "getAll" && fn != "setAll" && fn != "deleteOlder" && !ok {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "No key given")
	}
	// overwrite input and function to treat setSimpleValue like set
	if fn == "setSimpleValue" {
		valueArgName := args["valueArgName"]
		if valueArgName == "" {
			valueArgName = "value"
		}
		val, ok := args[valueArgName]
		if !ok {
			return freepsgraph.MakeOutputError(http.StatusBadRequest, "No value given")
		}
		fn = "set"
		input = freepsgraph.MakePlainOutput(val)
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
			maxAgeStr, maxAgeRequest := args["maxAge"]
			if maxAgeRequest {
				maxAge, err := time.ParseDuration(maxAgeStr)
				if err != nil {
					return freepsgraph.MakeOutputError(http.StatusBadRequest, "Cannot parse maxAge \"%v\" because of error: \"%v\"", maxAgeStr, err)
				}
				result[ns] = nsStore.GetAllValuesBeforeExpiration(maxAge)
			} else {
				result[ns] = nsStore.GetAllValues()
			}
		}
	case "setAll":
		{
			key = ""
			output = "empty"
			m := map[string]interface{}{}
			err := input.ParseJSON(&m)
			if err != nil {
				return freepsgraph.MakeOutputError(http.StatusBadRequest, "Cannot parse input: %v", err)
			}
			for inputKey, inputValue := range m {
				nsStore.SetValue(inputKey, freepsgraph.MakeObjectOutput(inputValue))
			}
		}
	case "get", "equals":
		{
			var io *freepsgraph.OperatorIO
			maxAgeStr, maxAgeRequest := args["maxAge"]
			if maxAgeRequest {
				maxAge, err := time.ParseDuration(maxAgeStr)
				if err != nil {
					return freepsgraph.MakeOutputError(http.StatusBadRequest, "Cannot parse maxAge \"%v\" because of error: \"%v\"", maxAgeStr, err)
				}
				io = nsStore.GetValueBeforeExpiration(key, maxAge)
			} else {
				io = nsStore.GetValue(key)
			}
			if io.IsError() {
				return io
			}

			if fn == "equals" {
				val, ok := args["value"]
				if !ok {
					val = input.GetString()
				}
				if io.GetString() != val {
					return freepsgraph.MakeOutputError(http.StatusExpectationFailed, "Values do not match")
				}
			}

			result[ns] = map[string]*freepsgraph.OperatorIO{key: io}
		}
	case "set":
		{
			maxAgeStr, maxAgeRequest := args["maxAge"]
			if maxAgeRequest {
				maxAge, err := time.ParseDuration(maxAgeStr)
				if err != nil {
					return freepsgraph.MakeOutputError(http.StatusBadRequest, "Cannot parse maxAge \"%v\" because of error: \"%v\"", maxAgeStr, err)
				}
				io := nsStore.OverwriteValueIfOlder(key, input, maxAge)
				if io.IsError() {
					return io
				}
			}
			nsStore.SetValue(key, input)
			result[ns] = map[string]*freepsgraph.OperatorIO{key: input}
		}
	case "compareAndSwap":
		{
			val, ok := args["value"]
			if !ok {
				return freepsgraph.MakeOutputError(http.StatusBadRequest, "No expected value given")
			}
			io := nsStore.CompareAndSwap(key, val, input)
			if io.IsError() {
				return io
			}
			result[ns] = map[string]*freepsgraph.OperatorIO{key: input}
		}
	case "del":
		{
			nsStore.DeleteValue(key)
			return freepsgraph.MakeEmptyOutput()
		}
	case "deleteOlder":
		{
			maxAgeStr, maxAgeRequest := args["maxAge"]
			if !maxAgeRequest {
				return freepsgraph.MakeOutputError(http.StatusBadRequest, "No maxAge given")
			}
			maxAge, err := time.ParseDuration(maxAgeStr)
			if err != nil {
				return freepsgraph.MakeOutputError(http.StatusBadRequest, "Cannot parse maxAge \"%v\" because of error: \"%v\"", maxAgeStr, err)
			}
			return freepsgraph.MakePlainOutput("Deleted %v records", nsStore.DeleteOlder(maxAge))
		}
	default:
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Unknown function")
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
			return freepsgraph.MakeObjectOutput(flatresult)
		}
	case "direct":
		{
			return result[ns][key]
		}
	case "bool":
		{
			return freepsgraph.MakePlainOutput("true")
		}
	case "empty":
		{
			return freepsgraph.MakeEmptyOutput()
		}
	case "hierarchy":
		{
			return freepsgraph.MakeObjectOutput(result)
		}
	}
	return freepsgraph.MakeOutputError(http.StatusBadRequest, "Unknown output type '%v'", output)
}

// GetFunctions returns the functions of this operator
func (o *OpStore) GetFunctions() []string {
	return []string{"get", "set", "del", "setSimpleValue", "equals", "getAll", "setAll", "compareAndSwap", "deleteOlder"}
}

// GetPossibleArgs returns the possible arguments for a function
func (o *OpStore) GetPossibleArgs(fn string) []string {
	switch fn {
	case "get":
		return []string{"namespace", "keyArgName", "key", "output", "maxAge"}
	case "getAll":
		return []string{"namespace", "maxAge"}
	case "deleteOlder":
		return []string{"namespace", "maxAge"}
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
		return []string{"namespace", "keyArgName", "key", "value", "output", "maxAge"}
	}
	return []string{}
}

// GetArgSuggestions returns suggestions for arguments
func (o *OpStore) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "namespace":
		{
			ns := map[string]string{}
			for _, n := range store.GetNamespaces() {
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
			nsStore := store.GetNamespace(ns)
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
			io := store.GetNamespace(ns).GetValue(key)
			return map[string]string{io.GetString(): io.GetString()}
		}
	case "output":
		{
			return map[string]string{"direct": "direct", "arguments/simple dict": "arguments", "hierarchy/complete tree": "hierarchy", "empty": "empty", "boolean value": "bool"}
		}
	case "maxAge":
		{
			return utils.GetDurationMap()
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

// Shutdown (noOp)
func (o *OpStore) Shutdown(ctx *utils.Context) {
}

package freepsstore

import (
	"math"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpStore struct {
	CR *utils.ConfigReader
	GE *freepsgraph.GraphEngine
}

var _ base.FreepsOperatorWithConfig = &OpStore{}
var _ base.FreepsOperatorWithDynamicFunctions = &OpStore{}

// GetDefaultConfig returns the default config for the http connector
func (o *OpStore) GetDefaultConfig() interface{} {
	return &StoreConfig{Namespaces: getDefaultNamespaces(), PostgresConnStr: "", MaxErrorLogSize: 1000}
}

// InitCopyOfOperator creates a copy of the operator
func (o *OpStore) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	store.namespaces = map[string]StoreNamespace{}
	store.config = config.(*StoreConfig)
	if store.config.PostgresConnStr != "" {
		err := store.initPostgres()
		if err != nil {
			ctx.GetLogger().Fatal(err)
		}
	}

	return &OpStore{CR: o.CR, GE: o.GE}, nil
}

// ExecuteDynamic is a single spaghetti - needs cleanup ... moving to opStoreV2.go
func (o *OpStore) ExecuteDynamic(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	result := map[string]map[string]*base.OperatorIO{}

	multiNs := fa.GetArray("namespace")
	if len(multiNs) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "No namespace given")
	}
	if len(multiNs) > 1 {
		if fn != "getall" {
			return base.MakeOutputError(http.StatusBadRequest, "Expected a single Namespace")
		}
		for _, ns := range multiNs {
			ns = utils.StringToIdentifier(ns)
			result[ns] = store.GetNamespaceNoError(ns).GetAllValues(0)
		}
		return base.MakeObjectOutput(result)
	}
	ns := utils.StringToIdentifier(multiNs[0])

	nsStore := store.GetNamespaceNoError(ns)
	keyArgName := fa.GetOrDefault("keyArgName", "key")
	if fn != "getall" && fn != "setall" && fn != "deleteolder" && fn != "search" && !fa.Has(keyArgName) {
		return base.MakeOutputError(http.StatusBadRequest, "Expected an argument called %v", keyArgName)
	}
	key := fa.Get(keyArgName)

	// default output shows the complete tree
	output := fa.GetOrDefault("output", "hierarchy")

	var err error
	maxAge := time.Duration(math.MaxInt64)
	minAge := time.Duration(0)
	maxAgeRequest := false
	maxAgeStr := fa.Get("maxAge")
	if maxAgeStr != "" {
		maxAgeRequest = true
		maxAge, err = time.ParseDuration(maxAgeStr)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Cannot parse maxAge \"%v\" because of error: \"%v\"", maxAgeStr, err)
		}
	}
	minAgeStr := fa.Get("minAge")
	if minAgeStr != "" {
		minAge, err = time.ParseDuration(minAgeStr)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Cannot parse minAge \"%v\" because of error: \"%v\"", minAgeStr, err)
		}
	}

	switch fn {
	case "getall":
		{
			r := nsStore.GetSearchResultWithMetadata(key, fa.Get("value"), fa.Get("modifiedBy"), minAge, maxAge)
			result[ns] = map[string]*base.OperatorIO{}
			for k, v := range r {
				result[ns][k] = v.data
			}
		}
	case "setall":
		{
			key = ""
			output = "empty"
			m := map[string]interface{}{}
			err := input.ParseJSON(&m)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, "Cannot parse input: %v", err)
			}
			nsStore.SetAll(m, ctx.GetID())
		}
	case "deleteolder":
		{
			if !maxAgeRequest {
				return base.MakeOutputError(http.StatusBadRequest, "No maxAge given")
			}
			return base.MakeSprintfOutput("Deleted %v records", nsStore.DeleteOlder(maxAge))
		}
	default:
		return base.MakeOutputError(http.StatusBadRequest, "Unknown function")
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
			return base.MakeObjectOutput(flatresult)
		}
	case "flat":
		{
			flatresult := map[string]interface{}{}
			for k, v := range result[ns] {
				if key == "" || key == k {
					flatresult[k] = v.Output
				}
			}
			return base.MakeObjectOutput(flatresult)
		}
	case "direct":
		{
			return result[ns][key]
		}
	case "bool":
		{
			return base.MakePlainOutput("true")
		}
	case "empty":
		{
			return base.MakeEmptyOutput()
		}
	case "hierarchy":
		{
			return base.MakeObjectOutput(result)
		}
	}
	return base.MakeOutputError(http.StatusBadRequest, "Unknown output type '%v'", output)
}

// GetDynamicFunctions returns the functions of this operator
func (o *OpStore) GetDynamicFunctions() []string {
	res := []string{"get", "getNamespaces", "set", "del", "setSimpleValue", "equals", "getAll", "setAll", "compareAndSwap", "deleteOlder", "search"}
	return res
}

// GetDynamicPossibleArgs returns the possible arguments for a function
func (o *OpStore) GetDynamicPossibleArgs(fn string) []string {
	switch fn {
	case "search":
		return []string{"namespace", "key", "value", "modifiedBy", "minAge", "maxAge"}
	case "get":
		return []string{"namespace", "keyArgName", "key", "output", "maxAge", "defaultValue"}
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

// GetDynamicArgSuggestions returns suggestions for arguments
func (o *OpStore) GetDynamicArgSuggestions(fn string, arg string, dynArgs base.FunctionArguments) map[string]string {
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
			ns := dynArgs.Get("namespace")
			if ns == "" {
				return map[string]string{}
			}
			nsStore := store.GetNamespaceNoError(ns)
			keys := map[string]string{}
			for _, k := range nsStore.GetKeys() {
				keys[k] = k
			}
			return keys
		}
	case "value":
		{
			ns := dynArgs.Get("namespace")
			if ns == "" {
				return map[string]string{}
			}
			if !dynArgs.Has("key") {
				return map[string]string{}
			}
			key := dynArgs.Get("key")
			io := store.GetNamespaceNoError(ns).GetValue(key)
			return map[string]string{io.GetData().GetString(): io.GetData().GetString()}
		}
	case "output":
		{
			return map[string]string{"direct": "direct", "arguments/string dict": "arguments", "hierarchy/complete tree": "hierarchy", "empty": "empty", "boolean value": "bool", "flat/simple dict": "flat"}
		}
	case "minAge":
		{
			return utils.GetDurationMap()
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

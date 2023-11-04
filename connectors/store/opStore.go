package freepsstore

import (
	"math"
	"net/http"
	"os"
	"strings"
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
	// get the hostname of this computer
	hostname, err := os.Hostname()
	if err != nil {
		panic("could not get hostname")
	}
	return &StoreConfig{PostgresConnStr: "", PostgresSchema: "freeps_" + hostname, ExecutionLogInPostgres: true, ExecutionLogName: "_execution_log", GraphInfoName: "_graph_info", ErrorLogName: "_error_log", OperatorInfoName: "_operator_info", MaxErrorLogSize: 1000}
}

// InitCopyOfOperator creates a copy of the operator
func (o *OpStore) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	store.namespaces = map[string]StoreNamespace{}
	store.config = config.(*StoreConfig)
	if store.config.PostgresConnStr != "" {
		err := store.initPostgresStores()
		if err != nil {
			ctx.GetLogger().Fatal(err)
		}
	}
	fns, err := newFileStoreNamespace()
	if err != nil {
		ctx.GetLogger().Fatal(err)
	}
	store.namespaces["_files"] = fns

	return &OpStore{CR: o.CR, GE: o.GE}, err
}

// ExecuteDynamic is a single spaghetti - needs cleanup ... moving to opStoreV2.go
func (o *OpStore) ExecuteDynamic(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	args := fa.GetOriginalCaseMap()
	result := map[string]map[string]*base.OperatorIO{}
	ns, ok := args["namespace"]
	if !ok {
		return base.MakeOutputError(http.StatusBadRequest, "No namespace given")
	}
	multiNs := strings.Split(ns, ",")
	if len(multiNs) > 1 && fn == "getall" {
		for _, ns := range multiNs {
			ns = utils.StringToIdentifier(ns)
			result[ns] = store.GetNamespace(ns).GetAllValues(0)
		}
		return base.MakeObjectOutput(result)
	}
	ns = utils.StringToIdentifier(ns)

	if fn == "createpostgresnamespace" {
		err := store.createPostgresNamespace(ns)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		return base.MakePlainOutput("Namespace %v created", ns)
	}
	nsStore := store.GetNamespace(ns)
	keyArgName := args["keyArgName"]
	if keyArgName == "" {
		keyArgName = "key"
	}
	key, ok := args[keyArgName]
	if fn != "getall" && fn != "setall" && fn != "deleteolder" && fn != "search" && !ok {
		return base.MakeOutputError(http.StatusBadRequest, "No key given")
	}

	output, ok := args["output"]
	if !ok {
		// default is the complete tree
		output = "hierarchy"
	}

	var err error
	maxAge := time.Duration(math.MaxInt64)
	minAge := time.Duration(0)
	maxAgeRequest := false
	maxAgeStr := args["maxAge"]
	if maxAgeStr != "" {
		maxAgeRequest = true
		maxAge, err = time.ParseDuration(maxAgeStr)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Cannot parse maxAge \"%v\" because of error: \"%v\"", maxAgeStr, err)
		}
	}
	minAgeStr := args["minAge"]
	if minAgeStr != "" {
		minAge, err = time.ParseDuration(minAgeStr)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Cannot parse minAge \"%v\" because of error: \"%v\"", minAgeStr, err)
		}
	}

	switch fn {
	case "getall":
		{
			r := nsStore.GetSearchResultWithMetadata(key, args["value"], args["modifiedBy"], minAge, maxAge)
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
			return base.MakePlainOutput("Deleted %v records", nsStore.DeleteOlder(maxAge))
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
	if db == nil {
		return res
	}
	return append(res, "createPostgresNamespace")
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
	case "createPostgresNamespace":
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
func (o *OpStore) GetDynamicArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
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

package freepsstore

import (
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpStore struct {
	cr *utils.ConfigReader
	GE *freepsgraph.GraphEngine
}

var _ base.FreepsBaseOperator = &OpStore{}

// NewOpStore creates a new store operator and re-initializes the store
func NewOpStore(cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) *OpStore {
	sc := getDefaultConfig()
	err := cr.ReadSectionWithDefaults("store", &sc)
	if err != nil {
		logrus.Fatal(err)
	}

	store.namespaces = map[string]StoreNamespace{}
	store.config = &sc
	if sc.PostgresConnStr != "" {
		err = store.initPostgresStores()
		if err != nil {
			logrus.Fatal(err)
		}
	}
	fns, err := newFileStoreNamespace()
	if err != nil {
		logrus.Fatal(err)
	}
	store.namespaces["_files"] = fns

	cr.WriteBackConfigIfChanged()
	if err != nil {
		logrus.Print(err)
	}
	return &OpStore{cr: cr, GE: ge}
}

// GetName returns the name of the operator
func (o *OpStore) GetName() string {
	return "store"
}

// Execute everything in a single spaghetti - needs cleanup
func (o *OpStore) Execute(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	if fn == "getNamespaces" {
		return base.MakeObjectOutput(store.GetNamespaces())
	}
	result := map[string]map[string]*base.OperatorIO{}
	ns, ok := args["namespace"]
	if !ok {
		return base.MakeOutputError(http.StatusBadRequest, "No namespace given")
	}
	multiNs := strings.Split(ns, ",")
	if len(multiNs) > 1 && fn == "getAll" {
		for _, ns := range multiNs {
			ns = utils.StringToIdentifier(ns)
			result[ns] = store.GetNamespace(ns).GetAllValues(0)
		}
		return base.MakeObjectOutput(result)
	}
	ns = utils.StringToIdentifier(ns)

	if fn == "createPostgresNamespace" {
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
	if fn != "getAll" && fn != "setAll" && fn != "deleteOlder" && fn != "search" && !ok {
		return base.MakeOutputError(http.StatusBadRequest, "No key given")
	}
	// overwrite input and function to treat setSimpleValue like set
	if fn == "setSimpleValue" {
		valueArgName := args["valueArgName"]
		if valueArgName == "" {
			valueArgName = "value"
		}
		val, ok := args[valueArgName]
		if !ok {
			return base.MakeOutputError(http.StatusBadRequest, "No value given")
		}
		fn = "set"
		input = base.MakePlainOutput(val)
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
	case "search":
		{
			output = "arguments" // just for completenes, will not be read afterwards
			fullres := nsStore.GetSearchResultWithMetadata(args["key"], args["value"], args["modifiedBy"], minAge, maxAge)
			return base.MakeObjectOutput(fullres)
		}
	case "getAll":
		{
			result[ns] = nsStore.GetAllFiltered(key, args["value"], args["modifiedBy"], minAge, maxAge)
		}
	case "setAll":
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
	case "get":
		{
			argsStruct := StoreGetArgs{Namespace: ns, Key: key, Output: output, MaxAge: nil, DefaultValue: nil}
			if maxAgeRequest {
				argsStruct.MaxAge = &maxAge
			}
			val, ok := args["defaultValue"]
			if ok {
				argsStruct.DefaultValue = &val
			}
			return o.Get(ctx, input, argsStruct)
		}
	case "equals":
		{
			argsStruct := StoreEqualsArgs{Namespace: ns, Key: key, Output: output, MaxAge: nil, Value: nil}
			if maxAgeRequest {
				argsStruct.MaxAge = &maxAge
			}
			val, ok := args["value"]
			if ok {
				argsStruct.Value = &val
			}
			return o.Equals(ctx, input, argsStruct)
		}
	case "set":
		{
			if maxAgeRequest {
				io := nsStore.OverwriteValueIfOlder(key, input, maxAge, ctx.GetID())
				if io.IsError() {
					return io
				}
			}
			io := nsStore.SetValue(key, input, ctx.GetID())
			if io.IsError() {
				return io
			}
			result[ns] = map[string]*base.OperatorIO{key: input}
		}
	case "compareAndSwap":
		{
			val, ok := args["value"]
			if !ok {
				return base.MakeOutputError(http.StatusBadRequest, "No expected value given")
			}
			io := nsStore.CompareAndSwap(key, val, input, ctx.GetID())
			if io.IsError() {
				return io
			}
			result[ns] = map[string]*base.OperatorIO{key: input}
		}
	case "del", "delete", "remove":
		{
			nsStore.DeleteValue(key)
			return base.MakeEmptyOutput()
		}
	case "deleteOlder":
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

// GetFunctions returns the functions of this operator
func (o *OpStore) GetFunctions() []string {
	res := []string{"get", "getNamespaces", "set", "del", "setSimpleValue", "equals", "getAll", "setAll", "compareAndSwap", "deleteOlder", "search"}
	if db == nil {
		return res
	}
	return append(res, "createPostgresNamespace")
}

// GetPossibleArgs returns the possible arguments for a function
func (o *OpStore) GetPossibleArgs(fn string) []string {
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

// StartListening (noOp)
func (o *OpStore) StartListening(ctx *base.Context) {
}

// Shutdown (noOp)
func (o *OpStore) Shutdown(ctx *base.Context) {
}

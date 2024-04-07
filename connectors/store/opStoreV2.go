package freepsstore

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

func (o *OpStore) modifyOutputSingleNamespace(ns string, outputPtr *string, result map[string]StoreEntry) *base.OperatorIO {
	if len(result) == 1 {
		for _, v := range result {
			if v.IsError() {
				return v.GetData()
			}
		}
	}
	output := "hierarchy"
	if outputPtr != nil {
		output = *outputPtr
	}
	switch output {
	case "full":
		{
			return base.MakeObjectOutput(result)
		}
	case "arguments":
		{
			flatresult := map[string]string{}
			for k, v := range result {
				flatresult[k] = v.GetData().GetString()
			}
			return base.MakeObjectOutput(flatresult)
		}
	case "flat":
		{
			flatresult := map[string]interface{}{}
			for k, v := range result {
				flatresult[k] = v.GetData().Output
			}
			return base.MakeObjectOutput(flatresult)
		}
	case "direct":
		{
			for _, v := range result {
				return v.GetData()
			}
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
			flatresult := map[string]*base.OperatorIO{}
			for k, v := range result {
				flatresult[k] = v.GetData()
			}
			return base.MakeObjectOutput(map[string]map[string]*base.OperatorIO{ns: flatresult})
		}
	}
	return base.MakeOutputError(http.StatusBadRequest, "Unknown output type '%v'", output)
}

// NamespaceSuggestions returns a list of namespaces
func (o *OpStore) NamespaceSuggestions() []string {
	return store.GetNamespaces()
}

// OutputSuggestions returns the different output types
func (o *OpStore) OutputSuggestions() []string {
	return []string{"full", "arguments", "flat", "direct", "bool", "empty", "hierarchy"}
}

func (o *OpStore) GetNamespaces(ctx *base.Context) *base.OperatorIO {
	return base.MakeObjectOutput(store.GetNamespaces())
}

// StoreGetSetEqualArgs are the arguments for the Get, Set and Equal function
type StoreGetSetEqualArgs struct {
	Namespace    string
	Key          *string
	KeyArgName   *string
	Output       *string
	DefaultValue *string // only used for Get
	Value        *string
	ValueArgName *string // only used for Equals/Set
	MaxAge       *time.Duration
}

// Init initializes the args with default values
func (p *StoreGetSetEqualArgs) Init(ctx *base.Context, op base.FreepsOperator, fn string) {
	p.Output = new(string)
	*p.Output = "hierarchy"
}

// KeySuggestions returns a list of keys for the given namespace
func (p *StoreGetSetEqualArgs) KeySuggestions() []string {
	if p.Namespace == "" {
		return []string{}
	}
	nsStore, _ := store.GetNamespace(p.Namespace)
	if nsStore == nil {
		return []string{}
	}
	return nsStore.GetKeys()
}

// ValueSuggestions returns a list of values for the given namespace and key
func (p *StoreGetSetEqualArgs) ValueSuggestions() []string {
	if p.Namespace == "" || p.Key == nil {
		return []string{}
	}
	if *p.Key == "" {
		return []string{}
	}
	nsStore, _ := store.GetNamespace(p.Namespace)
	if nsStore == nil {
		return []string{}
	}
	v := nsStore.GetValue(*p.Key)
	if v == NotFoundEntry {
		return []string{}
	}
	return []string{v.GetData().GetString()}
}

// GetKey returns the key based on the key or keyArgName
func (p *StoreGetSetEqualArgs) GetKey(fa base.FunctionArguments) (string, error) {
	key := "key"
	if p.KeyArgName == nil {
		if p.Key == nil {
			return key, fmt.Errorf("No key given")
		}
		return *p.Key, nil
	}

	key = fa.Get(*p.KeyArgName)
	if key == "" {
		return key, fmt.Errorf("No key \"%v\"  given", *p.KeyArgName)
	}
	return key, nil
}

// Get returns a value from the store that is not older than the given maxAge; returns the default value or an error if the value is older or not found
func (o *OpStore) Get(ctx *base.Context, input *base.OperatorIO, args StoreGetSetEqualArgs, vars base.FunctionArguments) *base.OperatorIO {
	key, err := args.GetKey(vars)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	nsStore, err := store.GetNamespace(args.Namespace)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	e := StoreEntry{}
	if args.MaxAge != nil {
		e = nsStore.GetValueBeforeExpiration(key, *args.MaxAge)
	} else {
		e = nsStore.GetValue(key)
	}

	if e == NotFoundEntry && args.DefaultValue != nil {
		e = StoreEntry{base.MakePlainOutput(*args.DefaultValue), time.Now(), ctx}
	}
	return o.modifyOutputSingleNamespace(args.Namespace, args.Output, map[string]StoreEntry{key: e})
}

// Equals returns an error if the value from the store is not equal to the given value
func (o *OpStore) Equals(ctx *base.Context, input *base.OperatorIO, args StoreGetSetEqualArgs, vars base.FunctionArguments) *base.OperatorIO {
	key, err := args.GetKey(vars)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	nsStore, err := store.GetNamespace(args.Namespace)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	e := StoreEntry{}
	if args.MaxAge != nil {
		e = nsStore.GetValueBeforeExpiration(key, *args.MaxAge)
	} else {
		e = nsStore.GetValue(key)
	}
	io := e.GetData()

	if io.IsError() {
		return io
	}

	val := input.GetString()
	if args.Value != nil {
		val = *args.Value
	}

	if io.GetString() != val {
		return base.MakeOutputError(http.StatusExpectationFailed, "Values do not match")
	}
	return o.modifyOutputSingleNamespace(args.Namespace, args.Output, map[string]StoreEntry{key: e})
}

// Set sets a value in the store
func (o *OpStore) Set(ctx *base.Context, input *base.OperatorIO, args StoreGetSetEqualArgs, vars base.FunctionArguments) *base.OperatorIO {
	key, err := args.GetKey(vars)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	nsStore, err := store.GetNamespace(args.Namespace)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	var e StoreEntry
	if args.MaxAge != nil {
		e = nsStore.OverwriteValueIfOlder(key, input, *args.MaxAge, ctx)
		if e.GetData().IsError() {
			return e.GetData()
		}
	}
	e = nsStore.SetValue(key, input, ctx)
	return o.modifyOutputSingleNamespace(args.Namespace, args.Output, map[string]StoreEntry{key: e})
}

// SetSimpleValue sets a value based on a parameter and ignores the input
func (o *OpStore) SetSimpleValue(ctx *base.Context, input *base.OperatorIO, p StoreGetSetEqualArgs, vars base.FunctionArguments) *base.OperatorIO {
	value := ""
	if p.ValueArgName == nil {
		if p.Value == nil {
			return base.MakeOutputError(http.StatusBadRequest, "No value given")
		}
		value = *p.Value
	} else {
		if !vars.Has(*p.ValueArgName) {
			return base.MakeOutputError(http.StatusBadRequest, "No value \"%v\" given", *p.ValueArgName)
		}
		value = vars.Get(*p.ValueArgName)
	}
	return o.Set(ctx, base.MakePlainOutput(value), p, vars)
}

// Delete deletes a key from the store
func (o *OpStore) Delete(ctx *base.Context, input *base.OperatorIO, args StoreGetSetEqualArgs, vars base.FunctionArguments) *base.OperatorIO {
	key, err := args.GetKey(vars)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	nsStore, err := store.GetNamespace(args.Namespace)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	nsStore.DeleteValue(key)
	return base.MakeEmptyOutput()
}

// Del deletes a key from the store
func (o *OpStore) Del(ctx *base.Context, input *base.OperatorIO, args StoreGetSetEqualArgs, vars base.FunctionArguments) *base.OperatorIO {
	return o.Delete(ctx, input, args, vars)
}

// Remove deletes a key from the store
func (o *OpStore) Remove(ctx *base.Context, input *base.OperatorIO, args StoreGetSetEqualArgs, vars base.FunctionArguments) *base.OperatorIO {
	return o.Delete(ctx, input, args, vars)
}

// CASArgs are the arguments for the CompareAndSwap function
type CASArgs struct {
	Namespace string
	Key       string
	Output    *string
	Value     string
}

// KeySuggestions returns a list of keys for the given namespace
func (p *CASArgs) KeySuggestions() []string {
	if p.Namespace == "" {
		return []string{}
	}

	nsStore, _ := store.GetNamespace(p.Namespace)
	if nsStore == nil {
		return []string{}
	}
	return nsStore.GetKeys()
}

// CompareAndSwap sets a value in the store if the current value is equal to the given value
func (o *OpStore) CompareAndSwap(ctx *base.Context, input *base.OperatorIO, args CASArgs) *base.OperatorIO {
	nsStore, err := store.GetNamespace(args.Namespace)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	e := nsStore.CompareAndSwap(args.Key, args.Value, input, ctx)

	return o.modifyOutputSingleNamespace(args.Namespace, args.Output, map[string]StoreEntry{args.Key: e})
}

// StoreSearchArgs are the arguments for the StoreSet function
type StoreSearchArgs struct {
	Namespace  string
	Key        *string
	Value      *string
	ModifiedBy *string
	MinAge     *time.Duration
	MaxAge     *time.Duration
	Output     *string
}

// Init initializes the StoreSearchArgs with default values
func (p *StoreSearchArgs) Init(ctx *base.Context, op base.FreepsOperator, fn string) {
	p.Output = new(string)
	*p.Output = "hierarchy"
	p.Key = new(string)
	p.Value = new(string)
	p.ModifiedBy = new(string)
	p.MinAge = new(time.Duration)
	*p.MinAge = 0
	p.MaxAge = new(time.Duration)
	*p.MaxAge = math.MaxInt64
}

var _ base.FreepsFunctionParametersWithInit = &StoreSearchArgs{}

// Search searches the store for values matching the given criteria
func (o *OpStore) Search(ctx *base.Context, input *base.OperatorIO, args StoreSearchArgs) *base.OperatorIO {
	nsStore, err := store.GetNamespace(args.Namespace)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	res := nsStore.GetSearchResultWithMetadata(*args.Key, *args.Value, *args.ModifiedBy, *args.MinAge, *args.MaxAge)
	return o.modifyOutputSingleNamespace(args.Namespace, args.Output, res)
}

// GetHook returns the hook for this operator
func (o *OpStore) GetHook() interface{} {
	eLog, err := store.GetNamespace(executionLogNamespace)
	if err != nil {
		// set alert
	}

	debugNs, err := store.GetNamespace(debugNamespace)
	if err != nil {
		// set alert
	}

	return &HookStore{executionLogNs: eLog, debugNs: debugNs, GE: o.GE}
}

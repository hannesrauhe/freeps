package freepsstore

import (
	"math"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

func (o *OpStore) modifyOutputSingleNamespace(ns string, output string, result map[string]StoreEntry) *base.OperatorIO {
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

// StoreGetArgs are the arguments for the StoreGet function
type StoreGetArgs struct {
	Namespace    string
	Key          string
	Output       string
	DefaultValue *string // only used for Get
	Value        *string // only used for Equals
	MaxAge       *time.Duration
}

// NamespaceSuggestions returns a list of namespaces
func (p *StoreGetArgs) NamespaceSuggestions(oc *OpStore) []string {
	return store.GetNamespaces()
}

// Get returns a value from the store that is not older than the given maxAge; returns the default value or an error if the value is older or not found
func (o *OpStore) Get(ctx *base.Context, input *base.OperatorIO, args StoreGetArgs) *base.OperatorIO {
	nsStore := store.GetNamespace(args.Namespace)
	e := StoreEntry{}
	if args.MaxAge != nil {
		e = nsStore.GetValueBeforeExpiration(args.Key, *args.MaxAge)
	} else {
		e = nsStore.GetValue(args.Key)
	}
	io := e.GetData()

	if io.IsError() {
		if args.DefaultValue == nil {
			return io
		}
		e = StoreEntry{base.MakePlainOutput(*args.DefaultValue), time.Now(), ctx.GetID()}
	}
	return o.modifyOutputSingleNamespace(args.Namespace, args.Output, map[string]StoreEntry{args.Key: e})
}

// Equals returns an error if the value from the store is not equal to the given value
func (o *OpStore) Equals(ctx *base.Context, input *base.OperatorIO, args StoreGetArgs) *base.OperatorIO {
	nsStore := store.GetNamespace(args.Namespace)
	e := StoreEntry{}
	if args.MaxAge != nil {
		e = nsStore.GetValueBeforeExpiration(args.Key, *args.MaxAge)
	} else {
		e = nsStore.GetValue(args.Key)
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
	return o.modifyOutputSingleNamespace(args.Namespace, args.Output, map[string]StoreEntry{args.Key: e})
}

// StoreSetArgs are the arguments for the StoreSet function
type StoreSetArgs struct {
	Namespace string
	Key       string
	Output    string
	MaxAge    *time.Duration
}

// NamespaceSuggestions returns a list of namespaces
func (p *StoreSetArgs) NamespaceSuggestions(oc *OpStore) []string {
	return store.GetNamespaces()
}

// Set sets a value in the store
func (o *OpStore) Set(ctx *base.Context, input *base.OperatorIO, args StoreSetArgs) *base.OperatorIO {
	nsStore := store.GetNamespace(args.Namespace)
	var e StoreEntry
	if args.MaxAge != nil {
		e = nsStore.OverwriteValueIfOlder(args.Key, input, *args.MaxAge, ctx.GetID())
		if e.GetData().IsError() {
			return e.GetData()
		}
	}
	e = nsStore.SetValue(args.Key, input, ctx.GetID())
	return o.modifyOutputSingleNamespace(args.Namespace, args.Output, map[string]StoreEntry{args.Key: e})
}

// Delete deletes a key from the store
func (o *OpStore) Delete(ctx *base.Context, input *base.OperatorIO, args StoreSetArgs) *base.OperatorIO {
	nsStore := store.GetNamespace(args.Namespace)
	nsStore.DeleteValue(args.Key)
	return base.MakeEmptyOutput()
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

// NamespaceSuggestions returns a list of namespaces
func (p *StoreSearchArgs) NamespaceSuggestions(oc *OpStore) []string {
	return store.GetNamespaces()
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
	nsStore := store.GetNamespace(args.Namespace)
	res := nsStore.GetSearchResultWithMetadata(*args.Key, *args.Value, *args.ModifiedBy, *args.MinAge, *args.MaxAge)
	return o.modifyOutputSingleNamespace(args.Namespace, *args.Output, res)
}

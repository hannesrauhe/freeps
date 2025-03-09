package ui

import (
	"fmt"
	"html/template"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
)

func (o *OpUI) createTemplateFuncMap(ctx *base.Context) template.FuncMap {
	funcMap := template.FuncMap{
		"add": func(a int, b int) int {
			return a + b
		},
		"divisibleBy": func(a int, b int) bool {
			return a != 0 && a%b == 0
		},
		"hasField": func(v interface{}, name string) bool {
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Ptr {
				rv = rv.Elem()
			}
			if rv.Kind() == reflect.Map {
				return rv.MapIndex(reflect.ValueOf(name)).IsValid()
			}
			if rv.Kind() != reflect.Struct {
				return false
			}
			return rv.FieldByName(name).IsValid()
		},
		"getContextID": func() string {
			return ctx.GetID()
		},
		"store_GetNamespaces": func() []string {
			ns := freepsstore.GetGlobalStore().GetNamespaces()
			sort.Strings(ns)
			return ns
		},
		"store_GetKeys": func(namespace string) []string {
			ns := freepsstore.GetGlobalStore().GetNamespaceNoError(namespace)
			if ns == nil {
				return nil
			}
			keys := ns.GetKeys()
			sort.Strings(keys)
			return keys
		},
		"store_GetAll": func(namespace string) map[string]*base.OperatorIO {
			ns := freepsstore.GetGlobalStore().GetNamespaceNoError(namespace)
			if ns == nil {
				return nil
			}
			return ns.GetAllValues(100)
		},
		"store_Search": func(namespace string, keyPattern string, valuePattern string, modifiedByPattern string, minAgeStr string, maxAgeStr string) map[string]freepsstore.ReadableStoreEntry {
			ns := freepsstore.GetGlobalStore().GetNamespaceNoError(namespace)
			if ns == nil {
				return nil
			}
			minAge := time.Duration(0)
			maxAge := time.Duration(math.MaxInt64)
			if minAgeStr != "" {
				minAge, _ = time.ParseDuration(minAgeStr)
			}
			if maxAgeStr != "" {
				maxAge, _ = time.ParseDuration(maxAgeStr)
			}
			retMap := map[string]freepsstore.ReadableStoreEntry{}
			for k, v := range ns.GetSearchResultWithMetadata(keyPattern, valuePattern, modifiedByPattern, minAge, maxAge) {
				retMap[k] = v.GetHumanReadable()
			}
			return retMap
		},
		"store_Get": func(namespace string, key string) interface{} {
			ns := freepsstore.GetGlobalStore().GetNamespaceNoError(namespace)
			if ns == nil {
				return nil
			}
			v := ns.GetValue(key)
			return v.GetData().Output
		},
		"store_GetString": func(namespace string, key string) string {
			ns := freepsstore.GetGlobalStore().GetNamespaceNoError(namespace)
			if ns == nil {
				return ""
			}
			v := ns.GetValue(key)
			return v.GetData().GetString()
		},
		"ge_GetOperators": func() []string {
			return o.ge.GetOperators()
		},
		"flow_GetFlowDescByTag": func(tagstr string) map[string]freepsflow.FlowDesc {
			tags := []string{}
			if tagstr != "" {
				tags = strings.Split(tagstr, ",")
			}
			return o.ge.GetFlowDescByTag(tags)
		},
		"flow_GetFlowSortedByNamesByTag": func(tagstr string) map[string]freepsflow.FlowDesc {
			flowByName := map[string]freepsflow.FlowDesc{}
			tags := []string{}
			if tagstr != "" {
				tags = strings.Split(tagstr, ",")
			}
			flowByID := o.ge.GetFlowDescByTag(tags)
			for flowID, v := range flowByID {
				name := flowID
				gd, err := v.GetCompleteDesc(flowID, o.ge)
				if err != nil {
					name = flowID + " (Error: " + err.Error() + ")"
				} else {
					name = gd.DisplayName
				}
				// add name to flow, if duplicate add id
				if _, ok := flowByName[name]; ok {
					flowByName[fmt.Sprintf("%v (ID: %v)", name, flowID)] = *gd
				} else {
					flowByName[name] = *gd
				}

			}
			return flowByName
		},
		"flow_ExecuteFlow": func(flowName string, mainArgsStr string) *base.OperatorIO {
			fa, err := base.NewFunctionArgumentsFromURLQuery(mainArgsStr)
			if err != nil {
				return base.MakeOutputError(400, "Could not parse mainArgs: %v", err)
			}
			return o.ge.ExecuteFlow(ctx, flowName, fa, base.MakeEmptyOutput())
		},
		"flow_ExecuteOperator": func(op string, fn string, mainArgsStr string) *base.OperatorIO {
			fa, err := base.NewFunctionArgumentsFromURLQuery(mainArgsStr)
			if err != nil {
				return base.MakeOutputError(400, "Could not parse mainArgs: %v", err)
			}
			return o.ge.ExecuteOperatorByName(ctx, op, fn, fa, base.MakeEmptyOutput())
		},
		"flow_GetTagMap": func() map[string][]string {
			return o.ge.GetTagMap()
		},
		"operator_GetFunctions": func(opName string) []string {
			op := o.ge.GetOperator(opName)
			if op == nil {
				return []string{}
			}
			return op.GetFunctions()
		},
		"operator_GetPossigbleArgs": func(opName string, fn string) []string {
			op := o.ge.GetOperator(opName)
			if op == nil {
				return []string{}
			}
			args := op.GetPossibleArgs(fn)
			if len(args) > 100 {
				return args[:100]
			}
			return args
		},
		"operator_GetArgSuggestions": func(opName string, fn string, arg string) map[string]string {
			op := o.ge.GetOperator(opName)
			if op == nil {
				return map[string]string{}
			}
			argSugg := op.GetArgSuggestions(fn, arg, base.MakeEmptyFunctionArguments())
			if len(argSugg) > 100 {
				argSuggShort := map[string]string{}
				i := 0
				for k, v := range argSugg {
					argSuggShort[k] = v
					i++
					if i > 100 {
						break
					}
				}
				return argSuggShort
			}
			return argSugg
		},
	}
	return funcMap
}

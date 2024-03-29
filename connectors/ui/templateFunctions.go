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
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
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
		"graph_GetGraphDescByTag": func(tagstr string) map[string]freepsgraph.GraphDesc {
			tags := []string{}
			if tagstr != "" {
				tags = strings.Split(tagstr, ",")
			}
			return o.ge.GetGraphDescByTag(tags)
		},
		"graph_GetGraphSortedByNamesByTag": func(tagstr string) map[string]freepsgraph.GraphDesc {
			graphByName := map[string]freepsgraph.GraphDesc{}
			tags := []string{}
			if tagstr != "" {
				tags = strings.Split(tagstr, ",")
			}
			graphByID := o.ge.GetGraphDescByTag(tags)
			for graphID, v := range graphByID {
				name := graphID
				gd, err := v.GetCompleteDesc(graphID, o.ge)
				if err != nil {
					name = graphID + " (Error: " + err.Error() + ")"
				} else {
					name = gd.DisplayName
				}
				// add name to graph, if duplicate add id
				if _, ok := graphByName[name]; ok {
					graphByName[fmt.Sprintf("%v (ID: %v)", name, graphID)] = *gd
				} else {
					graphByName[name] = *gd
				}

			}
			return graphByName
		},
		"graph_ExecuteGraph": func(graphName string, mainArgsStr string) *base.OperatorIO {
			mainArgs, err := utils.URLParseQuery(mainArgsStr)
			if err != nil {
				return base.MakeOutputError(400, "Could not parse mainArgs: %v", err)
			}
			return o.ge.ExecuteGraph(ctx, graphName, base.NewFunctionArguments(mainArgs), base.MakeEmptyOutput())
		},
		"graph_ExecuteOperator": func(op string, fn string, mainArgsStr string) *base.OperatorIO {
			mainArgs, err := utils.URLParseQuery(mainArgsStr)
			if err != nil {
				return base.MakeOutputError(400, "Could not parse mainArgs: %v", err)
			}
			return o.ge.ExecuteOperatorByName(ctx, op, fn, base.NewFunctionArguments(mainArgs), base.MakeEmptyOutput())
		},
		"graph_GetTagMap": func() map[string][]string {
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
			argSugg := op.GetArgSuggestions(fn, arg, map[string]string{})
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

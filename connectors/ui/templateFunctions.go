package ui

import (
	"html/template"
	"strings"

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
		"store_GetNamespaces": func() []string {
			return freepsstore.GetGlobalStore().GetNamespaces()
		},
		"store_GetKeys": func(namespace string) []string {
			ns := freepsstore.GetGlobalStore().GetNamespace(namespace)
			if ns == nil {
				return nil
			}
			return ns.GetKeys()
		},
		"store_GetAll": func(namespace string) map[string]*base.OperatorIO {
			ns := freepsstore.GetGlobalStore().GetNamespace(namespace)
			if ns == nil {
				return nil
			}
			return ns.GetAllValues(100)
		},
		"store_Get": func(namespace string, key string) interface{} {
			ns := freepsstore.GetGlobalStore().GetNamespace(namespace)
			if ns == nil {
				return nil
			}
			v := ns.GetValue(key)
			if v == nil {
				return nil
			}
			return v.Output
		},
		"graph_GetGraphInfoByTag": func(tagstr string) map[string]freepsgraph.GraphInfo {
			tags := strings.Split(tagstr, ",")
			return o.ge.GetGraphInfoByTag(tags)
		},
		"graph_ExecuteGraph": func(graphName string, mainArgsStr string) *base.OperatorIO {
			mainArgs, err := utils.URLParseQuery(mainArgsStr)
			if err != nil {
				return base.MakeOutputError(400, "Could not parse mainArgs: %v", err)
			}
			return o.ge.ExecuteGraph(ctx, graphName, mainArgs, base.MakeEmptyOutput())
		},
		"graph_ExecuteOperator": func(op string, fn string, mainArgsStr string) *base.OperatorIO {
			mainArgs, err := utils.URLParseQuery(mainArgsStr)
			if err != nil {
				return base.MakeOutputError(400, "Could not parse mainArgs: %v", err)
			}
			return o.ge.ExecuteOperatorByName(ctx, op, fn, mainArgs, base.MakeEmptyOutput())
		},
	}
	return funcMap
}
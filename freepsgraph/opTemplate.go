package freepsgraph

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type OpTemplate struct {
	ge  *GraphEngine
	tmc *freepsdo.TemplateMod
	cr  *utils.ConfigReader
}

var _ FreepsOperator = &OpTemplate{}

func NewTemplateOperator(ge *GraphEngine, cr *utils.ConfigReader) *OpTemplate {
	return &OpTemplate{ge: ge, tmc: freepsdo.NewTemplateMod(cr), cr: cr}
}

func (o *OpTemplate) Execute(ctx *utils.Context, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	switch fn {
	case "convertAll":
		r := make(map[string]GraphDesc)
		for n, t := range o.tmc.Config.Templates {
			g := o.convertTemplateToGraphDesc(t)
			o.ge.AddTemporaryGraph(n, g)
			r[n] = *g
		}
		return MakeObjectOutput(r)
	case "convertAllAndSave":
		r := make(map[string]GraphDesc)
		for n, t := range o.tmc.Config.Templates {
			g := o.convertTemplateToGraphDesc(t)
			r[n] = *g
		}
		if err := o.ge.AddExternalGraphs(r, "template2graphs.json"); err != nil {
			return MakeOutputError(http.StatusInternalServerError, "Could not store: %s", err)
		}
		return MakeObjectOutput(r)

	case "convert":
		tName, ok := mainArgs["name"]
		if !ok {
			return MakeOutputError(http.StatusBadRequest, "Missing argument name")
		}
		t, ok := o.tmc.GetTemplate(tName)
		if !ok {
			return MakeOutputError(http.StatusBadRequest, "Unknown template %v", tName)
		}
		g := o.convertTemplateToGraphDesc(t)
		o.ge.AddTemporaryGraph(tName, g)
		return MakeObjectOutput(g)
	}
	return MakeOutputError(http.StatusBadRequest, "Unknown function %v", fn)
}

func (o *OpTemplate) convertTemplateToGraphDesc(t *freepsdo.Template) *GraphDesc {
	pos := 0
	gd := &GraphDesc{Operations: make([]GraphOperationDesc, 0)}
	o.convert(&pos, gd, t, ROOT_SYMBOL)
	gd.OutputFrom = fmt.Sprintf("#%v", pos-1)
	return gd
}

func (o *OpTemplate) convert(pos *int, gd *GraphDesc, t *freepsdo.Template, caller string) {
	for _, ta := range t.Actions {
		args := make(map[string]string, 0)
		for k, v := range ta.Args {
			args[k] = fmt.Sprintf("%v", v)
		}
		operator := ta.Mod
		argsFrom := ""
		inputFrom := ""
		name := fmt.Sprintf("#%v", *pos)
		if operator == "template" {
			operator = "graph"
		}

		// template mods only had one type of input, operators have args and input
		// depending on the type of input they want arguments or input from the previous output
		switch operator {
		case "eval":
			if ta.Fn == "eval" {
				args["Output"] = "args" // eval output may be used as argument later
			}
			inputFrom = caller
		case "fritz":
			argsFrom = caller
		default:
			inputFrom = caller
		}
		god := GraphOperationDesc{Name: name, Operator: operator, Function: ta.Fn, Arguments: args, ArgumentsFrom: argsFrom, InputFrom: inputFrom}
		gd.Operations = append(gd.Operations, god)
		*pos++
		if ta.FwdTemplate != nil {
			o.convert(pos, gd, ta.FwdTemplate, name)
		}
		if ta.FwdTemplateName != "" {
			fwdGod := GraphOperationDesc{Name: fmt.Sprintf("#%v", *pos), Operator: "graph", Function: ta.FwdTemplateName, InputFrom: name}
			gd.Operations = append(gd.Operations, fwdGod)
			*pos++
		}
	}
}

func (o *OpTemplate) GetFunctions() []string {
	return []string{"convert", "convertAll"}
}

func (o *OpTemplate) GetPossibleArgs(fn string) []string {
	return []string{}
}

func (o *OpTemplate) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

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

func (o *OpTemplate) Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	switch fn {
	case "convertAll":
		r := make(map[string]GraphDesc)
		for n, t := range o.tmc.Config.Templates {
			g := o.convertTemplateToGraphDesc(t)
			o.ge.AddTemporaryGraph(n, g)
			r[n] = *g
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
	o.convert(&pos, gd, t, ROOT_SYMBOL, ROOT_SYMBOL)
	gd.OutputFrom = fmt.Sprintf("#%v", pos-1)
	return gd
}

func (o *OpTemplate) convert(pos *int, gd *GraphDesc, t *freepsdo.Template, ArgsFrom string, InputFrom string) {
	for _, ta := range t.Actions {
		args := make(map[string]string, 0)
		for k, v := range ta.Args {
			args[k] = fmt.Sprintf("%v", v)
		}
		operator := ta.Mod
		if operator == "template" {
			operator = "graph"
		}
		god := GraphOperationDesc{Name: fmt.Sprintf("#%v", *pos), Operator: operator, Function: ta.Fn, Arguments: args, ArgumentsFrom: ArgsFrom}
		gd.Operations = append(gd.Operations, god)
		fwdArgsFrom := fmt.Sprintf("%v", *pos)
		*pos++
		if ta.FwdTemplate != nil {
			o.convert(pos, gd, ta.FwdTemplate, fwdArgsFrom, fwdArgsFrom)
		}
		if ta.FwdTemplateName != "" {
			fwdGod := GraphOperationDesc{Name: fmt.Sprintf("#%v", *pos), Operator: "graph", Function: ta.FwdTemplateName, ArgumentsFrom: fwdArgsFrom}
			gd.Operations = append(gd.Operations, fwdGod)
			*pos++
		}
	}
}

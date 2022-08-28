package freepsgraph

import (
	"fmt"

	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type OpTemplate struct {
	tmc *freepsdo.TemplateMod
}

var _ FreepsOperator = &OpTemplate{}

func NewTemplateOperator(cr *utils.ConfigReader) *OpTemplate {
	return &OpTemplate{tmc: freepsdo.NewTemplateMod(cr)}
}

func (o *OpTemplate) Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	if fn == "convertAll" {
		r := make(map[string]GraphDesc)
		for n, t := range o.tmc.Config.Templates {
			gd := o.Convert(n, t)
			r[n] = gd
		}
		return &OperatorIO{HttpCode: 200, Output: r}
	}
	return MakeOutputError(404, "No template with name \"%s\" found", fn)
}

func (o *OpTemplate) ConvertAction(pos int, ta *freepsdo.TemplateAction) GraphOperationDesc {
	args := make(map[string]string, 0)
	for k, v := range ta.Args {
		args[k] = fmt.Sprintf("%v", v)
	}
	return GraphOperationDesc{Name: fmt.Sprintf("%v", pos), Operator: ta.Mod, Function: ta.Fn, Arguments: args}
}

func (o *OpTemplate) Convert(name string, t *freepsdo.Template) GraphDesc {
	gd := GraphDesc{Name: name, Operations: make([]GraphOperationDesc, 0)}
	pos := 0
	for _, a := range t.Actions {
		parentPos := pos
		god := o.ConvertAction(pos, &a)
		pos++
		gd.Operations = append(gd.Operations, god)
		if a.FwdTemplateName != "" {
			fwdGod := GraphOperationDesc{Name: fmt.Sprintf("%v", pos), Operator: "graph", Function: a.FwdTemplateName, InputFrom: fmt.Sprintf("%v", parentPos)}
			gd.Operations = append(gd.Operations, fwdGod)
		}
	}
	return gd
}

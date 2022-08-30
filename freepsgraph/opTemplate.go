package freepsgraph

import (
	"fmt"

	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type OpTemplate struct {
	tmc *freepsdo.TemplateMod
	cr  *utils.ConfigReader
}

var _ FreepsOperator = &OpTemplate{}

func NewTemplateOperator(cr *utils.ConfigReader) *OpTemplate {
	return &OpTemplate{tmc: freepsdo.NewTemplateMod(cr), cr: cr}
}

func (o *OpTemplate) Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	if fn == "convertAndStore" {
		r := make(map[string]GraphDesc)
		for n, t := range o.tmc.Config.Templates {
			pos := 0
			gd := GraphDesc{Name: n, Operations: make([]GraphOperationDesc, 0)}
			o.Convert(&pos, &gd, t, ROOT_SYMBOL, ROOT_SYMBOL)
			gd.OutputFrom = fmt.Sprintf("#%v", pos-1)
			r[n] = gd
		}
		o.cr.WriteObjectToFile(r, "convertedGraphs.json")
		return &OperatorIO{HTTPCode: 200, Output: r}
	}
	return MakeOutputError(404, "No template with name \"%s\" found", fn)
}

func (o *OpTemplate) Convert(pos *int, gd *GraphDesc, t *freepsdo.Template, ArgsFrom string, InputFrom string) {
	for _, ta := range t.Actions {
		args := make(map[string]string, 0)
		for k, v := range ta.Args {
			args[k] = fmt.Sprintf("%v", v)
		}
		god := GraphOperationDesc{Name: fmt.Sprintf("#%v", *pos), Operator: ta.Mod, Function: ta.Fn, Arguments: args, ArgumentsFrom: ArgsFrom}
		gd.Operations = append(gd.Operations, god)
		fwdArgsFrom := fmt.Sprintf("%v", *pos)
		*pos++
		if ta.FwdTemplate != nil {
			o.Convert(pos, gd, ta.FwdTemplate, fwdArgsFrom, fwdArgsFrom)
		}
		if ta.FwdTemplateName != "" {
			fwdGod := GraphOperationDesc{Name: fmt.Sprintf("#%v", *pos), Operator: "graph", Function: ta.FwdTemplateName, ArgumentsFrom: fwdArgsFrom}
			gd.Operations = append(gd.Operations, fwdGod)
			*pos++
		}
	}
}

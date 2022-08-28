package freepsgraph

import (
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
		r := make(map[string]*GraphDesc)
		for n, t := range o.tmc.Config.Templates {
			gd := o.Convert(n, t)
			r[n] = gd
		}
		return &OperatorIO{HttpCode: 200, Output: r}
	}
	return MakeOutputError(404, "No template with name \"%s\" found", fn)
}

func (o *OpTemplate) Convert(name string, t *freepsdo.Template) *GraphDesc {
	return &GraphDesc{Name: name}
}

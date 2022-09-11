package freepsgraph

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type OpUI struct {
	modinator *freepsdo.TemplateMod
	ge        *GraphEngine
}

var _ FreepsOperator = &OpUI{}

type TemplateData struct {
	Args           map[string]string
	ModSuggestions map[string]bool
	FnSuggestions  map[string]bool
	ArgSuggestions map[string]map[string]string
	Templates      map[string]bool
	TemplateJSON   string
	Output         string
}

type ShowGraphsData struct {
	Graphs []string
	Dot    string
}

// NewHTMLUI creates a UI interface based on the inline template above
func NewHTMLUI(modinator *freepsdo.TemplateMod, graphEngine *GraphEngine) *OpUI {
	h := OpUI{modinator: modinator, ge: graphEngine}

	return &h
}

func (o *OpUI) graphToDot(gd *GraphDesc) string {
	var s strings.Builder
	s.WriteString("digraph G {")
	s.WriteString("\nArguments")
	s.WriteString("\nInput")
	s.WriteString("\nOutput")
	for _, node := range gd.Operations {
		v := utils.ClearString(node.Name)
		argsF := "Arguments"
		if node.ArgumentsFrom != "" {
			if node.ArgumentsFrom == ROOT_SYMBOL {
				argsF = "Input"
			} else {
				argsF = utils.ClearString(node.ArgumentsFrom)
			}
		}
		s.WriteString("\n" + v)
		s.WriteString("\n" + argsF + "->" + v)

		if node.InputFrom != "" {
			inputF := "Input"
			if node.InputFrom != ROOT_SYMBOL {
				inputF = utils.ClearString(node.InputFrom)
			}
			s.WriteString("\n" + inputF + "->" + v + " [style=dashed]")
		}
	}
	OutputFrom := utils.ClearString(gd.Operations[len(gd.Operations)-1].Name)
	if gd.OutputFrom != "" {
		OutputFrom = utils.ClearString(gd.OutputFrom)
	}
	s.WriteString("\n" + OutputFrom + "->Output [style=dashed]")

	s.WriteString("\n}")
	return s.String()
}

func (o *OpUI) createTemplate(templateString string, templateData interface{}) *OperatorIO {
	t := template.New("general")
	t, _ = t.Parse(templateString)
	var w bytes.Buffer
	err := t.Execute(&w, templateData)
	if err != nil {
		log.Println(err)
		return MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	return MakeByteOutputWithContentType(w.Bytes(), "text/html; charset=utf-8")
}

func (o *OpUI) buildPartialTemplate(vars map[string]string) *GraphDesc {
	gd := &GraphDesc{}
	v, ok := vars["TemplateJSON"]
	if ok {
		json.Unmarshal([]byte(v), gd)
	}
	opNum, _ := vars["NumOps"]
	if gd.Operations == nil || len(gd.Operations) == 0 {
		gd.Operations = make([]GraphOperationDesc, 1)
		gd.Operations[0] = GraphOperationDesc{Operator: "echo", Function: "hello", Arguments: map[string]string{}}
	}
	targetNum, _ := strconv.Atoi(opNum)
	for len(gd.Operations) <= targetNum {
		gd.Operations = append(gd.Operations, GraphOperationDesc{Operator: "echo", Function: "hello", Arguments: map[string]string{}})
	}
	gopd := &gd.Operations[targetNum]
	for k, v := range vars {
		if len(k) > 4 && k[0:4] == "arg." {
			gopd.Arguments[k[4:]] = v
		}
		if k == "mod" {
			if _, ok := o.modinator.Mods[v]; ok {
				gopd.Operator = v
			}
		}
		if k == "fn" {
			gopd.Function = v
		}
	}

	return gd
}

func (o *OpUI) editGraph(vars map[string]string, input *OperatorIO) *OperatorIO {
	var gd *GraphDesc
	var exists bool
	if input.IsEmpty() {
		gd, exists = o.ge.GetGraphDesc(vars["graph"])
	}
	if !input.IsEmpty() || !exists {
		inBytes, err := input.GetBytes()
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, err.Error())
		}
		formInputQueryFormat, err := url.ParseQuery(string(inBytes))
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, err.Error())
		}
		formInput := utils.URLArgsToMap(formInputQueryFormat)
		gd = o.buildPartialTemplate(formInput)
	}
	td := &TemplateData{ModSuggestions: map[string]bool{}, FnSuggestions: map[string]bool{}, ArgSuggestions: make(map[string]map[string]string), Templates: map[string]bool{}}
	b, _ := json.MarshalIndent(gd, "", "  ")
	td.TemplateJSON = string(b)
	gopd := &gd.Operations[0]
	for k := range o.modinator.Mods {
		td.ModSuggestions[k] = (k == gopd.Operator)
	}

	mod := o.modinator.Mods[gopd.Operator]
	for _, k := range mod.GetFunctions() {
		td.FnSuggestions[k] = (k == gopd.Function)
	}

	for _, k := range mod.GetPossibleArgs(gopd.Function) {
		td.ArgSuggestions[k] = mod.GetArgSuggestions(gopd.Function, k, map[string]interface{}{})
	}

	// if vars.Has("Execute") {
	// 	jrw := freepsdo.NewResponseCollector(fmt.Sprintf("HTML UI: %v", req.RemoteAddr))
	// 	r.modinator.ExecuteTemplateAction(ta, jrw)
	// 	_, _, bytes := jrw.GetFinalResponse(true)
	// 	if len(bytes) == 0 {
	// 		td.Output = "<no content>"
	// 	} else {
	// 		td.Output = string(bytes)
	// 	}
	// }

	// if vars.Has("SaveTemplate") {
	// 	//TODO: use systemmod instead
	// 	r.modinator.SaveTemplateAction(vars.Get("TemplateName"), ta)
	// 	td.Output = "Saved " + vars.Get("TemplateName")
	// }

	return o.createTemplate(templateEditGraph, td)
}

func (o *OpUI) showGraphs(vars map[string]string, input *OperatorIO) *OperatorIO {
	t := template.New("showGraphs")
	t, _ = t.Parse(templateShowGraphs)
	var d ShowGraphsData
	d.Graphs = make([]string, 0)
	for n := range o.ge.GetAllGraphDesc() {
		d.Graphs = append(d.Graphs, n)
	}
	if g, ok := vars["graph"]; ok {
		if gd, ok := o.ge.GetGraphDesc(g); ok {
			d.Dot = o.graphToDot(gd)
		}
	}

	return o.createTemplate(templateShowGraphs, &d)
}

func (o *OpUI) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "edit":
		return o.editGraph(vars, input)
	}
	return o.showGraphs(vars, input)
}

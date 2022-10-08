package freepsgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/utils"
)

type OpUI struct {
	ge *GraphEngine
	cr *utils.ConfigReader
}

var _ FreepsOperator = &OpUI{}

type TemplateData struct {
	Args                 map[string]string
	OpSuggestions        map[string]bool
	FnSuggestions        map[string]bool
	ArgSuggestions       map[string]map[string]string
	InputFromSuggestions map[string]bool
	GraphJSON            string
	Output               string
	Numop                int
}

type ShowGraphsData struct {
	Graphs []string
	Dot    string
}

type EditConfigData struct {
	ConfigText string
}

// NewHTMLUI creates a UI interface based on the inline template above
func NewHTMLUI(cr *utils.ConfigReader, graphEngine *GraphEngine) *OpUI {
	h := OpUI{ge: graphEngine, cr: cr}

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

func (o *OpUI) buildPartialGraph(formInput map[string]string) (*GraphDesc, int) {
	gd := &GraphDesc{}
	v, ok := formInput["GraphJSON"]
	if ok {
		json.Unmarshal([]byte(v), gd)
	}
	opNum, _ := formInput["numop"]
	targetNum, _ := strconv.Atoi(opNum)
	if targetNum < 0 {
		targetNum = 0
	}
	if gd.Operations == nil || len(gd.Operations) == 0 {
		gd.Operations = make([]GraphOperationDesc, targetNum+1)
	}
	for len(gd.Operations) <= targetNum {
		gd.Operations = append(gd.Operations, GraphOperationDesc{Operator: "echo", Function: "hello", Arguments: map[string]string{}})
	}
	gopd := &gd.Operations[targetNum]
	for k, v := range formInput {
		if len(k) > 4 && k[0:4] == "arg." {
			if gopd.Arguments == nil {
				gopd.Arguments = make(map[string]string)
			}
			gopd.Arguments[k[4:]] = v
		}
		if k == "op" {
			gopd.Operator = v
		}
		if k == "fn" {
			gopd.Function = v
		}
		if k == "inputFrom" {
			gopd.InputFrom = v
		}
	}

	return gd, targetNum
}

func (o *OpUI) editGraph(vars map[string]string, input *OperatorIO) *OperatorIO {
	var gd *GraphDesc
	var exists bool
	targetNum := 0

	td := &TemplateData{OpSuggestions: map[string]bool{}, FnSuggestions: map[string]bool{}, ArgSuggestions: make(map[string]map[string]string), InputFromSuggestions: map[string]bool{}}

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
		gd, targetNum = o.buildPartialGraph(formInput)

		if _, ok := formInput["SaveGraph"]; ok {
			name := formInput["GraphName"]
			o.ge.AddTemporaryGraph(name, gd)
		}
		if _, ok := formInput["SaveTemporarily"]; ok {
			name := formInput["GraphName"]
			o.ge.AddTemporaryGraph(name, gd)
		}

		if _, ok := formInput["Execute"]; ok {
			o.ge.AddTemporaryGraph("UIgraph", gd)
			output := o.ge.ExecuteGraph("UIgraph", map[string]string{}, MakeEmptyOutput())
			td.Output = output.GetString()
		}
	}
	b, _ := json.MarshalIndent(gd, "", "  ")
	td.GraphJSON = string(b)
	gopd := &gd.Operations[targetNum]
	td.Numop = targetNum
	td.Args = gopd.Arguments
	for _, k := range o.ge.GetOperators() {
		td.OpSuggestions[k] = (k == gopd.Operator)
	}

	if o.ge.HasOperator(gopd.Operator) {
		mod := o.ge.GetOperator(gopd.Operator)
		for _, k := range mod.GetFunctions() {
			td.FnSuggestions[k] = (k == gopd.Function)
		}
		for _, k := range mod.GetPossibleArgs(gopd.Function) {
			td.ArgSuggestions[k] = mod.GetArgSuggestions(gopd.Function, k, map[string]string{})
		}
	}

	td.InputFromSuggestions[ROOT_SYMBOL] = (ROOT_SYMBOL == gopd.InputFrom)
	for i, op := range gd.Operations {
		name := op.Name
		if name == "" {
			name = fmt.Sprintf("#%d", i)
		}
		td.InputFromSuggestions[name] = (name == gopd.InputFrom)
	}

	return o.createTemplate(templateEditGraph, td)
}

func (o *OpUI) showGraphs(vars map[string]string, input *OperatorIO) *OperatorIO {
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

func (o *OpUI) editConfig(vars map[string]string, input *OperatorIO) *OperatorIO {
	var d EditConfigData
	if !input.IsEmpty() {
		inBytes, err := input.GetBytes()
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, err.Error())
		}
		formInputQueryFormat, err := url.ParseQuery(string(inBytes))
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, err.Error())
		}
		formInput := utils.URLArgsToMap(formInputQueryFormat)
		if _, ok := formInput["SaveConfig"]; ok {
			err = o.cr.SetConfigFileContent(formInput["ConfigText"])
			if err != nil {
				return MakeOutputError(http.StatusInternalServerError, err.Error())
			}
		}
	}
	d.ConfigText = o.cr.GetConfigFileContent()

	return o.createTemplate(templateEditConfig, &d)
}

func (o *OpUI) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "edit":
		return o.editGraph(vars, input)
	case "config":
		return o.editConfig(vars, input)
	}
	return o.showGraphs(vars, input)
}

func (o *OpUI) GetFunctions() []string {
	return []string{"edit", "show", "config"}
}

func (o *OpUI) GetPossibleArgs(fn string) []string {
	return []string{"graph"}
}

func (o *OpUI) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

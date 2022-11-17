package freepsgraph

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/utils"
	"github.com/hannesrauhe/freepslib"
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
	GraphName            string
	GraphJSON            string
	Output               string
	Numop                int
}

type ShowGraphsData struct {
	Graphs    []string
	GraphJSON string
	Output    string
}

type EditConfigData struct {
	ConfigText string
	Output     string
}

//go:embed templates/*
var templates embed.FS

// NewHTMLUI creates a UI interface based on the inline template above
func NewHTMLUI(cr *utils.ConfigReader, graphEngine *GraphEngine) *OpUI {
	h := OpUI{ge: graphEngine, cr: cr}

	return &h
}

func (o *OpUI) createTemplate(templateFilePath string, templateData interface{}) *OperatorIO {
	t, err := template.ParseFS(templates, "templates/"+templateFilePath)
	if err != nil {
		// could in theory be any other error as well, but I don't want to parse strings
		return MakeOutputError(http.StatusNotFound, "No such template \"%v\"", templateFilePath)
	}
	tFooter, err := template.ParseFS(templates, "templates/footer.html")
	if err != nil {
		log.Println(err)
		return MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	var w bytes.Buffer
	err = t.Execute(&w, templateData)
	if err != nil {
		log.Println(err)
		return MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	err = tFooter.Execute(&w, nil)
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

	td := &TemplateData{OpSuggestions: map[string]bool{}, FnSuggestions: map[string]bool{}, ArgSuggestions: make(map[string]map[string]string), InputFromSuggestions: map[string]bool{}, GraphName: vars["graph"]}

	if input.IsEmpty() {
		gd, exists = o.ge.GetGraphDesc(td.GraphName)
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
			td.GraphName = formInput["GraphName"]
			if td.GraphName == "" {
				return MakeOutputError(http.StatusBadRequest, "Graph name cannot be empty")
			}
			err := o.ge.AddExternalGraph(td.GraphName, gd, "")
			if err != nil {
				return MakeOutputError(http.StatusBadRequest, err.Error())
			}
		}
		if _, ok := formInput["SaveTemp"]; ok {
			td.GraphName = formInput["GraphName"]
			if td.GraphName == "" {
				return MakeOutputError(http.StatusBadRequest, "Graph name cannot be empty")
			}
			err := o.ge.AddTemporaryGraph(td.GraphName, gd)
			if err != nil {
				return MakeOutputError(http.StatusBadRequest, err.Error())
			}
		}

		if _, ok := formInput["Execute"]; ok {
			err := o.ge.AddTemporaryGraph("UIgraph", gd)
			if err != nil {
				return MakeOutputError(http.StatusBadRequest, err.Error())
			}
			td.Output = "/graph/UIgraph"
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
			td.ArgSuggestions[k] = mod.GetArgSuggestions(gopd.Function, k, td.Args)
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

	return o.createTemplate(`editgraph.html`, td)
}

func (o *OpUI) showGraphs(vars map[string]string, input *OperatorIO) *OperatorIO {
	var d ShowGraphsData
	d.Graphs = make([]string, 0)
	for n := range o.ge.GetAllGraphDesc() {
		d.Graphs = append(d.Graphs, n)
	}
	sort.Strings(d.Graphs)
	if g, ok := vars["graph"]; ok {
		if gd, ok := o.ge.GetGraphDesc(g); ok {
			b, _ := json.MarshalIndent(gd, "", "  ")
			d.GraphJSON = string(b)
		}
	}

	return o.createTemplate(`showgraphs.html`, &d)
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

	return o.createTemplate(`editconfig.html`, &d)
}

func (o *OpUI) fritzDeviceList(vars map[string]string, input *OperatorIO) *OperatorIO {
	var devicelist freepslib.AvmDeviceList
	err := input.ParseJSON(&devicelist)
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, "Error when parsing Devicelist: %v", err)
	}
	return o.createTemplate(`fritzdevicelist.html`, &devicelist)
}

func (o *OpUI) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "":
		fallthrough
	case "showGraphs":
		return o.showGraphs(vars, input)
	case "edit":
		return o.editGraph(vars, input)
	case "config":
		return o.editConfig(vars, input)
	case "show":
		return o.showGraphs(vars, input)
	case "fritzdevicelist":
		return o.fritzDeviceList(vars, input)
	default:
		tdata := make(map[string]interface{})
		err := input.ParseJSON(&tdata)
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "Error when parsing input: %v", err)
		}
		return o.createTemplate(fn, &tdata)
	}
}

func (o *OpUI) GetFunctions() []string {
	return []string{"edit", "show", "config", "fritzdevicelist"}
}

func (o *OpUI) GetPossibleArgs(fn string) []string {
	return []string{"graph"}
}

func (o *OpUI) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

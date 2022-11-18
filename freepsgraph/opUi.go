package freepsgraph

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/hannesrauhe/freeps/utils"
	"github.com/hannesrauhe/freepslib"
	log "github.com/sirupsen/logrus"
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
	Quicklink            string
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
var embeddedFiles embed.FS

// NewHTMLUI creates a UI interface based on the inline template above
func NewHTMLUI(cr *utils.ConfigReader, graphEngine *GraphEngine) *OpUI {
	h := OpUI{ge: graphEngine, cr: cr}

	return &h
}

func (o *OpUI) getTemplateNames() []string {
	tlist := make([]string, 0)
	ftlist, _ := embeddedFiles.ReadDir("templates")
	for _, e := range ftlist {
		if e.IsDir() {
			continue
		}
		tlist = append(tlist, e.Name())
	}
	ftlist, _ = os.ReadDir(path.Join(o.cr.GetConfigDir(), "templates"))
	for _, e := range ftlist {
		if e.IsDir() {
			continue
		}
		tlist = append(tlist, e.Name())
	}
	return tlist
}

func (o *OpUI) isCustomTemplate(templateBaseName string) (bool, string) {
	pathInFS := "templates/" + templateBaseName
	configPath := path.Join(o.cr.GetConfigDir(), pathInFS)
	info, err := os.Stat(configPath)
	if err == nil && !info.IsDir() {
		return true, configPath
	}
	return false, pathInFS
}

func (o *OpUI) openWritableTemplateFile(templateBaseName string) (*os.File, error) {
	if templateBaseName == "" {
		return nil, fmt.Errorf("empty Template Name not allowd")
	}
	pathInFS := "templates/" + templateBaseName
	configPath := path.Join(o.cr.GetConfigDir(), pathInFS)
	err := os.MkdirAll(path.Dir(configPath), 0755)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(configPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
}

func (o *OpUI) deleteTemplateFile(templateBaseName string) error {
	if templateBaseName == "" {
		return fmt.Errorf("empty Template Name not allowd")
	}
	pathInFS := "templates/" + templateBaseName
	configPath := path.Join(o.cr.GetConfigDir(), pathInFS)
	return os.Remove(configPath)
}

func (o *OpUI) getTemplateBytes(templateBaseName string, logger *log.Entry) ([]byte, error) {
	if templateBaseName == "" {
		return nil, fmt.Errorf("empty Template Name not allowd")
	}
	isCustom, path := o.isCustomTemplate(templateBaseName)
	if isCustom {
		logger.Debugf("found template \"%v\" in config dir", templateBaseName)
		return os.ReadFile(path)
	}
	return embeddedFiles.ReadFile(path)
}

func (o *OpUI) parseTemplate(templateBaseName string, logger *log.Entry) (*template.Template, error) {
	isCustom, path := o.isCustomTemplate(templateBaseName)
	if isCustom {
		logger.Debugf("found template \"%v\" in config dir", templateBaseName)
		return template.ParseFiles(path)
	}
	return template.ParseFS(embeddedFiles, path)
}

func (o *OpUI) createTemplate(templateBaseName string, templateData interface{}, logger *log.Entry) *OperatorIO {
	t, err := o.parseTemplate(templateBaseName, logger)
	if err != nil {
		// could in theory be any other error as well, but I don't want to parse strings
		return MakeOutputError(http.StatusNotFound, "No such template \"%v\"", templateBaseName)
	}
	var w bytes.Buffer
	err = t.Execute(&w, templateData)
	if err != nil {
		logger.Error(err)
		return MakeOutputError(http.StatusInternalServerError, err.Error())
	}

	/* parse footer if requested template is html-file */
	if filepath.Ext(templateBaseName) == ".html" {
		tFooter, err := o.parseTemplate("footer.html", logger)
		if err != nil {
			logger.Errorf("Problem when opening template footer: %v", err)
			return MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		err = tFooter.Execute(&w, nil)
		if err != nil {
			logger.Println(err)
			return MakeOutputError(http.StatusInternalServerError, err.Error())
		}
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

func (o *OpUI) editGraph(vars map[string]string, input *OperatorIO, logger *log.Entry) *OperatorIO {
	var gd *GraphDesc
	var exists bool
	targetNum := 0

	td := &TemplateData{OpSuggestions: map[string]bool{}, FnSuggestions: map[string]bool{}, ArgSuggestions: make(map[string]map[string]string), InputFromSuggestions: map[string]bool{}, GraphName: vars["graph"]}

	if input.IsEmpty() {
		gd, exists = o.ge.GetGraphDesc(td.GraphName)
	}
	if !input.IsEmpty() || !exists {
		formInputQueryFormat, err := input.ParseFormData()
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
	td.Quicklink = gopd.ToQuicklink()
	return o.createTemplate(`editgraph.html`, td, logger)
}

func (o *OpUI) showGraphs(vars map[string]string, input *OperatorIO, logger *log.Entry) *OperatorIO {
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

	return o.createTemplate(`showgraphs.html`, &d, logger)
}

func (o *OpUI) editConfig(vars map[string]string, input *OperatorIO, logger *log.Entry) *OperatorIO {
	var d EditConfigData
	if !input.IsEmpty() {
		formInputQueryFormat, err := input.ParseFormData()
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

	return o.createTemplate(`editconfig.html`, &d, logger)
}

func (o *OpUI) fritzDeviceList(vars map[string]string, input *OperatorIO, logger *log.Entry) *OperatorIO {
	var devicelist freepslib.AvmDeviceList
	err := input.ParseJSON(&devicelist)
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, "Error when parsing Devicelist: %v", err)
	}
	return o.createTemplate(`fritzdevicelist.html`, &devicelist, logger)
}

func (o *OpUI) editTemplate(vars map[string]string, input *OperatorIO, logger *log.Entry) *OperatorIO {
	tname := vars["templateName"]

	if !input.IsEmpty() {
		f, err := input.ParseFormData()
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "Error when parsing input: %v", err)
		}
		tname = f.Get("templateName")
		if tname == "" {
			return MakeOutputError(http.StatusBadRequest, "Posted empty templateName")
		}

		if f.Get("templateCode") == "" {
			return MakeOutputError(http.StatusBadRequest, "Posted empty templateCode")
		}
		tf, err := o.openWritableTemplateFile(tname)
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "Error when trying to open template: %v", err)
		}
		defer tf.Close()
		_, err = tf.WriteString(f.Get("templateCode"))
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "Error when trying to write template: %v", err)
		}
	}

	b, _ := o.getTemplateBytes(tname, logger)
	tdata := make(map[string]interface{})
	tdata["templateName"] = tname
	tdata["templateCode"] = template.HTML(b)
	return o.createTemplate(`edittemplate.html`, tdata, logger)
}

func (o *OpUI) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	stdlogger := log.StandardLogger()
	logger := stdlogger.WithField("component", "UI")

	switch fn {
	case "":
		fallthrough
	case "showGraphs":
		return o.showGraphs(vars, input, logger)
	case "edit":
		return o.editGraph(vars, input, logger)
	case "config":
		return o.editConfig(vars, input, logger)
	case "show":
		return o.showGraphs(vars, input, logger)
	case "editTemplate":
		return o.editTemplate(vars, input, logger)
	case "deleteTemplate":
		err := o.deleteTemplateFile(vars["templateName"])
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "Error when deleting template: %v", err)
		}
		return MakeEmptyOutput()
	case "fritzdevicelist":
		return o.fritzDeviceList(vars, input, logger)
	default:
		tdata := make(map[string]interface{})
		err := input.ParseJSON(&tdata)
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "Error when parsing input: %v", err)
		}
		return o.createTemplate(fn, &tdata, logger)
	}
}

func (o *OpUI) GetFunctions() []string {
	return []string{"edit", "show", "config", "editTemplate", "deleteTemplate", "fritzdevicelist"}
}

func (o *OpUI) GetPossibleArgs(fn string) []string {
	return []string{"graph", "templateName"}
}

func (o *OpUI) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	r := map[string]string{}
	if arg == "templateName" {
		for _, tn := range o.getTemplateNames() {
			r[tn] = tn
		}
	}
	return r
}

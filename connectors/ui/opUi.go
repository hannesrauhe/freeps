package ui

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
	"strconv"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

const buttonPrefix string = "button_"

type OpUI struct {
	ge *freepsflow.FlowEngine
	cr *utils.ConfigReader
}

var _ base.FreepsBaseOperator = &OpUI{}

type TemplateData struct {
	Args                 map[string]string
	OpSuggestions        map[string]bool
	FnSuggestions        map[string]bool
	ArgSuggestions       map[string]map[string]string
	TagSuggestions       map[string]string
	InputFromSuggestions []string
	FlowName             string
	FlowDesc             *freepsflow.FlowDesc
	FlowJSON             string
	Output               string
	Numop                int
	Quicklink            string
	Error                string
}

type ShowFlowsData struct {
	Flows    []string
	FlowJSON string
	Output   string
}

type EditConfigData struct {
	ConfigText string
	Output     string
}

//go:embed templates/*
var embeddedFiles embed.FS

// NewHTMLUI creates a UI interface based on the inline template above
func NewHTMLUI(cr *utils.ConfigReader, flowEngine *freepsflow.FlowEngine) *OpUI {
	h := OpUI{ge: flowEngine, cr: cr}

	return &h
}

// GetName returns the name of the operator
func (o *OpUI) GetName() string {
	return "ui"
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

func (o *OpUI) getFileBytes(templateBaseName string, logger *log.Entry) ([]byte, error) {
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

func (o *OpUI) parseTemplate(ctx *base.Context, templateBaseName string) (*template.Template, error) {
	isCustom, path := o.isCustomTemplate(templateBaseName)
	t := template.New(templateBaseName).Funcs(o.createTemplateFuncMap(ctx))
	if isCustom {
		return t.ParseFiles(path)
	}
	return t.ParseFS(embeddedFiles, path)
}

func (o *OpUI) createOutput(ctx *base.Context, templateBaseName string, templateData interface{}, logger *log.Entry, withFooter bool) *base.OperatorIO {
	/* parse as template if basename is html */
	if filepath.Ext(templateBaseName) == ".html" || filepath.Ext(templateBaseName) == ".htm" {
		t, err := o.parseTemplate(ctx, templateBaseName)
		if err != nil {
			// could be any other error code, but I don't want to parse error strings
			return base.MakeOutputError(http.StatusBadRequest, "Error with template \"%v\": \"%v\"", templateBaseName, err.Error())
		}
		var w bytes.Buffer
		styles, err := o.getFileBytes("style.html", logger)
		if err != nil {
			logger.Error(err)
			return base.MakeInternalServerErrorOutput(err)
		}
		w.WriteString(fmt.Sprintf("<title>%v</title>", templateBaseName))
		w.Write(styles)
		err = t.Execute(&w, templateData)
		if err != nil {
			logger.Error(err)
			return base.MakeInternalServerErrorOutput(err)
		}

		if withFooter {
			tFooter, err := o.parseTemplate(ctx, "footer.html")
			if err != nil {
				logger.Errorf("Problem when opening template footer: %v", err)
				return base.MakeInternalServerErrorOutput(err)
			}

			var fdata struct {
				Version   string
				StartedAt string
			}
			fdata.Version = utils.BuildVersion()
			fdata.StartedAt = utils.StartTimestamp.Format(time.RFC1123)
			err = tFooter.Execute(&w, &fdata)
			if err != nil {
				logger.Error(err)
				return base.MakeInternalServerErrorOutput(err)
			}
		}
		return base.MakeByteOutputWithContentType(w.Bytes(), "text/html; charset=utf-8")
	}

	// return file directly if not html:
	b, err := o.getFileBytes(templateBaseName, logger)
	if err != nil {
		// could be an internal error, but I don't want to parse error strings
		return base.MakeOutputError(http.StatusNotFound, "Error when reading plain file \"%v\": \"%v\"", templateBaseName, err.Error())
	}
	return base.MakeByteOutput(b)
}

func (o *OpUI) buildPartialFlow(formInput map[string]string) *freepsflow.FlowDesc {
	standardOP := []freepsflow.FlowOperationDesc{{Operator: "flow", Function: "storeUI", Arguments: base.MakeEmptyFunctionArguments()}}

	gd := &freepsflow.FlowDesc{}
	v, ok := formInput["FlowJSON"]
	if ok {
		json.Unmarshal([]byte(v), gd)
	}
	opNum, _ := formInput["selectednumop"]
	targetNum, _ := strconv.Atoi(opNum)
	if targetNum < 0 {
		targetNum = 0
	}
	if gd.Operations == nil || len(gd.Operations) == 0 {
		gd.Operations = standardOP[:]
	}
	if len(gd.Operations) <= targetNum {
		gd.Operations = append(gd.Operations, standardOP...)
	}
	gopd := &gd.Operations[targetNum]
	if gopd.Arguments == nil {
		gopd.Arguments = base.MakeEmptyFunctionArguments()
	}
	for k, v := range formInput {
		if utils.StringStartsWith(k, "arg.") {
			gopd.Arguments.Append(k[4:], v)
		} else if k == "newArg" && v != "" {
			gopd.Arguments.Append(v, "")
		} else if k == "delArg" {
			//TODO: implement delete
		} else if k == "addTag" && v != "" {
			gd.AddTags(v)
		} else if k == "delTag" {
			gd.RemoveTag(v)
		} else if k == "op" {
			gopd.Operator = v
		} else if k == "fn" {
			gopd.Function = v
		} else if k == "inputFrom" {
			gopd.InputFrom = v
		} else if k == "argumentsFrom" {
			gopd.ArgumentsFrom = v
		} else if k == "executeOnSuccessOf" {
			gopd.ExecuteOnSuccessOf = v
		} else if k == "executeOnFailOf" {
			gopd.ExecuteOnFailOf = v
		} else if k == "useMainArgs" {
			gopd.UseMainArgs = utils.ParseBool(v)
		} else if k == "opName" && len(v) > 0 && !utils.StringStartsWith(v, "#") {
			if gopd.Name == "" {
				gopd.Name = fmt.Sprintf("#%d", targetNum)
			}
			gd.RenameOperation(gopd.Name, v)
		} else if k == "flowOutput" {
			gd.OutputFrom = v
		}
	}

	/* modify operation list: adding and deleting */

	if newOp, ok := formInput["newOp"]; ok {
		newNum, err := strconv.Atoi(newOp)
		if err == nil && newNum <= len(gd.Operations) && newNum >= 0 {
			if newNum == 0 {
				gd.Operations = append(standardOP, gd.Operations...)
			} else if newNum == len(gd.Operations) {
				gd.Operations = append(gd.Operations, standardOP...)
			} else {
				gd.Operations = append(gd.Operations[:newNum+1], gd.Operations[newNum:]...)
				gd.Operations[newNum] = standardOP[0]
			}
		}
	}
	if delOp, ok := formInput["deleteOp"]; ok {
		delNum, err := strconv.Atoi(delOp)
		if err == nil && delNum < len(gd.Operations) && delNum >= 0 {
			if delNum == len(gd.Operations)-1 {
				gd.Operations = gd.Operations[:delNum]
			} else if delNum == 0 {
				gd.Operations = gd.Operations[1:]
			} else {
				gd.Operations = append(gd.Operations[:delNum], gd.Operations[delNum+1:]...)
			}
		}
	}

	return gd
}

func (o *OpUI) editFlow(ctx *base.Context, vars map[string]string, input *base.OperatorIO, logger *log.Entry, tmpl string) *base.OperatorIO {
	var gd *freepsflow.FlowDesc
	var exists bool
	targetNum := 0

	td := &TemplateData{OpSuggestions: map[string]bool{}, FnSuggestions: map[string]bool{}, ArgSuggestions: make(map[string]map[string]string), InputFromSuggestions: []string{}, FlowName: vars["flow"], TagSuggestions: map[string]string{}}

	if input.IsEmpty() {
		gd, exists = o.ge.GetFlowDesc(td.FlowName)
	}
	if !input.IsEmpty() || !exists {
		formInputQueryFormat, err := input.ParseFormData()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "%v", err.Error())
		}
		formInput := utils.URLArgsToMap(formInputQueryFormat)
		gd = o.buildPartialFlow(formInput)
		opNum, ok := formInput["numop"]
		if !ok {
			opNum, _ = formInput["selectednumop"]
		}
		targetNum, _ = strconv.Atoi(opNum)
		if targetNum < 0 || targetNum >= len(gd.Operations) {
			targetNum = 0
		}

		if _, ok := formInput["SaveFlow"]; ok {
			td.FlowName = formInput["FlowName"]
			if td.FlowName == "" {
				return base.MakeOutputError(http.StatusBadRequest, "Flow name cannot be empty")
			}
			err := o.ge.AddFlow(ctx, td.FlowName, *gd, true)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, "%v", err.Error())
			}
		}
		if _, ok := formInput["SaveTemp"]; ok {
			td.FlowName = formInput["FlowName"]
			if td.FlowName == "" {
				td.FlowName = ctx.GetID()
			}
			err := freepsstore.StoreFlow(td.FlowName, *gd, ctx)
			if err.IsError() {
				return err
			}
		}

		if _, ok := formInput["Execute"]; ok {
			td.FlowName = formInput["FlowName"]
			if td.FlowName == "" {
				td.FlowName = ctx.GetID()
			}
			err := freepsstore.StoreFlow(td.FlowName, *gd, ctx)
			if err.IsError() {
				return err
			}
			td.Output = "/flowBuilder/ExecuteFlowFromStore?flowName=" + td.FlowName
		}
	}

	// try to parse the FlowDesc and use normalized version for FlowDesc if available
	completeGD, err := gd.GetCompleteDesc("temp", o.ge)
	if err == nil {
		td.FlowDesc = completeGD
	} else {
		td.Error = err.Error()
		td.FlowDesc = gd
	}

	b, _ := json.MarshalIndent(gd, "", "  ")
	td.FlowJSON = string(b)
	gopd := &gd.Operations[targetNum]
	td.Numop = targetNum
	td.Args = gopd.Arguments.GetOriginalCaseMapJoined()
	for _, k := range o.ge.GetOperators() {
		td.OpSuggestions[k] = (k == gopd.Operator)
	}

	if o.ge.HasOperator(gopd.Operator) {
		mod := o.ge.GetOperator(gopd.Operator)
		for _, k := range mod.GetFunctions() {
			td.FnSuggestions[k] = (k == gopd.Function)
		}
		for _, k := range mod.GetPossibleArgs(gopd.Function) {
			td.ArgSuggestions[k] = mod.GetArgSuggestions(gopd.Function, k, base.NewFunctionArguments(td.Args))
		}
		for _, k := range gopd.Arguments.GetOriginalKeys() {
			td.ArgSuggestions[k] = mod.GetArgSuggestions(gopd.Function, k, base.NewFunctionArguments(td.Args))
		}
	}

	td.InputFromSuggestions = []string{freepsflow.ROOT_SYMBOL}
	for i, op := range gd.Operations {
		if i >= td.Numop {
			continue
		}
		name := op.Name
		if name == "" {
			name = fmt.Sprintf("#%d", i)
		}
		td.InputFromSuggestions = append(td.InputFromSuggestions, name)
	}
	td.TagSuggestions = o.ge.GetTags()
	for _, t := range gd.Tags {
		delete(td.TagSuggestions, t)
	}
	td.Quicklink = gopd.ToQuicklink()
	return o.createOutput(ctx, tmpl, td, logger, true)
}

func (o *OpUI) editTemplate(ctx *base.Context, vars map[string]string, input *base.OperatorIO) *base.OperatorIO {
	tname := vars["templateName"]

	if !input.IsEmpty() {
		f, err := input.ParseFormData()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Error when parsing input: %v", err)
		}
		tname = f.Get("templateName")
		if tname == "" {
			return base.MakeOutputError(http.StatusBadRequest, "Posted empty templateName")
		}

		if f.Get("templateCode") == "" {
			return base.MakeOutputError(http.StatusBadRequest, "Posted empty templateCode")
		}
		tf, err := o.openWritableTemplateFile(tname)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Error when trying to open template: %v", err)
		}
		defer tf.Close()
		_, err = tf.WriteString(f.Get("templateCode"))
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Error when trying to write template: %v", err)
		}
	}

	// TODO(HR): needs to be cleaned up: ctx/logger handling
	logger := ctx.GetLogger().WithField("", "")
	b, _ := o.getFileBytes(tname, logger)
	tdata := make(map[string]interface{})
	tdata["templateName"] = tname
	tdata["templateCode"] = template.HTML(b)
	return o.createOutput(ctx, `edittemplate.html`, tdata, logger, true)
}

func (o *OpUI) simpleTile(vars map[string]string, input *base.OperatorIO, ctx *base.Context) *base.OperatorIO {
	tdata := make(map[string]interface{})

	buttons := make(map[string]string)
	for k, v := range vars {
		if utils.StringStartsWith(k, buttonPrefix) {
			buttons[k[len(buttonPrefix):]] = v
		}
	}

	tdata["buttons"] = buttons
	tdata["input"] = input.Output
	tdata["arguments"] = vars
	tdata["status"] = vars["header"]
	tdata["status_error"] = ""
	tdata["status_ok"] = ""
	if !input.IsEmpty() {
		formdata, err := input.ParseFormData()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Error when parsing input: %v", err)
		}
		flowName := formdata.Get("ExecuteFlow")
		if flowName != "" {
			out := o.ge.ExecuteFlow(ctx, flowName, base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput())
			if out.IsError() {
				tdata["status_error"] = flowName
			} else {
				tdata["status_ok"] = flowName
			}
		}
	}

	templateName, ok := vars["templateName"]
	if !ok {
		templateName = "simpleTile.html"
	}
	return o.createOutput(ctx, templateName, tdata, ctx.GetLogger().WithField("component", "UIsimpleTile"), true)
}

func (o *OpUI) Execute(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	return o.ExecuteOld(ctx, fn, fa.GetOriginalCaseMapJoined(), input)
}

func (o *OpUI) ExecuteOld(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	logger := ctx.GetLogger().WithField("component", "UI")
	withFooter := !utils.ParseBool(args["noFooter"])
	delete(args, "noFooter")

	//TODO(HR): ensure that args are lowercase
	lowercaseArgs := utils.KeysToLower(args)

	switch fn {
	case "", "home":
		return o.editFlow(ctx, args, input, logger, "home.html")
	case "edit", "editFlow":
		return o.editFlow(ctx, args, input, logger, "editflow.html")
	case "editTemplate":
		return o.editTemplate(ctx, args, input)
	case "deleteTemplate":
		err := o.deleteTemplateFile(args["templateName"])
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Error when deleting template: %v", err)
		}
		return base.MakeEmptyOutput()
	case "simpleTile":
		return o.simpleTile(args, input, ctx)
	default:
		tdata := make(map[string]interface{})

		tdata["arguments"] = lowercaseArgs

		if input.IsFormData() {
			formInput, err := input.ParseFormData()
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, "Error when parsing input: %v", err)
			}
			opName := formInput.Get("ExecuteOperator")
			flowName := formInput.Get("ExecuteFlow")
			executeWithArgs, err := base.NewFunctionArgumentsFromURLQuery(formInput.Get("ExecuteArgs"))
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, "Error when parsing ExecuteArgs (\"%v\") in request: %v", formInput.Get("ExecuteArgs"), err)
			}
			for k, v := range formInput {
				if utils.StringStartsWith(k, "ExecuteArg.") {
					executeWithArgs.Append(k[11:], v...)
				}
			}
			executeWithInput := base.MakeEmptyOutput()
			if flowName != "" {
				tdata["response"] = o.ge.ExecuteFlow(ctx, flowName, executeWithArgs, executeWithInput)
			} else if opName != "" {
				fnName := formInput.Get("ExecuteFunction")
				tdata["response"] = o.ge.ExecuteOperatorByName(ctx, opName, fnName, executeWithArgs, executeWithInput)
			}
			tdata["input"] = formInput
		} else if !input.IsEmpty() {
			// Note: in order to have the UI show values as if they were printed as JSON, they are parsed once
			// This would lead to accessing the objects directly (MarshallJSON would not be called):
			// if input.IsObject() {
			// 	tdata["input"] = input.Output
			// }
			tinput := make(map[string]interface{})
			err := input.ParseJSON(&tinput)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, "Error when parsing input: %v", err)
			}
			tdata["input"] = tinput
		}
		tdata["selfPath"] = "/ui/" + fn
		tdata["selfURL"] = "/ui/" + fn + "?" + utils.MapToURLArgs(lowercaseArgs).Encode()
		return o.createOutput(ctx, fn, &tdata, logger, withFooter)
	}
}

func (o *OpUI) GetFunctions() []string {
	r := o.getTemplateNames()
	return append(r, "edit", "show", "config", "editTemplate", "deleteTemplate", "fritzdevicelist", "simpleTile")
}

func (o *OpUI) GetPossibleArgs(fn string) []string {
	switch fn {
	case "editFlow":
		return []string{"flow"}
	case "editTemplate", "deleteTemplate":
		return []string{"templateName"}
	case "simpleTile":
		return []string{"header", "button_On", "button_Off", "templateName"}
	}
	return []string{"noFooter"}
}

func (o *OpUI) GetArgSuggestions(fn string, arg string, otherArgs base.FunctionArguments) map[string]string {
	r := map[string]string{}
	if arg == "templateName" {
		for _, tn := range o.getTemplateNames() {
			r[tn] = tn
		}
	}
	if fn == "simpleTile" && utils.StringStartsWith(arg, buttonPrefix) {
		agd := o.ge.GetAllFlowDesc()
		flows := make(map[string]string)
		for n := range agd {
			flows[n] = n
		}
		return flows
	}
	return r
}

// StartListening (noOp)
func (o *OpUI) StartListening(ctx *base.Context) {
}

// Shutdown (noOp)
func (o *OpUI) Shutdown(ctx *base.Context) {
}

// GetHook (noOp)
func (o *OpUI) GetHook() interface{} {
	return nil
}

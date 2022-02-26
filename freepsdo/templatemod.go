package freepsdo

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/hannesrauhe/freeps/utils"
)

type TemplateModConfig struct {
	Templates        map[string]*Template
	TemplatesFromUrl string
}

var DefaultConfig = TemplateModConfig{}

type TemplateAction struct {
	Mod             string                 `json:",omitempty"`
	Fn              string                 `json:",omitempty"`
	Args            map[string]interface{} `json:",omitempty"`
	FwdTemplateName string                 `json:",omitempty"`
	FwdTemplate     *Template              `json:",omitempty"`
}

type Template struct {
	Actions []TemplateAction
}

type TemplateMod struct {
	Mods               map[string]Mod
	Config             TemplateModConfig
	TemporaryTemplates map[string]*Template
	ExternalTemplates  map[string]*Template
	cr                 *utils.ConfigReader
}

var _ Mod = &TemplateMod{}

func NewTemplateMod(cr *utils.ConfigReader) *TemplateMod {
	tmc := DefaultConfig
	err := cr.ReadSectionWithDefaults("TemplateMod", &tmc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	mods := map[string]Mod{}
	mods["curl"] = &CurlMod{}
	mods["echo"] = &EchoMod{}
	mods["script"] = NewScriptMod(cr)
	mods["fritz"] = NewFritzMod(cr)
	mods["flux"] = NewFluxMod(cr)
	mods["raspistill"] = &RaspistillMod{}

	if tmc.Templates == nil {
		tmc.Templates = map[string]*Template{}
	}
	ext := map[string]*Template{}
	byt := utils.ReadBytesFromUrl(tmc.TemplatesFromUrl)
	if len(byt) > 0 {
		json.Unmarshal(byt, &ext)
	}

	tm := &TemplateMod{Mods: mods, Config: tmc, ExternalTemplates: ext, cr: cr,
		TemporaryTemplates: map[string]*Template{"_last": &Template{Actions: []TemplateAction{{Mod: "echo", Fn: "hello"}}}}}
	mods["template"] = tm
	mods["system"] = NewSystemeMod(tm)
	return tm
}

func (m *TemplateMod) DoWithJSON(templateName string, jsonStr []byte, jrw *ResponseCollector) {
	template, exists := m.GetTemplate(templateName)
	if !exists {
		jrw.WriteError(http.StatusNotFound, "template %v unknown", templateName)
		return
	}
	if len(template.Actions) == 0 {
		jrw.WriteError(http.StatusNotExtended, "template %v has no actions", templateName)
		return
	}
	m.ExecuteTemplateWithAdditionalArgs(template, jsonStr, jrw)
}

func (m *TemplateMod) GetFunctions() []string {
	t := m.GetAllTemplates(false)
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	return keys
}

func (m *TemplateMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *TemplateMod) GetArgSuggestions(fn string, arg string) map[string]string {
	ret := map[string]string{}
	return ret
}

func (m *TemplateMod) ExecuteTemplateWithAdditionalArgs(template *Template, jsonStr []byte, jrw *ResponseCollector) {
	for _, t := range template.Actions {
		m.ExecuteTemplateActionWithAdditionalArgs(&t, jsonStr, jrw.Clone())
	}
}

func (m *TemplateMod) ExecuteTemplateActionWithAdditionalArgs(t *TemplateAction, moreJsonArgs []byte, jrw *ResponseCollector) {
	jrw.SetContext(t)
	mod, modExists := m.Mods[t.Mod]
	if !modExists {
		jrw.WriteError(http.StatusNotFound, "module %v unknown", t.Mod)
		return
	}

	copiedArgs := map[string]interface{}{}
	for k, v := range t.Args {
		copiedArgs[k] = v
	}
	err := json.Unmarshal(moreJsonArgs, &copiedArgs)
	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, "error when merging json values with json string \"%q\": %v", moreJsonArgs, err)
		return
	}
	jsonStr, err := json.Marshal(copiedArgs)
	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, "error when merging arguments: %v", err)
		return
	}
	mod.DoWithJSON(t.Fn, jsonStr, jrw)
	m.TemporaryTemplates["_last"] = &Template{Actions: []TemplateAction{{Mod: t.Mod, Fn: t.Fn, Args: copiedArgs}}}
	if len(t.FwdTemplateName) > 0 {
		o, err := jrw.GetMarshalledOutput()
		if err == nil {
			m.DoWithJSON(t.FwdTemplateName, o, jrw)
		}
	} else if t.FwdTemplate != nil {
		o, err := jrw.GetMarshalledOutput()
		if err == nil {
			m.ExecuteTemplateWithAdditionalArgs(t.FwdTemplate, o, jrw)
		}
	}
	jrw.WriteSuccess()
}

func (m *TemplateMod) ExecuteTemplateAction(ta *TemplateAction, jrw *ResponseCollector) {
	m.ExecuteTemplateActionWithAdditionalArgs(ta, []byte("{}"), jrw)
}

func (m *TemplateMod) ExecuteModWithJson(mod string, fn string, jsonStr []byte, jrw *ResponseCollector) {
	ta := TemplateAction{Mod: mod, Fn: fn}
	json.Unmarshal(jsonStr, &ta.Args)
	m.ExecuteTemplateAction(&ta, jrw)
}

func (m *TemplateMod) GetTemplate(templateName string) (*Template, bool) {
	template, exists := m.TemporaryTemplates[templateName]
	if exists {
		return template, true
	}
	template, exists = m.Config.Templates[templateName]
	if exists {
		return template, true
	}
	template, exists = m.ExternalTemplates[templateName]
	return template, exists
}

func (m *TemplateMod) GetAllTemplates(includeTemp bool) map[string]*Template {
	retMap := map[string]*Template{}
	if includeTemp {
		for k, v := range m.TemporaryTemplates {
			retMap[k] = v
		}
	}
	for k, v := range m.ExternalTemplates {
		retMap[k] = v
	}
	for k, v := range m.Config.Templates {
		retMap[k] = v
	}
	return retMap
}

func (m *TemplateMod) GetTemporaryTemplateAction(ID string) *TemplateAction {
	tpl, ok := m.TemporaryTemplates["_"+ID]
	if !ok {
		tpl = &Template{Actions: make([]TemplateAction, 1)}
		m.TemporaryTemplates["_"+ID] = tpl
	}
	return &tpl.Actions[0]
}

func (m *TemplateMod) RemoveTemporaryTemplate(ID string) {
	delete(m.TemporaryTemplates, "_"+ID)
}

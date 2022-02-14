package freepsdo

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/hannesrauhe/freeps/utils"
)

type TemplateModConfig map[string]Template

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
	Mods      map[string]Mod
	Templates TemplateModConfig
}

var _ Mod = &TemplateMod{}

func NewTemplateMod(cr *utils.ConfigReader) *TemplateMod {
	tmc := DefaultConfig
	err := cr.ReadSectionWithDefaults("templates", &tmc)
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
	mods["fritz"] = NewFritzMod(cr)
	mods["flux"] = NewFluxMod(cr)
	mods["raspistill"] = &RaspistillMod{}
	tm := &TemplateMod{Mods: mods, Templates: tmc}
	mods["template"] = tm
	return tm
}

func (m *TemplateMod) DoWithJSON(templateName string, jsonStr []byte, jrw *ResponseCollector) {
	template, exists := m.Templates[templateName]
	if !exists {
		jrw.WriteError(http.StatusNotFound, "template %v unknown", templateName)
		return
	}
	if len(template.Actions) == 0 {
		jrw.WriteError(http.StatusNotExtended, "template %v has no actions", templateName)
		return
	}
	m.ExecuteTemplateWithAdditionalArgs(&template, jsonStr, jrw)
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
	jsonStr, err := utils.OverwriteValuesWithJson(moreJsonArgs, copiedArgs)

	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, "error when merging json values with json string \"%q\": %v", moreJsonArgs, err)
		return
	}
	mod.DoWithJSON(t.Fn, jsonStr, jrw)
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

func (m *TemplateMod) ExecuteModWithJson(mod string, fn string, jsonStr []byte, jrw *ResponseCollector) {
	ta := TemplateAction{Mod: mod, Fn: fn}
	json.Unmarshal(jsonStr, &ta.Args)
	m.ExecuteTemplateActionWithAdditionalArgs(&ta, []byte("{}"), jrw)
}

package freepsdo

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/utils"
)

type TemplateModConfig struct {
	Templates        map[string]*Template
	TemplatesFromUrl string
	Verbose          bool
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
	Actions    []TemplateAction
	OutputMode OutputModeT `json:",omitempty"`
}

type TemplateMod struct {
	Mods               map[string]Mod
	Config             TemplateModConfig
	TemporaryTemplates map[string]*Template
	ExternalTemplates  map[string]*Template
	Cache              map[string][]byte
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
	mods["eval"] = &EvalMod{}
	mods["telegram"] = NewTelegramBot(cr)
	mods["script"] = NewScriptMod(cr)
	mods["fritz"] = NewFritzMod(cr)
	mods["flux"] = NewFluxMod(cr)
	mods["mutt"] = NewMuttMod(cr)
	mods["raspistill"] = &RaspistillMod{}

	if tmc.Templates == nil {
		tmc.Templates = map[string]*Template{}
	}
	ext := map[string]*Template{}
	byt := utils.ReadBytesFromUrl(tmc.TemplatesFromUrl)
	if len(byt) > 0 {
		json.Unmarshal(byt, &ext)
	}

	tm := &TemplateMod{Mods: mods, Config: tmc, ExternalTemplates: ext, cr: cr, Cache: map[string][]byte{},
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

func (m *TemplateMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	ret := map[string]string{}
	return ret
}

func (m *TemplateMod) ExecuteTemplateWithAdditionalArgs(template *Template, jsonStr []byte, jrw *ResponseCollector) {
	jrw.SetOutputMode(template.OutputMode)
	for _, t := range template.Actions {
		m.ExecuteTemplateActionWithAdditionalArgs(&t, jsonStr, jrw.Clone())
	}
}

func (m *TemplateMod) ExecuteTemplateActionWithAdditionalArgs(t *TemplateAction, moreJsonArgs []byte, jrw *ResponseCollector) {
	startTime := time.Now()

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
	if !jrw.isSubtreeFinished() {
		jrw.WriteSuccess()
	}
	if jrw.IsRoot() {
		m.TemporaryTemplates["_last"] = &Template{Actions: []TemplateAction{{Mod: t.Mod, Fn: t.Fn, Args: copiedArgs}}}
		m.Cache["_last"] = jrw.GetResponseTree()
		if m.Config.Verbose {
			log.Printf("Executed %v in %ds; Status: %v; triggered by: %v", *t, time.Now().Unix()-startTime.Unix(), jrw.GetStatusCode(), jrw.GetCreator())
		}
	}
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

func (m *TemplateMod) SaveTemplateAction(templateName string, ta *TemplateAction) error {
	template, exists := m.Config.Templates[templateName]
	if !exists {
		m.Config.Templates[templateName] = &Template{Actions: make([]TemplateAction, 0, 1)}
		template = m.Config.Templates[templateName]
	}
	template.Actions = append(template.Actions, *ta)
	m.cr.WriteSection("TemplateMod", m.Config, true)
	return nil
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

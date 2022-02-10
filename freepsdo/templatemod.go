package freepsdo

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/hannesrauhe/freeps/utils"
)

type TemplateModConfig map[string]Template

var DefaultConfig = TemplateModConfig{}

type TemplateAction struct {
	Mod          string
	Fn           string
	Args         map[string]interface{}
	NextTemplate string
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

func (m *TemplateMod) DoWithJSON(templateName string, jsonStr []byte, jrw *JsonResponse) {
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

func (m *TemplateMod) ExecuteTemplate(template *Template, jrw *JsonResponse) {
	m.ExecuteTemplateWithAdditionalArgs(template, []byte("{}"), jrw)
}

func (m *TemplateMod) ExecuteTemplateWithAdditionalArgs(template *Template, moreJsonArgs []byte, jrw *JsonResponse) {
	for i, t := range template.Actions {
		jrw := jrw.Clone(strconv.Itoa(i))
		m.ExecuteTemplateActionWithAdditionalArgs(&t, moreJsonArgs, jrw)
	}
}

func (m *TemplateMod) ExecuteTemplateActionWithAdditionalArgs(t *TemplateAction, moreJsonArgs []byte, jrw *JsonResponse) {
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
}

func (m *TemplateMod) ExecuteModWithJson(mod string, fn string, jsonStr []byte, jrw *JsonResponse) {
	cjrw := jrw.Clone(mod + "#" + fn)
	ta := TemplateAction{Mod: mod, Fn: fn}
	json.Unmarshal(jsonStr, &ta.Args)
	m.ExecuteTemplateActionWithAdditionalArgs(&ta, []byte("{}"), cjrw)
}

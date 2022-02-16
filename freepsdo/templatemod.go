package freepsdo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/hannesrauhe/freeps/utils"
)

type TemplateModConfig map[string]Template

var DefaultConfig = TemplateModConfig{}

type TemplateAction struct {
	Mod  string
	Fn   string
	Args map[string]interface{}
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
	mods["script"] = NewScriptMod(cr)
	mods["fritz"] = NewFritzMod(cr)
	mods["flux"] = NewFluxMod(cr)
	mods["raspistill"] = &RaspistillMod{}
	tm := &TemplateMod{Mods: mods, Templates: tmc}
	mods["template"] = tm
	return tm
}

func (m *TemplateMod) DoWithJSON(templateName string, jsonStr []byte, w http.ResponseWriter) {
	template, exists := m.Templates[templateName]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "template %v unknown", templateName)
		return
	}
	if len(template.Actions) == 0 {
		w.WriteHeader(http.StatusNotExtended)
		fmt.Fprintf(w, "template %v has no actions", templateName)
		return
	}
	m.ExecuteTemplateWithAdditionalArgs(&template, jsonStr, w)
}

func (m *TemplateMod) ExecuteTemplate(template *Template, w http.ResponseWriter) {
	m.ExecuteTemplateWithAdditionalArgs(template, []byte("{}"), w)
}

func (m *TemplateMod) ExecuteTemplateWithAdditionalArgs(template *Template, moreJsonArgs []byte, w http.ResponseWriter) {
	for _, t := range template.Actions {
		mod, modExists := m.Mods[t.Mod]
		if !modExists {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "module %v unknown", t.Mod)
			return
		}

		copiedArgs := map[string]interface{}{}
		for k, v := range t.Args {
			copiedArgs[k] = v
		}
		jsonStr, err := utils.OverwriteValuesWithJson(moreJsonArgs, copiedArgs)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error when merging json values with json string \"%q\": %v", moreJsonArgs, err)
			return
		}
		mod.DoWithJSON(t.Fn, jsonStr, w)
	}
}

func (m *TemplateMod) ExecuteModWithJson(mod string, fn string, jsonStr []byte, w http.ResponseWriter) {
	ta := TemplateAction{Mod: mod, Fn: fn}
	json.Unmarshal(jsonStr, &ta.Args)
	actions := []TemplateAction{ta}
	t := Template{Actions: actions}
	m.ExecuteTemplate(&t, w)
}

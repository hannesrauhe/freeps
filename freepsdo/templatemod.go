package freepsdo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/hannesrauhe/freeps/utils"
)

type TemplateModConfig struct {
	url string
}

var DefaultConfig = TemplateModConfig{url: "https://raw.githubusercontent.com/hannesrauhe/freeps/freepsd/freepsdo/templates.json"}

type TemplateAction struct {
	Mod      string
	Fn       string
	Args     map[string][]string
	JsonArgs map[string]interface{}
}

type Template struct {
	Actions []TemplateAction
}

type TemplateMod struct {
	Mods      map[string]Mod
	Templates map[string]Template
}

func NewTemplateMod(cr *utils.ConfigReader) *TemplateMod {
	tmc := DefaultConfig
	err := cr.ReadSectionWithDefaults("templatemod", &tmc)
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
	tm := &TemplateMod{Mods: mods, Templates: TemplatesFromUrl(tmc.url)}
	mods["template"] = tm
	return tm
}

func TemplatesFromUrl(url string) map[string]Template {
	t := map[string]Template{}
	c := http.Client{}
	resp, err := c.Get(url)
	if err != nil {
		log.Printf("Error when reading from %v: %v", url, err)
		return t
	}
	if resp.StatusCode > 300 {
		log.Printf("Error when reading from %v: Status code %v", url, resp.StatusCode)
		return t
	}
	byt, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error when reading from %v: %v", url, err)
		return t
	}
	err = json.Unmarshal(byt, &t)
	if err != nil {
		log.Printf("Error when parsing json: %v\n %q", err, byt)
	}

	return t
}

func TemplatesFromFile(path string) map[string]Template {
	t := map[string]Template{}
	byt, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("Error when reading from %v: %v", path, err)
		return t
	}
	err = json.Unmarshal(byt, &t)
	if err != nil {
		log.Printf("Error when parsing json: %v\n %q", err, byt)
	}
	return t
}

func (m *TemplateMod) Do(templateName string, args map[string][]string, w http.ResponseWriter) {
	m.DoWithJSON(templateName, []byte{}, w)
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
	m.ExecuteTemplateWithAdditionalArgs(template, []byte{}, w)
}

func (m *TemplateMod) ExecuteTemplateWithAdditionalArgs(template *Template, moreJsonArgs []byte, w http.ResponseWriter) {
	for _, t := range template.Actions {
		mod, modExists := m.Mods[t.Mod]
		if !modExists {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "module %v unknown", t.Mod)
			return
		}
		if t.Args != nil && len(t.Args) > 0 {
			mod.Do(t.Fn, t.Args, w)
		} else {
			copiedArgs := map[string]interface{}{}
			for k, v := range t.JsonArgs {
				copiedArgs[k] = v
			}
			jsonStr, _ := utils.OverwriteValuesWithJson(moreJsonArgs, copiedArgs)
			mod.DoWithJSON(t.Fn, jsonStr, w)
		}
	}
}

func (m *TemplateMod) ExecuteModWithArgs(mod string, fn string, args map[string][]string, w http.ResponseWriter) {
	ta := TemplateAction{Mod: mod, Fn: fn, Args: args}
	actions := []TemplateAction{ta}
	t := Template{Actions: actions}
	m.ExecuteTemplate(&t, w)
}

func (m *TemplateMod) ExecuteModWithJson(mod string, fn string, jsonStr []byte, w http.ResponseWriter) {
	ta := TemplateAction{Mod: mod, Fn: fn, JsonArgs: map[string]interface{}{}}
	json.Unmarshal(jsonStr, &ta.JsonArgs)
	actions := []TemplateAction{ta}
	t := Template{Actions: actions}
	m.ExecuteTemplate(&t, w)
}

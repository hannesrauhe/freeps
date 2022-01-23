package restonatorx

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/hannesrauhe/freeps/utils"
)

type TemplateAction struct {
	Mod  string
	Fn   string
	Args map[string][]string
}

type Template struct {
	Actions []TemplateAction
}

type TemplateMod struct {
	Mods      map[string]RestonatorMod
	Templates map[string]Template
}

func NewTemplateModFromUrl(url string, mods map[string]RestonatorMod) *TemplateMod {
	c := http.Client{}
	resp, err := c.Get(url)
	if err != nil {
		return nil
	}
	byt, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	var t map[string]Template
	err = json.Unmarshal(byt, &t)
	if err != nil {
		log.Printf("Error when parsing json: %v\n %q", err, byt)
	}
	return &TemplateMod{Mods: mods, Templates: t}
}

func NewTemplateModFromFile(path string, mods map[string]RestonatorMod) *TemplateMod {
	byt, err := ioutil.ReadFile(path)
	if err != nil {
		return nil
	}
	var t map[string]Template
	err = json.Unmarshal(byt, &t)
	if err != nil {
		log.Printf("Error when parsing json: %v\n %q", err, byt)
	}
	return &TemplateMod{Mods: mods, Templates: t}
}

func (m *TemplateMod) Do(templateName string, args map[string][]string, w http.ResponseWriter) {
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
	m.ExecuteTemplate(&template, w)
}

func (m *TemplateMod) ExecuteTemplate(template *Template, w http.ResponseWriter) {
	for _, t := range template.Actions {
		mod, modExists := m.Mods[t.Mod]
		if !modExists {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "module %v unknown", t.Mod)
			return
		}
		mod.Do(t.Fn, t.Args, w)
	}
}

func (m *TemplateMod) ExecuteMod(mod string, fn string, argstring string) {
	w := utils.StoreWriter{}
	args, _ := url.ParseQuery(argstring)
	ta := TemplateAction{Mod: mod, Fn: fn, Args: args}
	actions := []TemplateAction{ta}
	t := Template{Actions: actions}
	m.ExecuteTemplate(&t, &w)
	w.Print()
}

package restonatorx

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
	for _, t := range template.Actions {
		m.Mods[t.Mod].Do(t.Fn, t.Args, w)
	}
}

package freepsdo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

type SystemMod struct {
	Modinator *TemplateMod
}

var _ Mod = &SystemMod{}

func NewSystemeMod(modintor *TemplateMod) *SystemMod {
	return &SystemMod{Modinator: modintor}
}

func (m *SystemMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	var args map[string]string
	json.Unmarshal(jsonStr, &args)
	for expectedFn, expectedArgs := range fnArgs {
		if fn == expectedFn {
			for _, a := range expectedArgs {
				if _, ok := args[a]; !ok {
					jrw.WriteError(http.StatusBadRequest, "expected argument \"%s\"", a)
					return
				}
			}
			break
		}
	}
	switch fn {
	case "GetTemplate":
		m.getTemplate(args["name"], jrw)
	case "RenameTemplate":
		m.renameTemplate(args["name"], args["newName"], jrw)
	case "DeleteTemplate":
		m.deleteTemplate(args["name"], jrw)
	case "SaveLast":
		m.saveLast(args["newName"], jrw)
	case "MergeTemplates":
		m.mergeTemplates(args["src"], args["dest"], jrw)
	default:
		jrw.WriteError(404, "Function %s not found", fn)
	}
}

func (m *SystemMod) GetFunctions() []string {
	fn := make([]string, 0, len(fnArgs))
	for k := range fnArgs {
		fn = append(fn, k)
	}
	sort.Strings(fn)
	return fn
}

func (m *SystemMod) GetPossibleArgs(fn string) []string {
	if f, ok := fnArgs[fn]; ok {
		return f
	}
	return make([]string, 0)
}

func (m *SystemMod) GetArgSuggestions(fn string, arg string) map[string]string {
	if arg == "src" || arg == "dest" || arg == "name" {
		ret := map[string]string{}
		for k := range m.Modinator.Templates {
			ret[k] = k
		}
		return ret
	}
	if fn == "SaveLast" {
		sug := fmt.Sprintf("%s-%s", m.Modinator.Templates["_last"].Actions[0].Mod, m.Modinator.Templates["_last"].Actions[0].Fn)
		return map[string]string{sug: sug}
	}
	return map[string]string{}
}

var fnArgs map[string][]string = map[string][]string{
	"GetTemplate":    {"name"},
	"DeleteTemplate": {"name"},
	"RenameTemplate": {"name", "newName"},
	"SaveLast":       {"newName"},
	"MergeTemplates": {"src", "dest"},
}

func (m *SystemMod) getTemplate(name string, jrw *ResponseCollector) {
	tpl, ok := m.Modinator.Templates[name]
	if !ok {
		jrw.WriteError(404, "No template named %s found", name)
	}
	jrw.WriteSuccessMessage(tpl)
}

func (m *SystemMod) saveLast(name string, jrw *ResponseCollector) {
	name = m.pickFreeTemplateName(name)
	m.Modinator.Templates[name] = m.Modinator.Templates["_last"]
	jrw.WriteSuccessf("Saved as %s", name)
}

func (m *SystemMod) deleteTemplate(name string, jrw *ResponseCollector) {
	delete(m.Modinator.Templates, name)
}

func (m *SystemMod) renameTemplate(name string, newName string, jrw *ResponseCollector) {
	_, ok := m.Modinator.Templates[name]
	if !ok {
		jrw.WriteError(404, "Template named %s not found", name)
	}
	_, ok = m.Modinator.Templates[newName]
	if ok {
		jrw.WriteError(http.StatusConflict, "Template %s already exists", newName)
		return
	}
	m.Modinator.Templates[newName] = m.Modinator.Templates[name]
	delete(m.Modinator.Templates, name)
}

func (m *SystemMod) mergeTemplates(srcName string, destName string, jrw *ResponseCollector) {
	src, ok := m.Modinator.Templates[srcName]
	if !ok {
		jrw.WriteError(404, "Src template named %s not found", srcName)
	}
	dest, ok := m.Modinator.Templates[destName]
	if !ok {
		jrw.WriteError(404, "Dest template named %s not found", destName)
	}
	dest.Actions = append(dest.Actions, src.Actions...)
	jrw.WriteSuccessf("Merged %s into %s", srcName, destName)
}

func (m *SystemMod) pickFreeTemplateName(name string) string {
	ok := true
	newName := name
	i := 0
	for {
		_, ok = m.Modinator.Templates[newName]
		if !ok {
			return newName
		}
		newName = fmt.Sprintf("%s-%d", name, i)
	}
}

package freepsdo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/disk"
	"github.com/mackerelio/go-osstat/loadavg"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/mackerelio/go-osstat/network"
	"github.com/mackerelio/go-osstat/uptime"
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
	case "GetLastResponse":
		jrw.WriteSuccessMessage(m.Modinator.Cache["_last"])
	case "OSStats":
		var s interface{}
		var err error
		switch args["statType"] {
		case "cpu":
			s, err = cpu.Get()
		case "disk":
			s, err = disk.Get()
		case "loadavg":
			s, err = loadavg.Get()
		case "memory":
			s, err = memory.Get()
		case "network":
			s, err = network.Get()
		case "uptime ":
			s, err = uptime.Get()
		default:
			err = fmt.Errorf("Unknown statistics Type: %s", args["statType"])
		}
		if err == nil {
			jrw.WriteSuccessMessage(s)
		} else {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
		}
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

func (m *SystemMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	if arg == "src" || arg == "dest" || arg == "name" {
		ret := map[string]string{}
		for k := range m.Modinator.GetAllTemplates(false) {
			ret[k] = k
		}
		return ret
	}
	if fn == "SaveLast" {
		lastT, _ := m.Modinator.GetTemplate("_last")
		sug := fmt.Sprintf("%s-%s", lastT.Actions[0].Mod, lastT.Actions[0].Fn)
		return map[string]string{sug: sug}
	}
	if arg == "statType" {
		return map[string]string{"cpu": "cpu",
			"disk":    "disk",
			"loadavg": "loadavg",
			"memory":  "memory",
			"network": "network",
			"uptime ": "uptime "}
	}
	return map[string]string{}
}

var fnArgs map[string][]string = map[string][]string{
	"GetTemplate":     {"name"},
	"DeleteTemplate":  {"name"},
	"RenameTemplate":  {"name", "newName"},
	"SaveLast":        {"newName"},
	"MergeTemplates":  {"src", "dest"},
	"GetLastResponse": {},
	"OSStats":         {"statType"},
}

func (m *SystemMod) getTemplate(name string, jrw *ResponseCollector) {
	tpl, ok := m.Modinator.GetTemplate(name)
	if !ok {
		jrw.WriteError(404, "No template named %s found", name)
	}
	jrw.WriteSuccessMessage(tpl)
}

func (m *SystemMod) saveLast(name string, jrw *ResponseCollector) {
	name = m.pickFreeTemplateName(name)
	m.Modinator.Config.Templates[name] = m.Modinator.TemporaryTemplates["_last"]
	m.Modinator.cr.WriteSection("TemplateMod", m.Modinator.Config, true)
	jrw.WriteSuccessf("Saved as %s", name)
}

func (m *SystemMod) deleteTemplate(name string, jrw *ResponseCollector) {
	delete(m.Modinator.Config.Templates, name)
	m.Modinator.cr.WriteSection("TemplateMod", m.Modinator.Config, true)
	jrw.WriteSuccessf("Deleted %s", name)
}

func (m *SystemMod) renameTemplate(name string, newName string, jrw *ResponseCollector) {
	template, ok := m.Modinator.GetTemplate(name)
	if !ok {
		jrw.WriteError(404, "Template named %s not found", name)
	}
	_, ok = m.Modinator.Config.Templates[newName]
	if ok {
		jrw.WriteError(http.StatusConflict, "Template %s already exists", newName)
		return
	}
	m.Modinator.Config.Templates[newName] = template
	delete(m.Modinator.Config.Templates, name)
	m.Modinator.cr.WriteSection("TemplateMod", m.Modinator.Config, true)
	jrw.WriteSuccessf("Renamed %s to %s", name, newName)
}

func (m *SystemMod) mergeTemplates(srcName string, destName string, jrw *ResponseCollector) {
	src, ok := m.Modinator.GetTemplate(srcName)
	if !ok {
		jrw.WriteError(404, "Src template named %s not found", srcName)
	}
	dest, ok := m.Modinator.GetTemplate(destName)
	if !ok {
		jrw.WriteError(404, "Dest template named %s not found", destName)
	}
	// make sure the template is in a writable location
	m.Modinator.Config.Templates[destName] = dest
	dest.Actions = append(dest.Actions, src.Actions...)
	jrw.WriteSuccessf("Merged %s into %s", srcName, destName)
	m.Modinator.cr.WriteSection("TemplateMod", m.Modinator.Config, true)
}

func (m *SystemMod) pickFreeTemplateName(name string) string {
	ok := true
	newName := name
	i := 0
	for {
		_, ok = m.Modinator.Config.Templates[newName]
		if !ok {
			return newName
		}
		newName = fmt.Sprintf("%s-%d", name, i)
	}
}

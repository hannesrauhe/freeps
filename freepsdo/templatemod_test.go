package freepsdo

import (
	"encoding/json"
	"testing"
)

type MockMod struct {
	DoCount      int
	LastFunction string
	LastJSON     []byte
}

var _ Mod = &MockMod{}

func (m *MockMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	m.DoCount++
	m.LastFunction = fn
	m.LastJSON = jsonStr
}

func (m *MockMod) GetFunctions() []string {
	keys := make([]string, 0)
	return keys
}

func (m *MockMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *MockMod) GetArgSuggestions(fn string, arg string) map[string]string {
	ret := map[string]string{}
	return ret
}

type TestStruct struct {
	DefaultArg   int
	OverwriteArg int
	NewArg       int
	DiffArg      int
}

func NewTMMock(templates map[string]*Template) (*TemplateMod, *MockMod) {
	mods := map[string]Mod{}
	mm := &MockMod{}
	mods["mock"] = mm
	tm := &TemplateMod{Mods: mods, Config: TemplateModConfig{Templates: templates}, TemporaryTemplates: map[string]*Template{}, ExternalTemplates: map[string]*Template{}}
	mods["template"] = tm
	return tm, mm
}

func TestCallTemplateWithJsonArgs(t *testing.T) {
	ta := TemplateAction{Mod: "mock", Fn: "fn", Args: map[string]interface{}{"defaultArg": 1, "overwriteArg": 1}}
	actions := []TemplateAction{ta}
	tpl := &Template{Actions: actions}
	tm, mm := NewTMMock(map[string]*Template{"tpl1": tpl})

	w := NewResponseCollector()
	tm.ExecuteModWithJson("template", "tpl1", []byte(`{"newArg":3, "overwriteArg":5}`), w)
	expected := TestStruct{DefaultArg: 1, OverwriteArg: 5, NewArg: 3}
	var actual TestStruct
	json.Unmarshal(mm.LastJSON, &actual)
	if expected != actual {
		t.Errorf("Unexpected parameters passed to Tpl: %v", actual)
	}

	w2 := NewResponseCollector()
	tm.ExecuteModWithJson("template", "tpl1", []byte(`{"DiffArg":42}`), w2)
	expected2 := TestStruct{DefaultArg: 1, OverwriteArg: 1, DiffArg: 42}
	var actual2 TestStruct
	json.Unmarshal(mm.LastJSON, &actual2)
	if expected2 != actual2 {
		t.Errorf("Unexpected parameters passed to Tpl: %v", actual2)
	}
}

func TestTemporaryTemplateActions(t *testing.T) {
	ta := TemplateAction{Mod: "mock", Fn: "fn", Args: map[string]interface{}{"defaultArg": 1, "overwriteArg": 1}}
	actions := []TemplateAction{ta}
	tpl := &Template{Actions: actions}

	mods := map[string]Mod{}
	mm := &MockMod{}
	mods["mock"] = mm
	tm := &TemplateMod{Mods: mods, TemporaryTemplates: map[string]*Template{"tpl1": tpl}}
	mods["template"] = tm

	tta := tm.GetTemporaryTemplateAction("1")
	if tta == nil && tta.Mod != "" {
		t.Fatal("unexpected TA")
	}

	tta.Mod = "foo"

	tta2 := tm.GetTemporaryTemplateAction("1")
	if tta.Mod != tta2.Mod {
		t.Fatal("unexpected TTA")
	}
	tm.RemoveTemporaryTemplate("1")

	tta3 := tm.GetTemporaryTemplateAction("1")
	if tta3 == nil && tta3.Mod != "" {
		t.Fatal("unexpected TA")
	}
}

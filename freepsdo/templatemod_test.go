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

func TestCallTemplateWithJsonArgs(t *testing.T) {
	ta := TemplateAction{Mod: "mock", Fn: "fn", Args: map[string]interface{}{"defaultArg": 1, "overwriteArg": 1}}
	actions := []TemplateAction{ta}
	tpl := &Template{Actions: actions}

	mods := map[string]Mod{}
	mm := &MockMod{}
	mods["mock"] = mm
	tm := &TemplateMod{Mods: mods, Templates: map[string]*Template{"tpl1": tpl}}
	mods["template"] = tm

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
	tm := &TemplateMod{Mods: mods, Templates: map[string]*Template{"tpl1": tpl}}
	mods["template"] = tm

	if tm.GetTemporaryTemplateAction(0) != nil {
		t.Fatal("unexpected TA")
	}

	if tm.CreateTemporaryTemplateAction() != 0 {
		t.Fatal("unexpected Temporary ID")
	}

	if tm.GetTemporaryTemplateAction(1) != nil {
		t.Fatal("unexpected TA")
	}

	if tm.CreateTemporaryTemplateAction() != 1 {
		t.Fatal("unexpected Temporary ID")
	}

	if tm.CreateTemporaryTemplateAction() != 2 {
		t.Fatal("unexpected Temporary ID")
	}

	tta := tm.GetTemporaryTemplateAction(1)
	if tta == nil {
		t.Fatal("unexpected TTA")
	}
	tta.Mod = "foo"

	tta = tm.GetTemporaryTemplateAction(0)
	if tta.Mod != "" {
		t.Fatal("unexpected TTA")
	}

	tta = tm.GetTemporaryTemplateAction(1)
	if tta.Mod != "foo" {
		t.Fatal("unexpected TTA")
	}
}

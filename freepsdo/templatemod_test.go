package freepsdo

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hannesrauhe/freeps/utils"
)

type MockMod struct {
	DoCount      int
	LastFunction string
	LastArgs     map[string][]string
	LastJSON     []byte
}

func (m *MockMod) Do(function string, args map[string][]string, w http.ResponseWriter) {
	m.DoCount++
	m.LastFunction = function
	m.LastArgs = args
}

func (m *MockMod) DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter) {
	m.DoCount++
	m.LastFunction = fn
	m.LastJSON = jsonStr
}

type TestStruct struct {
	DefaultArg   int
	OverwriteArg int
	NewArg       int
	DiffArg      int
}

func TestCallTemplateWithJsonArgs(t *testing.T) {
	ta := TemplateAction{Mod: "mock", Fn: "fn", JsonArgs: map[string]interface{}{"defaultArg": 1, "overwriteArg": 1}}
	actions := []TemplateAction{ta}
	tpl := Template{Actions: actions}

	mods := map[string]Mod{}
	mm := &MockMod{}
	mods["mock"] = mm
	tm := &TemplateMod{Mods: mods, Templates: map[string]Template{"tpl1": tpl}}
	mods["template"] = tm

	w := utils.StoreWriter{}

	tm.DoWithJSON("tpl1", []byte(`{"newArg":3, "overwriteArg":5}`), &w)
	expected := TestStruct{DefaultArg: 1, OverwriteArg: 5, NewArg: 3}
	var actual TestStruct
	json.Unmarshal(mm.LastJSON, &actual)
	if expected != actual {
		t.Errorf("Unexpected parameters passed to Tpl: %v", actual)
	}

	tm.DoWithJSON("tpl1", []byte(`{"DiffArg":42}`), &w)
	expected2 := TestStruct{DefaultArg: 1, OverwriteArg: 1, DiffArg: 42}
	var actual2 TestStruct
	json.Unmarshal(mm.LastJSON, &actual2)
	if expected2 != actual2 {
		t.Errorf("Unexpected parameters passed to Tpl: %v", actual2)
	}
}

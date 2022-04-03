package freepsdo

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/hannesrauhe/freeps/utils"
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
	var retMap map[string]interface{}
	json.Unmarshal(jsonStr, &retMap)
	jrw.WriteSuccessMessage(retMap)
}

func (m *MockMod) GetFunctions() []string {
	keys := make([]string, 0)
	return keys
}

func (m *MockMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *MockMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
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
	tmpFile, _ := ioutil.TempFile(os.TempDir(), "freeps-")
	cr, _ := utils.NewConfigReader(tmpFile.Name())
	mods := map[string]Mod{}
	mm := &MockMod{}
	mods["mock"] = mm
	tm := &TemplateMod{Mods: mods, cr: cr, Config: TemplateModConfig{Templates: templates}, TemporaryTemplates: map[string]*Template{}, ExternalTemplates: map[string]*Template{}, Cache: make(map[string][]byte)}
	mods["template"] = tm
	return tm, mm
}

func TestCallTemplateWithJsonArgs(t *testing.T) {
	ta := TemplateAction{Mod: "mock", Fn: "fn", Args: map[string]interface{}{"defaultArg": 1, "overwriteArg": 1}}
	actions := []TemplateAction{ta}
	tpl := &Template{Actions: actions}
	tm, mm := NewTMMock(map[string]*Template{"tpl1": tpl})

	w := NewResponseCollector("")
	tm.ExecuteModWithJson("template", "tpl1", []byte(`{"newArg":3, "overwriteArg":5}`), w)
	expected := TestStruct{DefaultArg: 1, OverwriteArg: 5, NewArg: 3}
	var actual TestStruct
	json.Unmarshal(mm.LastJSON, &actual)
	assert.Equal(t, expected, actual)
	r, err := w.GetOutput()
	assert.NilError(t, err)
	assert.Assert(t, r != nil)
	assert.Assert(t, is.Contains(r.Output, "defaultArg"))

	w2 := NewResponseCollector("")
	tm.ExecuteModWithJson("template", "tpl1", []byte(`{"DiffArg":42}`), w2)
	expected2 := TestStruct{DefaultArg: 1, OverwriteArg: 1, DiffArg: 42}
	var actual2 TestStruct
	json.Unmarshal(mm.LastJSON, &actual2)
	assert.Equal(t, expected2, actual2)
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

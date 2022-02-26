package freepsdo

import (
	"testing"
)

func TestTemplateFunctions(t *testing.T) {
	ta := TemplateAction{Mod: "mock", Fn: "fn", Args: map[string]interface{}{"defaultArg": 1, "overwriteArg": 1}}
	actions := []TemplateAction{ta}
	tpl := &Template{Actions: actions}

	tm, _ := NewTMMock(map[string]*Template{"tpl1": tpl})
	sm := NewSystemeMod(tm)
	tm.Mods["system"] = sm

	w := NewResponseCollector()
	tm.ExecuteModWithJson("template", "tpl1", []byte(`{"newArg":3, "overwriteArg":5}`), w)

	w2 := NewResponseCollector()
	tm.ExecuteModWithJson("system", "MergeTemplates", []byte(`{"src":"_last", "dest":"tpl1"}`), w2)
	if w2.IsStatusFailed() {
		t.Fatal(w2.GetFinalResponse())
	}
	if len(tm.Config.Templates["tpl1"].Actions) != 2 {
		t.Fatal("Merging failed")
	}

	w3 := NewResponseCollector()
	tm.ExecuteModWithJson("system", "RenameTemplate", []byte(`{"name":"tpl1", "newName":"tpl2"}`), w3)
	if w3.IsStatusFailed() {
		t.Fatal(w3.GetFinalResponse())
	}
	if len(tm.Config.Templates["tpl2"].Actions) != 2 {
		t.Fatal("Merging failed")
	}

	w4 := NewResponseCollector()
	tm.ExecuteModWithJson("system", "DeleteTemplate", []byte(`{"name":"tpl2"}`), w4)
	if w4.IsStatusFailed() {
		t.Fatal(w4.GetFinalResponse())
	}
	if _, ok := tm.Config.Templates["tpl2"]; ok {
		t.Fatal("Merging failed")
	}
}

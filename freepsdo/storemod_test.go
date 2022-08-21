package freepsdo

import (
	"os"
	"testing"
)

func TestStoreMod(t *testing.T) {
	ta := TemplateAction{Mod: "mock", Fn: "fn", Args: map[string]interface{}{"defaultArg": 1, "overwriteArg": 1}}
	actions := []TemplateAction{ta}
	tpl := &Template{Actions: actions}

	tm, _ := NewTMMock(map[string]*Template{"tpl1": tpl})
	tm.Mods["store"] = NewStoreMod()
	{
		w := NewResponseCollector("")
		tm.ExecuteModWithJson("store", "set", []byte(`{"key":"yeahy", "value":5}`), w)
		if w.IsStatusFailed() {
			_, _, b := w.GetFinalResponse(false)
			os.Stdout.Write(b)
			t.Fatal()
		}
	}
	{
		w := NewResponseCollector("")
		tm.ExecuteModWithJson("store", "get", []byte(`{"key":"yeahy"}`), w)
		if w.IsStatusFailed() {
			t.Fatal(w.GetFinalResponse(false))
		}
	}
	{
		w := NewResponseCollector("")
		tm.ExecuteModWithJson("store", "get", []byte(`{"key":"yeahynot"}`), w)
		if !w.IsStatusFailed() {
			t.Fatal("key should not exist")
		}
	}
}

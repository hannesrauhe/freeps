package freepsdo

import "encoding/json"

type EchoMod struct {
}

var _ Mod = &EchoMod{}

func (m *EchoMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	if fn == "bytes" {
		jrw.WriteSuccessMessage(jsonStr)
	} else if fn == "hello" {
		jrw.WriteSuccessf("Hello World")
	} else if fn == "direct" {
		var v interface{}
		json.Unmarshal(jsonStr, &v)
		jrw.WriteSuccessMessage(v)
	} else if fn == "escaped" {
		jrw.WriteSuccessMessage(string(jsonStr))
	}
}

func (m *EchoMod) GetFunctions() []string {
	return []string{"bytes", "hello", "direct", "escaped"}
}

func (m *EchoMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *EchoMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	ret := map[string]string{}
	return ret
}

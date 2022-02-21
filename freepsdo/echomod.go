package freepsdo

import "encoding/json"

type EchoMod struct {
}

var _ Mod = &EchoMod{}

func (m *EchoMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	if fn == "bytes" {
		jrw.WriteSuccessMessage(jsonStr)
	} else if fn == "direct" {
		var v interface{}
		json.Unmarshal(jsonStr, &v)
		jrw.WriteSuccessMessage(v)
	} else if fn == "escaped" {
		jrw.WriteSuccessMessage(string(jsonStr))
	}
}

func (m *EchoMod) GetFunctions() []string {
	keys := make([]string, 0)
	return keys
}

func (m *EchoMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *EchoMod) GetArgSuggestions(fn string, arg string) map[string]string {
	ret := map[string]string{}
	return ret
}

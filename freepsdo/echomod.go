package freepsdo

import "encoding/json"

type EchoMod struct {
}

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

var _ Mod = &EchoMod{}

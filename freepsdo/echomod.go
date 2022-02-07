package freepsdo

import (
	"fmt"
	"net/http"
)

type EchoMod struct {
}

func (m *EchoMod) DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter) {
	fmt.Fprintf(w, "Function: %v\nArgs: %q\n", fn, jsonStr)
}

var _ Mod = &EchoMod{}

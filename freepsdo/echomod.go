package freepsdo

import (
	"fmt"
	"net/http"
)

type EchoMod struct {
}

func (m *EchoMod) Do(function string, args map[string][]string, w http.ResponseWriter) {
	fmt.Fprintf(w, "Function: %v\nArgs: %v\n", function, args)
}

func (m *EchoMod) DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter) {
	fmt.Fprintf(w, "Function: %v\nArgs: %q\n", fn, jsonStr)
}

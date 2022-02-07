package freepsdo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
)

type ScriptMod struct {
}

var _ Mod = &ScriptMod{}

type ScriptParameters struct {
	Args []string
}

func (m *ScriptMod) DoWithJSON(function string, jsonStr []byte, w http.ResponseWriter) {
	params := ScriptParameters{}
	json.Unmarshal(jsonStr, &params)
	cmd := exec.Command("./scripts/"+function, params.Args...)
	stdout, err := cmd.Output()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Executed: %v\nParameters: %v\nError: %v", function, params.Args, string(err.Error()))
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Executed: %v\nParameters: %v\nOutput: %v", function, params.Args, string(stdout))
	}

}

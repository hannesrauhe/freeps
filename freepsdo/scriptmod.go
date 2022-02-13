package freepsdo

import (
	"encoding/json"
	"net/http"
	"os/exec"
)

type ScriptMod struct {
}

var _ Mod = &ScriptMod{}

type ScriptParameters struct {
	Args []string
}

func (m *ScriptMod) DoWithJSON(function string, jsonStr []byte, jrw *ResponseCollector) {
	params := ScriptParameters{}
	json.Unmarshal(jsonStr, &params)
	cmd := exec.Command("./scripts/"+function, params.Args...)
	stdout, err := cmd.Output()
	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, "Executed: %v\nParameters: %v\nError: %v", function, params.Args, string(err.Error()))
	} else {
		jrw.WriteSuccessf("Executed: %v\nParameters: %v\nOutput: %v", function, params.Args, string(stdout))
	}

}

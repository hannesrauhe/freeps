package restonatorx

import (
	"fmt"
	"net/http"
	"os/exec"
)

type ScriptMod struct {
	functions map[string][]string
}

func (m *ScriptMod) Do(function string, args map[string][]string, w http.ResponseWriter) {
	cmd := exec.Command("./scripts/"+function, args["args"]...)
	stdout, err := cmd.Output()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Executed: %v\nParameters: %v\nError: %v", function, args["arg"], string(err.Error()))
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Executed: %v\nParameters: %v\nOutput: %v", function, args["arg"], string(stdout))
	}
}

package freepsdo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"github.com/hannesrauhe/freeps/utils"
)

type ScriptMod struct {
	ScriptDir string
}

type ScriptModConfig struct {
	ScriptDir string
}

var ScriptModDefaultConfig = ScriptModConfig{"/etc/freepsd/scripts"}

var _ Mod = &ScriptMod{}

type ScriptParameters struct {
	Mode string
	Args []string
}

func NewScriptMod(cr *utils.ConfigReader) *ScriptMod {
	conf := ScriptModDefaultConfig
	err := cr.ReadSectionWithDefaults("scriptmod", &conf)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	return &ScriptMod{ScriptDir: conf.ScriptDir}
}

func (m *ScriptMod) DoWithJSON(function string, jsonStr []byte, w http.ResponseWriter) {
	params := ScriptParameters{}
	err := json.Unmarshal(jsonStr, &params)

	if err != nil {
		log.Printf("%q", jsonStr)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	cmd := exec.Command(m.ScriptDir+"/"+function, params.Args...)
	var stdout []byte
	if params.Mode == "detach" {
		err = cmd.Start()
	} else {
		stdout, err = cmd.Output()
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Executed: %v\nParameters: %v\nError: %v", function, params.Args, string(err.Error()))
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Executed: %v\nParameters: %v\nOutput: %v", function, params.Args, string(stdout))
	}

}

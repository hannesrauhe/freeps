package freepsdo

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"path"

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

func (m *ScriptMod) DoWithJSON(function string, jsonStr []byte, jrw *ResponseCollector) {
	params := ScriptParameters{}
	err := json.Unmarshal(jsonStr, &params)

	if err != nil {
		log.Printf("%q", jsonStr)
		jrw.WriteError(http.StatusBadRequest, "%v", err.Error())
		return
	}
	scriptName := path.Base(function)
	cmd := exec.Command(m.ScriptDir+"/"+scriptName, params.Args...)
	var stdout []byte
	if params.Mode == "detach" {
		err = cmd.Start()
	} else {
		stdout, err = cmd.Output()
	}
	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, "Executed: %v\nParameters: %v\nError: %v", scriptName, params.Args, string(err.Error()))
	} else {
		jrw.WriteSuccessf("Executed: %v\nParameters: %v\nOutput: %v", scriptName, params.Args, string(stdout))
	}
}

func (m *ScriptMod) GetFunctions() []string {
	keys := make([]string, 0)
	return keys
}

func (m *ScriptMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *ScriptMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	ret := map[string]string{}
	return ret
}

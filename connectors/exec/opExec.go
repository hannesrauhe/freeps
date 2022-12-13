package freepsexec

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

type ExecutableConfig struct {
	Path               string
	OutputContentType  string
	DefaultArguments   map[string]string
	AvailableArguments map[string]map[string]string
}

type OpExecConfig struct {
	Programs map[string]ExecutableConfig
}

// DefaultConfig shows sample configuration with raspistill-settings
var DefaultConfig = OpExecConfig{
	Programs: map[string]ExecutableConfig{
		"raspistill": {
			Path:              "/usr/bin/raspistill",
			OutputContentType: "image/jpeg",
			AvailableArguments: map[string]map[string]string{
				"-rot": {"0": "0", "90": "90", "180": "180", "270": "270"},
				"-ss":  {"1s": "1000", "2s": "2000", "3s": "3000", "4s": "4000", "5s": "5000"},
			},
			DefaultArguments: map[string]string{
				"-w":           "1600",
				"-h":           "1200",
				"-o":           "-",
				"-e":           "jpg",
				"--quality":    "90",
				"--brightness": "50"},
		},
	},
}

// OpExec defines an Operator that executes a given binary
type OpExec struct {
	ExecutableConfig
	name string
}

var _ freepsgraph.FreepsOperator = &OpExec{}

// GetName returns the name of the Executable as given by the config
func (o *OpExec) GetName() string {
	return o.name
}

func (o *OpExec) execBin(ctx *utils.Context, argsmap map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	args := []string{}
	for k, v := range argsmap {
		args = append(args, k)
		if v != "" {
			args = append(args, v)
		}
	}

	e := exec.Command(o.Path, args...)
	if !input.IsEmpty() {
		stdin, err := e.StdinPipe()
		if err != nil {
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Error attaching stdin pipe: %v", err.Error())
		}
		inputbytes, err := input.GetBytes()
		if err != nil {
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Error getting bytes from input: %v", err.Error())
		}

		go func() {
			defer stdin.Close()

			stdin.Write(inputbytes)
		}()
	}

	byt, err := e.CombinedOutput()

	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Error executing %v: %v", o.name, err.Error())
	}

	return freepsgraph.MakeByteOutputWithContentType(byt, "image/jpeg")
}

// Execute executes the binary
func (o *OpExec) Execute(ctx *utils.Context, fn string, vars map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	switch fn {
	case "do":
		argsmap := map[string]string{}
		for k, v := range o.DefaultArguments {
			argsmap[k] = v
		}
		for k, v := range vars {
			argsmap[k] = v
		}
		return o.execBin(ctx, argsmap, input)
	case "doNoDefaultArgs":
		return o.execBin(ctx, vars, input)
	}

	return freepsgraph.MakeOutputError(http.StatusNotFound, "Function %v not found", fn)
}

// GetFunctions returns functions representing how to execute bin
func (o *OpExec) GetFunctions() []string {
	ret := []string{"do", "doNoDefaultArgs"}
	return ret
}

// GetPossibleArgs returns possible command line arguments
func (o *OpExec) GetPossibleArgs(fn string) []string {
	ret := []string{}
	for k, _ := range o.AvailableArguments {
		ret = append(ret, k)
	}
	for k, _ := range o.DefaultArguments {
		ret = append(ret, k)
	}
	return ret
}

// GetArgSuggestions returns suggestions for command line arguments
func (o *OpExec) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	if r, exists := o.AvailableArguments[arg]; exists {
		return r
	}
	if r, exists := o.DefaultArguments[arg]; exists {
		return map[string]string{r + " (default)": r}
	}
	return map[string]string{}
}

// AddExecOperators adds executables to the config
func AddExecOperators(cr *utils.ConfigReader, graphEngine *freepsgraph.GraphEngine) error {
	execConfig := DefaultConfig
	err := cr.ReadSectionWithDefaults("executables", &execConfig)
	if err != nil {
		return fmt.Errorf("Could not read executables from config: %v", err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	for name, config := range execConfig.Programs {
		if graphEngine.HasOperator(name) {
			log.Errorf("Cannot add executable Operator %v, an operator with that name already exists", name)
			continue
		}
		o := &OpExec{config, name}
		graphEngine.AddOperator(o)
	}
	return nil
}

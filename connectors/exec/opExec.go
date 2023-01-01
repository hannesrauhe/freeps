package freepsexec

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"sync"
	"syscall"

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
	name        string
	bgChan      chan error
	cmd         *exec.Cmd
	processLock sync.Mutex
	bgOutput    bytes.Buffer
}

var _ freepsgraph.FreepsOperator = &OpExec{}

// GetName returns the name of the Executable as given by the config
func (o *OpExec) GetName() string {
	return o.name
}

func makeArgs(argsmap map[string]string) []string {
	args := []string{}
	for k, v := range argsmap {
		args = append(args, k)
		if v != "" {
			args = append(args, v)
		}
	}
	return args
}

func (o *OpExec) execBin(ctx *utils.Context, argsmap map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	args := makeArgs(argsmap)

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

	return freepsgraph.MakeByteOutputWithContentType(byt, o.OutputContentType)
}

func (o *OpExec) runInBackground(ctx *utils.Context, argsmap map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	o.processLock.Lock()
	defer o.processLock.Unlock()

	if o.cmd != nil {
		return freepsgraph.MakeOutputError(http.StatusConflict, "Error executing %v in background, it's already running", o.name)
	}

	args := makeArgs(argsmap)

	o.bgOutput.Reset()
	o.cmd = exec.Command(o.Path, args...)
	// This sets up a process group which we kill later.
	o.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	o.cmd.Stdout = &o.bgOutput

	if err := o.cmd.Start(); err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Error executing %v: %v", o.name, err.Error())
	}
	o.bgChan = make(chan error, 1)

	go func() {
		o.bgChan <- o.cmd.Wait()
		ctx.GetLogger().Debugf("%v", string(o.bgOutput.Bytes()))
		o.processLock.Lock()
		defer o.processLock.Unlock()
		close(o.bgChan)
		o.cmd = nil
	}()
	return freepsgraph.MakeEmptyOutput()
}

func (o *OpExec) stopBackground(ctx *utils.Context) *freepsgraph.OperatorIO {
	o.processLock.Lock()
	defer o.processLock.Unlock()

	if o.cmd == nil {
		return freepsgraph.MakeOutputError(http.StatusGone, "No process running")
	}

	pgid, err := syscall.Getpgid(o.cmd.Process.Pid)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Could not kill process group: %v", err)
	}

	if err := syscall.Kill(-pgid, 15); err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Could not kill process group: %v", err)
	}

	return freepsgraph.MakeByteOutput(o.bgOutput.Bytes())
}

// Execute executes the binary
func (o *OpExec) Execute(ctx *utils.Context, fn string, vars map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {

	argsmap := map[string]string{}
	for k, v := range o.DefaultArguments {
		argsmap[k] = v
	}
	for k, v := range vars {
		argsmap[k] = v
	}

	switch fn {
	case "do", "run":
		return o.execBin(ctx, argsmap, input)
	case "doNoDefaultArgs", "runWithoutDefaultArgs":
		return o.execBin(ctx, vars, input)
	case "runInBackground":
		return o.runInBackground(ctx, argsmap, input)
	case "stopBackgroundProcess":
		return o.stopBackground(ctx)
	}

	return freepsgraph.MakeOutputError(http.StatusNotFound, "Function %v not found", fn)
}

// GetFunctions returns functions representing how to execute bin
func (o *OpExec) GetFunctions() []string {
	ret := []string{"run", "runWithoutDefaultArgs", "runInBackground", "stopBackgroundProcess"}
	return ret
}

// GetPossibleArgs returns possible command line arguments
func (o *OpExec) GetPossibleArgs(fn string) []string {
	ret := []string{}
	for k := range o.AvailableArguments {
		ret = append(ret, k)
	}
	for k := range o.DefaultArguments {
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

// Shutdown (noOp)
func (o *OpExec) Shutdown(ctx *utils.Context) {
	o.stopBackground(ctx)
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
		o := &OpExec{config, name, make(chan error), nil, sync.Mutex{}, bytes.Buffer{}}
		graphEngine.AddOperator(o)
	}
	return nil
}
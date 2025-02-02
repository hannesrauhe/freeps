//go:build !noexec && linux

package freepsexec

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

type ExecutableConfig struct {
	Path               string
	OutputContentType  string
	DefaultArguments   map[string]string
	AvailableArguments map[string]map[string]string
	DefaultEnv         map[string]string
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
		"graphviz": {
			Path: "/usr/bin/dot",
			AvailableArguments: map[string]map[string]string{
				"-rot": {"0": "0", "90": "90", "180": "180", "270": "270"},
				"-ss":  {"1s": "1000", "2s": "2000", "3s": "3000", "4s": "4000", "5s": "5000"},
			},
			DefaultArguments: map[string]string{
				"-Tpng": "",
			},
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

var _ base.FreepsBaseOperator = &OpExec{}

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

func (o *OpExec) execBin(ctx *base.Context, args []string, env map[string]string, input *base.OperatorIO) *base.OperatorIO {
	var err error

	e := exec.Command(o.Path, args...)
	if env != nil {
		envArr := os.Environ()
		for k, v := range env {
			envArr = append(envArr, fmt.Sprintf("%v=%v", k, v))
		}
		e.Env = envArr
	}
	e.Dir, err = utils.GetTempDir()
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot set working dir: %v", err.Error())
	}
	if !input.IsEmpty() {
		stdin, err := e.StdinPipe()
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "Error attaching stdin pipe: %v", err.Error())
		}
		inputbytes, err := input.GetBytes()
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "Error getting bytes from input: %v", err.Error())
		}

		go func() {
			defer stdin.Close()

			stdin.Write(inputbytes)
		}()
	}

	byt, err := e.CombinedOutput()

	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error executing %v: %v\n%q\n", o.name, err.Error(), byt)
	}

	return base.MakeByteOutputWithContentType(byt, o.OutputContentType)
}

func (o *OpExec) runInBackground(ctx *base.Context, argsmap map[string]string, input *base.OperatorIO) *base.OperatorIO {
	o.processLock.Lock()
	defer o.processLock.Unlock()

	if o.cmd != nil {
		return base.MakeOutputError(http.StatusConflict, "Error executing %v in background, it's already running", o.name)
	}

	args := makeArgs(argsmap)

	o.bgOutput.Reset()
	o.cmd = exec.Command(o.Path, args...)
	// This sets up a process group which we kill later.
	o.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	o.cmd.Stdout = &o.bgOutput

	if err := o.cmd.Start(); err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error executing %v: %v", o.name, err.Error())
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
	return base.MakeEmptyOutput()
}

func (o *OpExec) stopBackground(ctx *base.Context) *base.OperatorIO {
	o.processLock.Lock()
	defer o.processLock.Unlock()

	if o.cmd == nil {
		return base.MakeOutputError(http.StatusGone, "No process running")
	}

	pgid, err := syscall.Getpgid(o.cmd.Process.Pid)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Could not kill process group: %v", err)
	}

	if err := syscall.Kill(-pgid, 15); err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Could not kill process group: %v", err)
	}

	return base.MakeByteOutput(o.bgOutput.Bytes())
}

func (o *OpExec) Execute(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	return o.ExecuteOld(ctx, fn, fa.GetOriginalCaseMapJoined(), input)
}

// Execute executes the binary
func (o *OpExec) ExecuteOld(ctx *base.Context, fn string, vars map[string]string, input *base.OperatorIO) *base.OperatorIO {
	argsmap := map[string]string{}
	for k, v := range o.DefaultArguments {
		argsmap[k] = v
	}
	for k, v := range vars {
		argsmap[k] = v
	}

	env := map[string]string{}
	for k, v := range o.DefaultEnv {
		env[k] = v
	}

	switch fn {
	case "do", "run":
		args := makeArgs(argsmap)
		return o.execBin(ctx, args, env, input)
	case "doNoDefaultArgs", "runWithoutDefaultArgs":
		args := makeArgs(vars)
		return o.execBin(ctx, args, env, input)
	case "runSingleArgString":
		args := strings.Split(vars["argString"], " ")
		for k, v := range vars {
			if k == "argString" {
				continue
			}
			env[k] = v
		}
		return o.execBin(ctx, args, env, input)
	case "runInBackground":
		return o.runInBackground(ctx, argsmap, input)
	case "stopBackgroundProcess":
		return o.stopBackground(ctx)
	}

	return base.MakeOutputError(http.StatusNotFound, "Function %v not found", fn)
}

// GetFunctions returns functions representing how to execute bin
func (o *OpExec) GetFunctions() []string {
	ret := []string{"run", "runSingleArgString", "runWithoutDefaultArgs", "runInBackground", "stopBackgroundProcess"}
	return ret
}

// GetPossibleArgs returns possible command line arguments
func (o *OpExec) GetPossibleArgs(fn string) []string {
	ret := []string{}
	if fn == "runSingleArgString" {
		ret = append(ret, "argString")
		for k := range o.DefaultEnv {
			ret = append(ret, k)
		}
	}
	for k := range o.AvailableArguments {
		ret = append(ret, k)
	}
	for k := range o.DefaultArguments {
		ret = append(ret, k)
	}
	return ret
}

// GetArgSuggestions returns suggestions for command line arguments
func (o *OpExec) GetArgSuggestions(fn string, arg string, otherArgs base.FunctionArguments) map[string]string {
	if fn == "runSingleArgString" {
		if arg == "argString" {
			return map[string]string{"argString": ""}
		}
		if r, exists := o.DefaultEnv[arg]; exists {
			return map[string]string{r + " (default)": r}
		}
		return map[string]string{}
	}

	if r, exists := o.AvailableArguments[arg]; exists {
		return r
	}
	if r, exists := o.DefaultArguments[arg]; exists {
		return map[string]string{r + " (default)": r}
	}
	return map[string]string{}
}

// StartListening (noOp)
func (o *OpExec) StartListening(ctx *base.Context) {
}

// Shutdown (noOp)
func (o *OpExec) Shutdown(ctx *base.Context) {
	o.stopBackground(ctx)
}

// GetHook (noOp)
func (o *OpExec) GetHook() interface{} {
	return nil
}

// AddExecOperators adds executables to the config
func AddExecOperators(cr *utils.ConfigReader, flowEngine *freepsflow.FlowEngine) error {
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
		if flowEngine.HasOperator(name) {
			log.Errorf("Cannot add executable Operator %v, an operator with that name already exists", name)
			continue
		}
		if config.AvailableArguments == nil {
			config.AvailableArguments = map[string]map[string]string{}
		}
		if config.DefaultArguments == nil {
			config.DefaultArguments = map[string]string{}
		}
		if config.DefaultEnv == nil {
			config.DefaultEnv = map[string]string{}
		}
		o := &OpExec{config, name, make(chan error), nil, sync.Mutex{}, bytes.Buffer{}}
		flowEngine.AddOperator(o)
	}
	return nil
}

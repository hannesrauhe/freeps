//go:build !noexec && linux

package freepsexecutable

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
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpExecutable struct {
	CR          *utils.ConfigReader
	GE          *freepsgraph.GraphEngine
	conf        ExecutableConfig
	name        string
	bgChan      chan error
	cmd         *exec.Cmd
	processLock sync.Mutex
	bgOutput    bytes.Buffer
}

var _ base.FreepsOperatorWithConfig = &OpExecutable{}

func (o *OpExecutable) GetDefaultConfig(fullName string) interface{} {
	configName := "default"
	s := strings.SplitAfterN(fullName, ".", 2)
	if len(s) >= 2 {
		configName = s[1]
	}
	switch configName {
	case "raspistill":
		return &ExecutableConfig{
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
		}

	case "graphviz":
		return &ExecutableConfig{
			Path: "/usr/bin/dot",
			AvailableArguments: map[string]map[string]string{
				"-rot": {"0": "0", "90": "90", "180": "180", "270": "270"},
				"-ss":  {"1s": "1000", "2s": "2000", "3s": "3000", "4s": "4000", "5s": "5000"},
			},
			DefaultArguments: map[string]string{
				"-Tpng": "",
			},
		}
	}
	return &ExecutableConfig{}
}

func (o *OpExecutable) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*ExecutableConfig)
	return &OpExecutable{name: name, conf: *cfg, bgChan: make(chan error), cmd: nil, processLock: sync.Mutex{}, bgOutput: bytes.Buffer{}}, nil
}

func makeArgs(argstring string, ConfigArgsmap map[string]string, FunctionArgsmap map[string]string) []string {
	args := strings.Split(argstring, " ")
	argsmap := map[string]string{}
	if ConfigArgsmap != nil {
		for k, v := range ConfigArgsmap {
			argsmap[k] = v
		}
	}
	if FunctionArgsmap != nil {
		for k, v := range FunctionArgsmap {
			argsmap[k] = v
		}
	}
	for k, v := range argsmap {
		args = append(args, k)
		if v != "" {
			args = append(args, v)
		}
	}
	return args
}

func (o *OpExecutable) execBin(ctx *base.Context, args []string, env map[string]string, input *base.OperatorIO) *base.OperatorIO {
	var err error

	e := exec.Command(o.conf.Path, args...)
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

	return base.MakeByteOutputWithContentType(byt, o.conf.OutputContentType)
}

func (o *OpExecutable) runInBackground(ctx *base.Context, args []string, input *base.OperatorIO) *base.OperatorIO {
	o.processLock.Lock()
	defer o.processLock.Unlock()

	if o.cmd != nil {
		return base.MakeOutputError(http.StatusConflict, "Error executing %v in background, it's already running", o.name)
	}

	o.bgOutput.Reset()
	o.cmd = exec.Command(o.conf.Path, args...)
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

func (o *OpExecutable) stopBackground(ctx *base.Context) *base.OperatorIO {
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

type RunArgs struct {
	ArgsString string
}

func (o *OpExecutable) Run(ctx *base.Context, input *base.OperatorIO, runArgs RunArgs, fa base.FunctionArguments) *base.OperatorIO {
	args := makeArgs(runArgs.ArgsString, o.conf.DefaultArguments, fa.GetOriginalCaseMapJoined())
	return o.execBin(ctx, args, nil, input)
}

func (o *OpExecutable) RunInBackground(ctx *base.Context, fn string, vars map[string]string, input *base.OperatorIO) *base.OperatorIO {
	return base.MakeEmptyOutput()
}

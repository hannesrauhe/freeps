package freepsgraph

import (
	"encoding/json"
	"fmt"
)

type OutputT string

const (
	Empty  OutputT = ""
	Error  OutputT = "error"
	String OutputT = "string"
)

type OperatorIO struct {
	OutputType OutputT
	HTTPCode   uint32
	Output     interface{}
}

func MakeOutputError(code uint32, msg string, a ...interface{}) *OperatorIO {
	err := fmt.Errorf(msg, a...)
	return &OperatorIO{OutputType: Error, HTTPCode: code, Output: err}
}

func MakeEmptyOutput() *OperatorIO {
	return &OperatorIO{OutputType: Empty, HTTPCode: 200, Output: nil}
}

func (io *OperatorIO) GetMap() (map[string]string, error) {
	v, ok := io.Output.(map[string]string)
	if ok {
		return v, nil
	}
	return nil, fmt.Errorf("Output is not of type map")
}

func (io *OperatorIO) IsError() bool {
	return io.OutputType == Error
}

func (io *OperatorIO) ToString() string {
	if io.IsError() {
		return fmt.Sprintf("Error Code: %v,\n%v\n", io.HTTPCode, io.Output.(error))
	} else {
		o, _ := json.MarshalIndent(io.Output, "", "  ")
		return fmt.Sprintf("Error Code: %v,\nOutput Type: %T,\n%v\n", io.HTTPCode, io.Output, string(o))
	}
}

package freepsgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type OutputT string

const (
	Empty  OutputT = ""
	Error  OutputT = "error"
	String OutputT = "string"
	Byte   OutputT = "byte"
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

func MakeByteOutput(output []byte) *OperatorIO {
	return &OperatorIO{OutputType: Byte, HTTPCode: 200, Output: output}
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

func (oio *OperatorIO) ToString() string {
	b := bytes.NewBufferString("")
	oio.WriteTo(b)
	return b.String()
}

func (oio *OperatorIO) WriteTo(bwriter io.Writer) {
	if oio.IsError() {
		fmt.Fprintf(bwriter, "Error Code: %v,\n%v\n", oio.HTTPCode, oio.Output.(error))
		return
	}
	fmt.Fprintf(bwriter, "Error Code: %v,\nOutput Type: %T,\n", oio.HTTPCode, oio.Output)
	if oio.OutputType == Byte {
		bwriter.Write(oio.Output.([]byte))
		return
	}
	o, _ := json.MarshalIndent(oio.Output, "", "  ")
	fmt.Fprintf(bwriter, "%v\n", string(o))
}

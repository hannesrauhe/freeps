package freepsgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type OutputT string

const (
	Empty     OutputT = ""
	Error     OutputT = "error"
	PlainText OutputT = "plain"
	Byte      OutputT = "byte"
	Object    OutputT = "object"
)

type OperatorIO struct {
	OutputType  OutputT
	HTTPCode    uint32
	Output      interface{}
	ContentType string
}

func MakeOutputError(code uint32, msg string, a ...interface{}) *OperatorIO {
	err := fmt.Errorf(msg, a...)
	return &OperatorIO{OutputType: Error, HTTPCode: code, Output: err}
}

func MakeEmptyOutput() *OperatorIO {
	return &OperatorIO{OutputType: Empty, HTTPCode: 200, Output: nil}
}

func MakePlainOutput(msg string, a ...interface{}) *OperatorIO {
	return &OperatorIO{OutputType: PlainText, HTTPCode: 200, Output: fmt.Sprintf(msg, a...)}
}

func MakeByteOutput(output []byte) *OperatorIO {
	return &OperatorIO{OutputType: Byte, HTTPCode: 200, Output: output}
}

func MakeByteOutputWithContentType(output []byte, contentType string) *OperatorIO {
	return &OperatorIO{OutputType: Byte, HTTPCode: 200, Output: output, ContentType: contentType}
}

func MakeObjectOutput(output interface{}) *OperatorIO {
	return &OperatorIO{OutputType: Object, HTTPCode: 200, Output: output}
}

func (io *OperatorIO) GetMap() (map[string]string, error) {
	v, ok := io.Output.(map[string]string)
	if ok {
		return v, nil
	}
	return nil, fmt.Errorf("Output is not of type map")
}

func (io *OperatorIO) ParseJSON(obj interface{}) error {
	if io.OutputType == Byte {
		v, ok := io.Output.([]byte)
		if ok {
			return json.Unmarshal(v, obj)
		}
	}

	return fmt.Errorf("Output is not of type %v and cannot be parsed to JSON")
}

func (io *OperatorIO) GetBytes() ([]byte, error) {
	switch io.OutputType {
	case Empty:
		return make([]byte, 0), nil
	case Byte:
		return io.Output.([]byte), nil
	case PlainText:
		return []byte(io.Output.(string)), nil
	case Error:
		return []byte(io.Output.(error).Error()), nil
	default:
		return json.MarshalIndent(io.Output, "", "  ")
	}
}

func (io *OperatorIO) IsError() bool {
	return io.OutputType == Error
}

func (io *OperatorIO) IsEmpty() bool {
	switch io.OutputType {
	case Empty:
		return true
	case Byte:
		return len(io.Output.([]byte)) == 0
	case PlainText:
		return len(io.Output.(string)) == 0
	default:
		return false
	}
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

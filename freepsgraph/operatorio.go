package freepsgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/sirupsen/logrus"
)

type OutputT string

const (
	Empty     OutputT = ""
	Error     OutputT = "error"
	PlainText OutputT = "plain"
	Byte      OutputT = "byte"
	Object    OutputT = "object"
)

// OperatorIO is the input and output of an operator, once created it should not be modified
// Note: the Store Operator depends on this struct being immutable
type OperatorIO struct {
	OutputType  OutputT
	HTTPCode    int
	Output      interface{}
	ContentType string `json:",omitempty"`
}

func MakeOutputError(code int, msg string, a ...interface{}) *OperatorIO {
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

func (io *OperatorIO) GetArgsMap() (map[string]string, error) {
	strmap, ok := io.Output.(map[string]string)
	if ok {
		return strmap, nil
	}
	generalmap, ok := io.Output.(map[string]interface{})
	if ok {
		strmap := make(map[string]string)
		for k, v := range generalmap {
			strmap[k] = fmt.Sprintf("%v", v)
		}
		return strmap, nil
	}
	opmap, ok := io.Output.(map[string]*OperatorIO)
	if ok {
		strmap := make(map[string]string)
		for k, v := range opmap {
			strmap[k] = fmt.Sprintf("%v", v)
		}
		return strmap, nil
	}
	if io.IsEmpty() {
		return map[string]string{}, nil
	}
	return nil, fmt.Errorf("Output is not of type map, but %T", io.Output)
}

func (io *OperatorIO) ParseJSON(obj interface{}) error {
	if io.OutputType == Byte {
		v, ok := io.Output.([]byte)
		if ok {
			return json.Unmarshal(v, obj)
		}
	} else {
		byt, err := json.Marshal(io.Output)
		if err != nil {
			return err
		}
		return json.Unmarshal(byt, obj)
	}

	return fmt.Errorf("Output is of type \"%v\" and cannot be parsed to JSON", io.OutputType)
}

// ParseFormData returns the data as posted by a HTML form
func (io *OperatorIO) ParseFormData() (url.Values, error) {
	inBytes, err := io.GetBytes()
	if err != nil {
		return nil, err
	}
	return url.ParseQuery(string(inBytes))

	// return fmt.Errorf("Output is of type \"%v\" and does not look like form data", io.OutputType)
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

func (io *OperatorIO) GetString() string {
	switch io.OutputType {
	case Empty:
		return ""
	case Byte:
		b := io.Output.([]byte)
		if len(b) > 1024*10 {
			return string(b[:1024*10]) + "..."
		}
		return string(b)
	case PlainText:
		return io.Output.(string)
	case Error:
		return io.Output.(error).Error()
	default:
		byt, _ := json.MarshalIndent(io.Output, "", "  ")
		return string(byt)
	}
}

func (io *OperatorIO) GetError() error {
	switch io.OutputType {
	case Error:
		return io.Output.(error)
	default:
		return nil
	}
}

func (io *OperatorIO) GetStatusCode() int {
	return io.HTTPCode
}

func (io *OperatorIO) IsError() bool {
	return io.OutputType == Error
}

func (io *OperatorIO) IsPlain() bool {
	return io.OutputType == PlainText
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

func (oio *OperatorIO) Log(logger logrus.FieldLogger) {
	if logger == nil {
		logger = logrus.StandardLogger()
		logger.Warnf("No logger provided to OperatorIO.Log, using standard logger")
	}
	logline := "Output: " + oio.ToString()
	if len(logline) > 1000 {
		logger.Debugf(logline)
		logline = logline[:1000] + "..."
	}
	logger.Infof(logline)
}

func (oio *OperatorIO) ToString() string {
	b := bytes.NewBufferString("")
	oio.WriteTo(b)
	return b.String()
}

func (oio *OperatorIO) WriteTo(bwriter io.Writer) (int, error) {
	if oio.IsError() {
		return fmt.Fprintf(bwriter, "Error Code: %v, %v", oio.HTTPCode, oio.Output.(error))
	}
	fmt.Fprintf(bwriter, "Output Type: %T", oio.Output)
	if oio.OutputType == Byte {
		return bwriter.Write(oio.Output.([]byte))
	}
	o, _ := json.MarshalIndent(oio.Output, "", "  ")
	return fmt.Fprintf(bwriter, "%v", string(o))
}

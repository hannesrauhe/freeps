package base

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"

	"github.com/hannesrauhe/freeps/utils"
	"github.com/jeremywohl/flatten"
	"github.com/sirupsen/logrus"
)

type OutputT string

const (
	Empty         OutputT = ""
	Error         OutputT = "error"
	PlainText     OutputT = "plain"
	Byte          OutputT = "byte"
	Object        OutputT = "object"
	Integer       OutputT = "integer"
	FloatingPoint OutputT = "floating"
)

const MAXSTRINGLENGTH = 1024 * 10

// OperatorIO is the input and output of an operator, once created it should not be modified
// Note: the Store Operator depends on this struct being immutable
type OperatorIO struct {
	OutputType  OutputT
	HTTPCode    int
	Output      interface{}
	ContentType string `json:",omitempty"`
}

func MakeErrorOutputFromError(err error) *OperatorIO {
	return &OperatorIO{OutputType: Error, HTTPCode: http.StatusInternalServerError, Output: err}
}

func MakeOutputError(code int, msg string, a ...interface{}) *OperatorIO {
	err := fmt.Errorf(msg, a...)
	return &OperatorIO{OutputType: Error, HTTPCode: code, Output: err}
}

func MakeEmptyOutput() *OperatorIO {
	return &OperatorIO{OutputType: Empty, HTTPCode: 200, Output: nil}
}

func MakeSprintfOutput(msg string, a ...interface{}) *OperatorIO {
	return &OperatorIO{OutputType: PlainText, HTTPCode: 200, Output: fmt.Sprintf(msg, a...)}
}

func MakePlainOutput(msg string) *OperatorIO {
	return &OperatorIO{OutputType: PlainText, HTTPCode: 200, Output: msg}
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

func MakeObjectOutputWithContentType(output interface{}, contentType string) *OperatorIO {
	return &OperatorIO{OutputType: Object, HTTPCode: 200, Output: output, ContentType: contentType}
}

func MakeIntegerOutput(output interface{}) *OperatorIO {
	/* panic if output is not numeric */
	switch output.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		break
	default:
		panic(fmt.Sprintf("Cannot make integer output, type is %T", output))
	}
	return &OperatorIO{OutputType: Integer, HTTPCode: 200, Output: output}
}

func MakeFloatOutput(output interface{}) *OperatorIO {
	/* panic if output is not numeric */
	switch output.(type) {
	case float32, float64:
		break
	default:
		panic(fmt.Sprintf("Cannot make floating point output, type is %T", output))
	}
	return &OperatorIO{OutputType: FloatingPoint, HTTPCode: 200, Output: output}
}

func (io *OperatorIO) GetArgsMap() (map[string]string, error) {
	if io.IsEmpty() {
		return map[string]string{}, nil
	}

	strmap := make(map[string]string)

	switch t := io.Output.(type) {
	case map[string]string:
		return t, nil
	case map[string]interface{}:
		for k, v := range t {
			strmap[k] = fmt.Sprintf("%v", v)
		}
		return strmap, nil
	case map[string]*OperatorIO:
		for k, v := range t {
			strmap[k] = fmt.Sprintf("%v", v)
		}
		return strmap, nil
	}

	if io.ParseJSON(&strmap) == nil {
		return strmap, nil
	}

	generalmap := map[string]interface{}{}
	err := io.ParseJSON(&generalmap)
	if err == nil {
		interfacemap, err := flatten.Flatten(generalmap, "", flatten.DotStyle)
		if err == nil {
			for k, v := range interfacemap {
				strmap[k] = fmt.Sprintf("%v", v)
			}
			return strmap, nil
		}
	}

	return nil, fmt.Errorf("Output is not convertible to type string map, type is %T", io.Output)
}

func (io *OperatorIO) GetMap() (map[string]interface{}, error) {
	if io.IsEmpty() {
		return map[string]interface{}{}, nil
	}

	switch t := io.Output.(type) {
	case map[string]interface{}:
		return t, nil
	}

	generalmap := map[string]interface{}{}
	if io.ParseJSON(&generalmap) == nil {
		return generalmap, nil
	}

	return nil, fmt.Errorf("Output is not convertible to type map, type is %T", io.Output)
}

func (io *OperatorIO) ParseJSON(obj interface{}) error {
	if io.IsEmpty() {
		return nil
	}
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
	if io.IsFormData() {
		formData, isType := io.Output.(url.Values)
		if !isType {
			return nil, fmt.Errorf("Input should be form data but cannot be interpreted as such")
		}
		return formData, nil
	}
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

func (io *OperatorIO) GetSize() (int, error) {
	switch io.OutputType {
	case Empty:
		return 0, nil
	case Byte:
		return len(io.Output.([]byte)), nil
	case PlainText:
		return len(io.Output.(string)), nil
	case Error:
		return len(io.Output.(error).Error()), nil
	default:
		return 0, fmt.Errorf("Size not available")
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

func (io *OperatorIO) GetObject() interface{} {
	switch io.OutputType {
	case Object:
		return io.Output
	default:
		return nil
	}
}

// GetString returns the output as a string, it will convert to string if not already, returns "" if not possible
func (io *OperatorIO) GetString() string {
	var b []byte
	switch io.OutputType {
	case Empty:
		return ""
	case Byte:
		b = io.Output.([]byte)
	case PlainText:
		return io.Output.(string)
	case Error:
		return io.Output.(error).Error()
	case Integer, FloatingPoint:
		return fmt.Sprintf("%v", io.Output)
	default:
		b, _ = json.MarshalIndent(io.Output, "", "  ")
	}

	if len(b) > MAXSTRINGLENGTH {
		return fmt.Sprintf("%s...", b[:MAXSTRINGLENGTH-3])
	}
	return fmt.Sprintf("%s", b)
}

// GetFloat64 returns the output as a float64, it will convert/round possibly losing precision, returns NaN if not possible
// convert: if true, it will convert to float64 if not already, if false it will return NaN if not already a floating point type (it will always convert floating point types)
func (io *OperatorIO) GetFloat64(convert bool) float64 {
	if !convert {
		switch io.OutputType {
		case FloatingPoint:
			break
		default:
			return math.NaN()
		}
	}
	f, err := utils.ConvertToFloat(io.Output)
	if err != nil {
		return math.NaN()
	}
	return f
}

// GetInt64 returns the output as an int64, it will convert/round possibly losing precision, returns 0 if not possible
// convert: if true, it will convert to int64 if not already, if false it will return 0 if not already an integer type (it will always convert integer types)
func (io *OperatorIO) GetInt64(convert bool) (int64, error) {
	if !convert {
		switch io.OutputType {
		case Integer:
			break
		default:
			return 0, fmt.Errorf("Output is not an integer")
		}
	}
	i, err := utils.ConvertToInt64(io.Output)
	if err != nil {
		return 0, fmt.Errorf("Cannot convert output to integer: %v", err)
	}
	return i, nil
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

func (io *OperatorIO) IsObject() bool {
	return io.OutputType == Object
}

func (io *OperatorIO) IsByte() bool {
	return io.OutputType == Byte
}

func (io *OperatorIO) IsInteger() bool {
	return io.OutputType == Integer
}

func (io *OperatorIO) IsFloatingPoint() bool {
	return io.OutputType == FloatingPoint
}

func (io *OperatorIO) IsFormData() bool {
	return io.IsObject() && io.ContentType == "application/x-www-form-urlencoded"
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
	logger.Debug(logline)
}

func (oio *OperatorIO) ToString() string {
	b := bytes.NewBufferString("")
	oio.WriteTo(b, MAXSTRINGLENGTH)
	return b.String()
}

func (oio *OperatorIO) WriteTo(bwriter io.Writer, maxLen int) (int, error) {
	if oio.IsError() {
		return fmt.Fprintf(bwriter, "Error Code: %v, %v", oio.HTTPCode, oio.Output.(error))
	}

	if oio.HTTPCode != 200 {
		fmt.Fprintf(bwriter, "HTTP Code: %v, ", oio.HTTPCode)
	}
	if oio.IsEmpty() {
		return fmt.Fprintf(bwriter, "Empty Output")
	}

	if oio.ContentType != "" {
		fmt.Fprintf(bwriter, "Content Type: %v, ", oio.ContentType)
	}

	if oio.OutputType == PlainText {
		if len(oio.Output.(string)) > maxLen {
			return fmt.Fprintf(bwriter, "%v...", oio.Output.(string)[:maxLen])
		}
		return fmt.Fprintf(bwriter, "%v", oio.Output.(string))
	}

	fmt.Fprintf(bwriter, "Output Type: %v, Data Type: %T, Value: ", oio.OutputType, oio.Output)

	var o []byte

	if oio.OutputType == Byte {
		o, _ = oio.Output.([]byte)
	} else {
		o, _ = json.MarshalIndent(oio.Output, "", "  ")
		// return fmt.Fprintf(bwriter, "%v", string(o)[:maxLen])
	}

	// write the first 1024 bytes of the output
	if len(o) > maxLen {
		return bwriter.Write(o[:maxLen])
	}
	return bwriter.Write(o)
}

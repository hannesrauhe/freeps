package freepsgraph

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/jeremywohl/flatten"
)

type MessageAndTime struct {
	msg     []byte
	expires time.Time
	counter int
}

type OpEval struct {
	lastMessage MessageAndTime
}

var _ base.FreepsOperator = &OpEval{}

type EvalArgs struct {
	ValueName string
	ValueType string
	Operation string
	Operand   interface{}
	Output    string
}

type DedupArgs struct {
	Retention string
}

// GetName returns the name of the operator
func (o *OpEval) GetName() string {
	return "eval"
}

func (m *OpEval) Execute(ctx *base.Context, fn string, vars map[string]string, input *base.OperatorIO) *base.OperatorIO {
	switch fn {
	case "echo":
		if m, ok := vars["output"]; ok {
			return base.MakePlainOutput(m)
		}
		return base.MakeEmptyOutput()
	case "hasInput":
		if input.IsEmpty() {
			return base.MakeOutputError(http.StatusBadRequest, "Expected input")
		}
		return input
	case "formToJSON":
		o, err := input.ParseFormData()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "input not valid form data: %v", err)
		}
		o2 := utils.URLArgsToMap(o)
		return base.MakeObjectOutput(o2)
	case "echoArguments":
		output := map[string]interface{}{}
		if !input.IsEmpty() {
			if m, ok := vars["inputKey"]; ok {
				output[m] = input.Output
			} else {
				iMap, err := input.GetArgsMap()
				if err != nil {
					return base.MakeOutputError(http.StatusBadRequest, "input cannot be converted to map[string]string, assign inputKey")
				}
				for k, v := range iMap {
					output[k] = v
				}
			}
		}
		for k, v := range vars {
			if k == "inputKey" {
				continue
			}
			output[k] = v
		}
		return base.MakeObjectOutput(output)
	case "flatten":
		return m.Flatten(vars, input)
	case "eval":
		return m.Eval(vars, input)
	case "split":
		return m.Split(vars, input)
	case "regexp":
		return m.Regexp(vars, input)
	case "strreplace":
		return base.MakePlainOutput(strings.Replace(input.GetString(), vars["search"], vars["replace"], -1))
	case "dedup":
		var args DedupArgs
		err := utils.ArgsMapToObject(vars, &args)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "request cannot be parsed into a map: %v", err)
		}
		retDur, err := time.ParseDuration(args.Retention)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "cannot parse retention time: %v", err)
		}
		b, err := input.GetBytes()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "cannot get bytes of message [implementation error?]: %v", err)
		}
		if time.Now().Before(m.lastMessage.expires) && bytes.Compare(m.lastMessage.msg, b) == 0 {
			m.lastMessage.expires = time.Now().Add(retDur)
			m.lastMessage.counter++
			return base.MakeOutputError(http.StatusConflict, "Msg received %v times", m.lastMessage.counter)
		}
		m.lastMessage = MessageAndTime{msg: b, expires: time.Now().Add(retDur), counter: 1}

		return input
	}
	return base.MakeOutputError(http.StatusBadRequest, "No such function \"%v\"", fn)
}

func (m *OpEval) GetFunctions() []string {
	return []string{"eval", "regexp", "dedup", "echo", "echoArguments", "flatten", "strreplace", "split", "formToJSON", "hasInput"}
}

func (m *OpEval) GetPossibleArgs(fn string) []string {
	if fn == "echo" {
		return []string{"output"}
	}
	if fn == "echoArguments" {
		return []string{"inputKey"}
	}
	if fn == "dedup" {
		return []string{"retention"}
	}
	ret := []string{"valueName", "valueType", "operation", "operand"}
	return ret
}

func (m *OpEval) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	if fn == "echo" {
		return map[string]string{"output": "output"}
	}
	switch arg {
	case "valueType":
		return map[string]string{"int": "int", "string": "string", "bool": "bool", "float": "float"}
	case "operation":
		return map[string]string{"eq": "eq", "gt": "gt", "lt": "lt", "id": "id"}
	case "retention":
		return utils.GetDurationMap()
	}
	return map[string]string{}
}

func (m *OpEval) Flatten(vars map[string]string, input *base.OperatorIO) *base.OperatorIO {
	nestedArgsMap := map[string]interface{}{}
	err := input.ParseJSON(&nestedArgsMap)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a map")
	}

	argsmap, err := flatten.Flatten(nestedArgsMap, "", flatten.DotStyle)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a flat map: %v", err)
	}
	return base.MakeObjectOutput(argsmap)
}

func (m *OpEval) Eval(vars map[string]string, input *base.OperatorIO) *base.OperatorIO {
	var args EvalArgs
	err := utils.ArgsMapToObject(vars, &args)
	if err != nil || args.ValueName == "" || args.ValueType == "" {
		return base.MakeOutputError(http.StatusBadRequest, "Missing args")
	}

	nestedArgsMap := map[string]interface{}{}
	err = input.ParseJSON(&nestedArgsMap)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a map")
	}

	argsmap, err := flatten.Flatten(nestedArgsMap, "", flatten.DotStyle)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a flat map: %v", err)
	}

	vInterface, ok := argsmap[args.ValueName]
	if !ok {
		return base.MakeOutputError(http.StatusBadRequest, "expected value %s in request", args.ValueName)
	}

	if args.Operation == "id" {
		return base.MakeObjectOutput(vInterface)
	}

	result := false
	switch args.ValueType {
	case "int":
		result, err = m.EvalInt(vInterface, args.Operation, args.Operand)
	case "float":
		result, err = m.EvalFloat(vInterface, args.Operation, args.Operand)
	case "string":
		result, err = m.EvalString(vInterface, args.Operation, args.Operand)
	case "bool":
		result, err = parseBoolOrReturnDirectly(vInterface)
	default:
		err = fmt.Errorf("No such type %s", args.ValueType)
	}
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err)
	}
	if result {
		switch args.Output {
		case "flat":
			fallthrough
		case "args":
			{
				return base.MakeObjectOutput(argsmap)
			}
		case "input":
			{
				return input
			}
		default:
			return base.MakeEmptyOutput()
		}
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "Eval %v resulted in false", vars)
}

func (m *OpEval) EvalInt(vInterface interface{}, op string, v2Interface interface{}) (bool, error) {
	v, err := parseIntOrReturnDirectly(vInterface)
	if err != nil {
		return false, err
	}
	v2, err := parseIntOrReturnDirectly(v2Interface)
	if err != nil {
		return false, err
	}
	switch op {
	case "lt":
		return v < v2, nil
	case "gt":
		return v > v2, nil
	case "eq":
		return v == v2, nil
	}
	return false, fmt.Errorf("No such operation \"%s\"", op)
}

func (m *OpEval) EvalFloat(vInterface interface{}, op string, v2Interface interface{}) (bool, error) {
	v, err := parseFloatOrReturnDirectly(vInterface)
	if err != nil {
		return false, err
	}
	v2, err := parseFloatOrReturnDirectly(v2Interface)
	if err != nil {
		return false, err
	}
	switch op {
	case "lt":
		return v < v2, nil
	case "gt":
		return v > v2, nil
	}
	return false, fmt.Errorf("No such operation \"%s\"", op)
}

func (m *OpEval) EvalString(vInterface interface{}, op string, v2Interface interface{}) (bool, error) {
	v, err := parseStringOrReturnDirectly(vInterface)
	if err != nil {
		return false, err
	}
	v2, err := parseStringOrReturnDirectly(v2Interface)
	if err != nil {
		return false, err
	}
	switch op {
	case "lt":
		return v < v2, nil
	case "gt":
		return v > v2, nil
	case "eq":
		return v == v2, nil
	}
	return false, fmt.Errorf("No such operation \"%s\"", op)
}

func (m *OpEval) Regexp(args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	re, err := regexp.Compile(args["regexp"])
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Invalid regexp: %v", err)
	}
	switch args["operation"] {
	case "find":
		loc := re.FindStringIndex(input.GetString())
		if loc == nil {
			return base.MakeOutputError(http.StatusExpectationFailed, "No match")
		}
		return base.MakePlainOutput(input.GetString()[loc[0]:loc[1]])
	case "findstringsubmatch":
		loc := re.FindStringSubmatchIndex(input.GetString())
		if loc == nil {
			return base.MakeOutputError(http.StatusExpectationFailed, "No match")
		}
		return base.MakePlainOutput(input.GetString()[loc[2]:loc[3]])
	}
	return base.MakeOutputError(http.StatusBadRequest, "No such op %s", args["op"])
}

func (m *OpEval) Split(argsmap map[string]string, input *base.OperatorIO) *base.OperatorIO {
	sep := argsmap["sep"]
	if sep == "" {
		return base.MakeOutputError(http.StatusBadRequest, "Need a separator (sep) to split")
	}
	strArray := strings.Split(input.GetString(), sep)

	posStr := argsmap["pos"]
	if posStr == "" {
		return base.MakeObjectOutput(strArray)
	}
	pos, err := parseIntOrReturnDirectly(posStr)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v is not an integer: %v", posStr, err.Error())
	}
	if pos >= len(strArray) {
		return base.MakeOutputError(http.StatusBadRequest, "Pos %v not available in array %v", pos, strArray)
	}
	return base.MakePlainOutput(strArray[pos])
}

// Shutdown (noOp)
func (o *OpEval) Shutdown(ctx *base.Context) {
}

func parseIntOrReturnDirectly(v interface{}) (int, error) {
	switch v.(type) {
	case int:
		return v.(int), nil
	case int64:
		return int(v.(int64)), nil
	case int32:
		return int(v.(int32)), nil
	case float64:
		return int(math.Round(v.(float64))), nil
	case []byte:
		b := v.([]byte)
		if len(b) == 0 {
			return 0, fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as Int, array is empty", v, v)
		}
		return int(b[0]), nil
	case string:
		vInt, err := strconv.Atoi(v.(string))
		if err != nil {
			return 0, err
		}
		return vInt, nil
	}
	return 0, fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as Int", v, v)
}

func parseFloatOrReturnDirectly(v interface{}) (float64, error) {
	switch v.(type) {
	case int:
		return float64(v.(int)), nil
	case int64:
		return float64(v.(int64)), nil
	case int32:
		return float64(v.(int32)), nil
	case float64:
		return v.(float64), nil
	case string:
		vF, err := strconv.ParseFloat(v.(string), 64)
		if err != nil {
			return 0, err
		}
		return vF, nil
	}
	return 0, fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as Float64", v, v)
}

func parseBoolOrReturnDirectly(v interface{}) (bool, error) {
	switch v.(type) {
	case bool:
		return v.(bool), nil
	case []byte:
		b := v.([]byte)
		if len(b) == 0 {
			return false, fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as bool, array is empty", v, v)
		}
		return b[0] != 0, nil
	case string:
		vB, err := strconv.ParseBool(v.(string))
		if err != nil {
			return false, err
		}
		return vB, nil
	}
	return false, fmt.Errorf("Cannot parse \"%v\" of type \"%T\"  as Bool", v, v)
}

func parseStringOrReturnDirectly(v interface{}) (string, error) {
	switch v.(type) {
	case string:
		return v.(string), nil
	case bool:
		return strconv.FormatBool(v.(bool)), nil
	case int:
		return strconv.Itoa(v.(int)), nil
	case int64:
		return strconv.FormatInt(v.(int64), 10), nil
	case int32:
		return strconv.FormatInt(int64(v.(int32)), 10), nil
	case float64:
		return strconv.FormatFloat(v.(float64), 'f', -1, 64), nil
	}
	return "", fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as String", v, v)
}

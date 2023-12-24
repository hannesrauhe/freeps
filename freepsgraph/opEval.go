package freepsgraph

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
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

var _ base.FreepsBaseOperator = &OpEval{}

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
		result, err = utils.ConvertToBool(vInterface)
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
	v, err := utils.ConvertToInt64(vInterface)
	if err != nil {
		return false, err
	}
	v2, err := utils.ConvertToInt64(v2Interface)
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
	v, err := utils.ConvertToFloat(vInterface)
	if err != nil {
		return false, err
	}
	v2, err := utils.ConvertToFloat(v2Interface)
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
	v, err := utils.ConvertToString(vInterface)
	if err != nil {
		return false, err
	}
	v2, err := utils.ConvertToString(v2Interface)
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
	pos, err := utils.ConvertToInt64(posStr)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v is not an integer: %v", posStr, err.Error())
	}
	if pos >= int64(len(strArray)) {
		return base.MakeOutputError(http.StatusBadRequest, "Pos %v not available in array %v", pos, strArray)
	}
	return base.MakePlainOutput(strArray[pos])
}

// StartListening (noOp)
func (o *OpEval) StartListening(ctx *base.Context) {
}

// Shutdown (noOp)
func (o *OpEval) Shutdown(ctx *base.Context) {
}

// GetHook returns the hook for this operator
func (o *OpEval) GetHook() interface{} {
	return nil
}

// GetTriggers returns a list of triggers for this operator
func (o *OpEval) GetTriggers() []base.FreepsTrigger {
	return []base.FreepsTrigger{}
}

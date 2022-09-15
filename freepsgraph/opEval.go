package freepsgraph

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"time"

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

var _ FreepsOperator = &OpEval{}

type EvalArgs struct {
	ValueName string
	ValueType string
	Operation string
	Operand   interface{}
}

type DedupArgs struct {
	Retention string
}

func (m *OpEval) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "echo":
		if m, ok := vars["output"]; ok {
			return MakePlainOutput(m)
		}
		return MakeEmptyOutput()
	case "eval":
		fallthrough
	case "regexp":
		return m.EvalAndRegexp(fn, vars, input)
	case "dedup":
		var args DedupArgs
		err := utils.ArgsMapToObject(vars, &args)
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "request cannot be parsed into a map: %v", err)
		}
		retDur, err := time.ParseDuration(args.Retention)
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "cannot parse retention time: %v", err)
		}
		b, err := input.GetBytes()
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "cannot get bytes of message [implementation error?]: %v", err)
		}
		if time.Now().Before(m.lastMessage.expires) && bytes.Compare(m.lastMessage.msg, b) == 0 {
			m.lastMessage.expires = time.Now().Add(retDur)
			m.lastMessage.counter++
			return MakeOutputError(http.StatusConflict, "Msg received %v times", m.lastMessage.counter)
		}
		m.lastMessage = MessageAndTime{msg: b, expires: time.Now().Add(retDur), counter: 1}

		return input
	}
	return MakeOutputError(http.StatusBadRequest, "No such function \"%v\"", fn)
}

func (m *OpEval) GetFunctions() []string {
	return []string{"eval", "regexp", "dedup"}
}

func (m *OpEval) GetPossibleArgs(fn string) []string {
	if fn == "dedup" {
		return []string{"retention"}
	}
	ret := []string{"valueName", "valueType", "operation", "operand"}
	return ret
}

func (m *OpEval) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	switch arg {
	case "valueType":
		return map[string]string{"int": "int"}
	case "operation":
		return map[string]string{"eq": "eq", "gt": "gt", "lt": "lt"}
	case "retention":
		return map[string]string{"1s": "1s", "10s": "10s", "100s": "100s"}
	}
	return map[string]string{}
}

func (m *OpEval) EvalAndRegexp(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	var args EvalArgs
	err := utils.ArgsMapToObject(vars, &args)
	if err != nil || args.ValueName == "" || args.ValueType == "" {
		return MakeOutputError(http.StatusBadRequest, "Missing args")
	}

	nestedArgsMap := map[string]interface{}{}
	err = input.ParseJSON(&nestedArgsMap)
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, "request cannot be parsed into a map")
	}

	argsmap, err := flatten.Flatten(nestedArgsMap, "", flatten.DotStyle)
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, "request cannot be parsed into a flat map: %v", err)
	}

	vInterface, ok := argsmap[args.ValueName]
	if !ok {
		return MakeOutputError(http.StatusBadRequest, "expected value %s in request", args.ValueName)
	}

	switch fn {
	case "eval":
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
			return MakeOutputError(http.StatusBadRequest, "%v", err)
		}
		if result {
			return input
		} else {
			return MakeOutputError(http.StatusExpectationFailed, "Eval %v resulted in false", vars)
		}
	case "regexp":
		resultString, err := m.Regexp(vInterface, args.Operation, args.Operand)
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, "%v", err)
		}
		return MakePlainOutput(resultString)
	}
	return MakeOutputError(http.StatusBadRequest, "Unknown function %v", fn)
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

func (m *OpEval) Regexp(vInterface interface{}, op string, regexpInterface interface{}) (string, error) {
	v, err := parseStringOrReturnDirectly(vInterface)
	if err != nil {
		return "", err
	}
	regexpString, err := parseStringOrReturnDirectly(regexpInterface)
	if err != nil {
		return "", err
	}
	re, err := regexp.Compile(regexpString)
	if err != nil {
		return "", err
	}
	switch op {
	case "find":
		loc := re.FindStringIndex(v)
		if loc == nil {
			return "", fmt.Errorf("No match")
		}
		return v[loc[0]:loc[1]], nil
	}
	return "", fmt.Errorf("No such operation \"%s\"", op)
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
	}
	return "", fmt.Errorf("Cannot parse \"%v\" of type \"%T\"  as String", v, v)
}

package freepsdo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jeremywohl/flatten"
)

type EvalMod struct {
}

var _ Mod = &EvalMod{}

type EvalArgs struct {
	ValueName string
	ValueType string
	Operation string
	Operand   string
}

func (m *EvalMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	var args EvalArgs
	err := json.Unmarshal(jsonStr, &args)
	if err != nil || args.ValueName == "" || args.ValueType == "" || args.Operation == "" || args.Operand == "" {
		jrw.WriteError(http.StatusBadRequest, "request cannot be parsed or is missing a value")
		return
	}
	nestedArgsMap := map[string]interface{}{}
	err = json.Unmarshal(jsonStr, &nestedArgsMap)
	if err != nil {
		jrw.WriteError(http.StatusBadRequest, "request cannot be parsed into a map")
		return
	}

	argsmap, err := flatten.Flatten(nestedArgsMap, "", flatten.DotStyle)
	if err != nil {
		jrw.WriteError(http.StatusBadRequest, "request cannot be parsed into a flat map")
		return
	}

	vInterface, ok := argsmap[args.ValueName]
	if !ok {
		jrw.WriteError(http.StatusBadRequest, "expected value %s in request", args.ValueName)
		return
	}
	v, ok := vInterface.(string)
	if !ok {
		jrw.WriteError(http.StatusBadRequest, "value %s needs to be a string to be parsed", args.ValueName)
		return
	}

	result := false
	switch args.ValueType {
	case "int":
		result, err = m.EvalInt(v, args.Operation, args.Operand)
	default:
		err = fmt.Errorf("No such type %s", args.ValueType)
	}
	if err != nil {
		jrw.WriteError(http.StatusBadRequest, "%v", err)
	}
	if result {
		jrw.WriteSuccessMessage(nestedArgsMap)
	} else {
		jrw.WriteMessageWithCodef(http.StatusExpectationFailed, "Eval resulted in false")
	}
}

func (m *EvalMod) GetFunctions() []string {
	return []string{"eval"}
}

func (m *EvalMod) GetPossibleArgs(fn string) []string {
	ret := []string{"valueName", "valueType", "operation", "operand"}
	return ret
}

func (m *EvalMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	switch arg {
	case "valueType":
		return map[string]string{"int": "int"}
	case "operation":
		return map[string]string{"eq": "eq", "gt": "gt", "lt": "lt"}
	}
	return map[string]string{}
}

func (m *EvalMod) EvalInt(vStr string, op string, v2Str string) (bool, error) {
	v, err := strconv.Atoi(vStr)
	if err != nil {
		return false, err
	}
	v2, err := strconv.Atoi(v2Str)
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

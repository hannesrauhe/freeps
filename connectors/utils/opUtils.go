package freepsutils

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/jeremywohl/flatten"
)

// OpUtils is a collection of utility operations
type OpUtils struct {
}

var _ base.FreepsOperator = &OpUtils{}

// FlattenArgs are the arguments for the Echo function
type FlattenArgs struct {
	IncludeRegexp *string
	ExcludeRegexp *string
}

func (m *OpUtils) flatten(input *base.OperatorIO) (map[string]interface{}, *base.OperatorIO) {
	nestedArgsMap := map[string]interface{}{}
	err := input.ParseJSON(&nestedArgsMap)
	if err != nil {
		return nil, base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a map")
	}

	argsmap, err := flatten.Flatten(nestedArgsMap, "", flatten.DotStyle)
	if err != nil {
		return nil, base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a flat map: %v", err)
	}
	return argsmap, nil
}

// Flatten flattens the input from a nested map to a flat map
func (m *OpUtils) Flatten(ctx *base.Context, input *base.OperatorIO, args FlattenArgs) *base.OperatorIO {
	argsmap, err := m.flatten(input)
	if err != nil {
		return err
	}

	if args.IncludeRegexp != nil {
		re, err := regexp.Compile(*args.IncludeRegexp)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "include regexp cannot be compiled: %v", err)
		}
		// store keys that match the regexp
		keysToRemove := []string{}
		for k := range argsmap {
			if !re.MatchString(k) {
				keysToRemove = append(keysToRemove, k)
			}
		}
		// remove keys that do not match the regexp
		for _, k := range keysToRemove {
			delete(argsmap, k)
		}
	}
	if args.ExcludeRegexp != nil {
		re, err := regexp.Compile(*args.ExcludeRegexp)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "exclude regexp cannot be compiled: %v", err)
		}
		// store keys that match the regexp
		keysToRemove := []string{}
		for k := range argsmap {
			if re.MatchString(k) {
				keysToRemove = append(keysToRemove, k)
			}
		}
		// remove keys that do not match the regexp
		for _, k := range keysToRemove {
			delete(argsmap, k)
		}
	}

	return base.MakeObjectOutput(argsmap)
}

// ExtractArgs are the arguments for the Extract function
type ExtractArgs struct {
	Key         string
	Type        *string
	ContentType *string
}

// TypeSuggestions returns a list of possible types for the given key
func (m *OpUtils) TypeSuggestions() []string {
	return []string{"string", "int", "float", "bool", "quotedstring", "stringobject", "bytesfrombase64", "base64encoded"}
}

// ContenttypeSuggestions returns a list of possible content types for the given key
func (m *OpUtils) ContenttypeSuggestions() []string {
	return []string{"application/json", "application/xml", "application/yaml"}
}

// Extract extracts the value of a given key from the input, if necessary it tries to flatten the input first
func (m *OpUtils) Extract(ctx *base.Context, input *base.OperatorIO, args ExtractArgs) *base.OperatorIO {
	nestedArgsMap := map[string]interface{}{}
	err := input.ParseJSON(&nestedArgsMap)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a map")
	}
	vInterface, ok := nestedArgsMap[args.Key]
	if !ok {
		argsmap, err := flatten.Flatten(nestedArgsMap, "", flatten.DotStyle)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a flat map: %v", err)
		}
		vInterface, ok = argsmap[args.Key]
		if !ok {
			return base.MakeOutputError(http.StatusBadRequest, "expected value %s in request", args.Key)
		}
	}

	if args.Type == nil {
		return base.MakeObjectOutput(vInterface)
	}
	var outputObject any
	switch utils.StringToLower(*args.Type) {
	case "int":
		outputObject, err = utils.ConvertToInt64(vInterface)
	case "float":
		outputObject, err = utils.ConvertToFloat(vInterface)
	case "bool":
		outputObject, err = utils.ConvertToBool(vInterface)
	case "quotedstring", "stringobject":
		outputObject, err = utils.ConvertToString(vInterface)
	case "string":
		outputObject, err = utils.ConvertToString(vInterface)
		if err == nil {
			return base.MakePlainOutput(outputObject.(string))
		}
	case "bytesfrombase64", "base64encoded":
		b64str, ok := vInterface.(string)
		if !ok {
			return base.MakeOutputError(http.StatusBadRequest, "expected value %s in request to be a string", args.Key)
		}
		byt, ierr := base64.StdEncoding.DecodeString(b64str)
		if ierr == nil {
			if args.ContentType != nil {
				return base.MakeByteOutputWithContentType(byt, *args.ContentType)
			}
			return base.MakeByteOutput(byt)
		}
		err = fmt.Errorf("expected value %s in request to be a base64 encoded string: %v", args.Key, err)
	default:
		return base.MakeOutputError(http.StatusBadRequest, "No such type %s", *args.Type)
	}
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err)
	}
	return base.MakeObjectOutput(outputObject)
}

// EchoArgs are the arguments for the Echo function
type EchoArgs struct {
	Output   *string
	Silent   *bool
	AsString *bool
}

// Echo returns the given output or an empty string if no output is given
func (m *OpUtils) Echo(ctx *base.Context, input *base.OperatorIO, args EchoArgs) *base.OperatorIO {
	if args.Output != nil {
		return base.MakePlainOutput(*args.Output)
	}
	if args.Silent != nil && *args.Silent {
		return base.MakeEmptyOutput()
	}

	if args.AsString != nil && *args.AsString {
		return base.MakePlainOutput(input.GetString())
	}

	return input
}

// HasInput returns an error if the input is empty
func (m *OpUtils) HasInput(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	if input.IsEmpty() {
		return base.MakeOutputError(http.StatusExpectationFailed, "Expected input")
	}
	return input
}

// FormToJSON converts the input from form data to JSON
func (m *OpUtils) FormToJSON(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	o, err := input.ParseFormData()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input not valid form data: %v", err)
	}
	o2 := utils.URLArgsToMap(o)
	return base.MakeObjectOutput(o2)
}

// Fail returns an error
func (m *OpUtils) Fail(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	return base.MakeOutputError(http.StatusExpectationFailed, "Fail")
}

// EchoArgumentsArgs are the arguments for the EchoArguments function
type EchoArgumentsArgs struct {
	InputKey *string
}

// EchoArguments returns the arguments as a map
func (m *OpUtils) EchoArguments(ctx *base.Context, input *base.OperatorIO, args EchoArgumentsArgs, otherArgs base.FunctionArguments) *base.OperatorIO {
	output := map[string]interface{}{}
	if !input.IsEmpty() {
		if args.InputKey != nil {
			output[*args.InputKey] = input.Output
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
	for k, v := range otherArgs.GetOriginalCaseMapOnlyFirst() {
		output[k] = v
	}
	return base.MakeObjectOutput(output)
}

// MergeInputAndArguments merges the input and the arguments into a single map
func (m *OpUtils) MergeInputAndArguments(ctx *base.Context, input *base.OperatorIO, args base.FunctionArguments) *base.OperatorIO {
	output := map[string]interface{}{}
	input.ParseJSON(&output)
	for k, v := range args.GetOriginalCaseMap() {
		output[k] = v
	}
	return base.MakeObjectOutput(output)
}

// RemapKeys renames arguments in the input based on the given mapping
func (m *OpUtils) RemapKeys(ctx *base.Context, input *base.OperatorIO, args EchoArgumentsArgs, mapping base.FunctionArguments) *base.OperatorIO {
	oldArgs, err := input.GetArgsMap()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be converted to map[string]string: %v", err)
	}
	output := map[string]string{}
	for k, v := range oldArgs {
		if mapping.Has(k) {
			newKeys := mapping.Get(k) // TODO(HR): somehow this should be parse without comma I think
			for _, newKey := range strings.Split(newKeys, ",") {
				output[newKey] = v
			}
		} else {
			_, oldKeyExists := output[k]
			if !oldKeyExists {
				output[k] = v
			}
		}
	}
	return base.MakeObjectOutput(output)
}

// StringSplitArgs are the arguments for the Split function
type StringSplitArgs struct {
	Sep string
	Pos int
}

// StringSplit splits the input by the given separator and returns the part at the given position
func (m *OpUtils) StringSplit(ctx *base.Context, input *base.OperatorIO, args StringSplitArgs) *base.OperatorIO {
	if args.Sep == "" {
		return base.MakeOutputError(http.StatusBadRequest, "Need a separator (sep) to split")
	}
	strArray := strings.Split(input.GetString(), args.Sep)
	if args.Pos >= len(strArray) {
		return base.MakeOutputError(http.StatusBadRequest, "Pos %v not available in array %v", args.Pos, strArray)
	}
	return base.MakePlainOutput(strArray[args.Pos])
}

// StringReplaceArgs are the arguments for the StringReplace function
type StringReplaceArgs struct {
	InputString *string
	Search      string
	Replace     string
}

// StringReplace replaces the given search string with the given replace string
func (m *OpUtils) StringReplace(ctx *base.Context, input *base.OperatorIO, args StringReplaceArgs) *base.OperatorIO {
	inputStr := input.GetString()
	if args.InputString != nil {
		inputStr = *args.InputString
	}
	return base.MakePlainOutput(strings.Replace(inputStr, args.Search, args.Replace, -1))
}

// StringReplaceMultiArgs
type StringReplaceMultiArgs struct {
	InputString *string
}

// StringReplaceMulti replaces given args framed with "%" with their values
func (m *OpUtils) StringReplaceMulti(ctx *base.Context, input *base.OperatorIO, args StringReplaceMultiArgs, otherArgs base.FunctionArguments) *base.OperatorIO {
	inputStr := input.GetString()
	if args.InputString != nil {
		inputStr = *args.InputString
	}
	for k, v := range otherArgs.GetOriginalCaseMapOnlyFirst() {
		searchStr := "%" + k + "%"
		inputStr = strings.Replace(inputStr, searchStr, v, -1)
	}
	return base.MakePlainOutput(inputStr)
}

// StringAppendArgs
type StringAppendArgs struct {
	InputString    *string
	StringToAppend string
}

// StringReplaceMulti replaces given args framed with "%" with their values
func (m *OpUtils) StringAppend(ctx *base.Context, input *base.OperatorIO, args StringAppendArgs) *base.OperatorIO {
	inputStr := input.GetString()
	if args.InputString != nil {
		inputStr = *args.InputString
	}
	return base.MakePlainOutput(inputStr + args.StringToAppend)
}

// ConvertFormDataToInputArgs are the arguments for the ConvertFormDataToInput function
type ConvertFormDataToInputArgs struct {
	InputFieldName *string
}

// ConvertFormDataToInput takes the "input" field from the form data and passes it on directly
func (m *OpUtils) ConvertFormDataToInput(ctx *base.Context, input *base.OperatorIO, args ConvertFormDataToInputArgs) *base.OperatorIO {
	formData, err := input.ParseFormData()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input not valid form data: %v", err)
	}
	inputFieldName := "input"
	if args.InputFieldName != nil {
		inputFieldName = *args.InputFieldName
	}
	if formData.Has(inputFieldName) {
		return base.MakePlainOutput(formData.Get(inputFieldName))
	}
	return base.MakeOutputError(http.StatusBadRequest, "input not valid form data: no input field")
}

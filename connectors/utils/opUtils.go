package freepsutils

import (
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

func (m *OpUtils) flatten(ctx *base.Context, input *base.OperatorIO) (map[string]interface{}, *base.OperatorIO) {
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
	argsmap, err := m.flatten(ctx, input)
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
	Key string
}

// Extract extracts the value of a given key from the input, if necessary it tries to flatten the input first
func (m *OpUtils) Extract(ctx *base.Context, input *base.OperatorIO, args ExtractArgs) *base.OperatorIO {
	nestedArgsMap := map[string]interface{}{}
	err := input.ParseJSON(&nestedArgsMap)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a map")
	}
	vInterface, ok := nestedArgsMap[args.Key]
	if ok {
		return base.MakeObjectOutput(vInterface)
	}

	argsmap, err := flatten.Flatten(nestedArgsMap, "", flatten.DotStyle)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be parsed into a flat map: %v", err)
	}
	vInterface, ok = argsmap[args.Key]
	if !ok {
		return base.MakeOutputError(http.StatusBadRequest, "expected value %s in request", args.Key)
	}
	return base.MakeObjectOutput(vInterface)
}

// EchoArgs are the arguments for the Echo function
type EchoArgs struct {
	Output *string
	Silent *bool
}

// Echo returns the given output or an empty string if no output is given
func (m *OpUtils) Echo(ctx *base.Context, input *base.OperatorIO, args EchoArgs) *base.OperatorIO {
	if args.Output != nil {
		return base.MakePlainOutput(*args.Output)
	}
	if args.Silent != nil && *args.Silent {
		return base.MakeEmptyOutput()
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

// EchoArgumentsArgs are the arguments for the EchoArguments function
type EchoArgumentsArgs struct {
	InputKey *string
}

// EchoArguments returns the arguments as a map
func (m *OpUtils) EchoArguments(ctx *base.Context, input *base.OperatorIO, args EchoArgumentsArgs, otherArgs map[string]string) *base.OperatorIO {
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
	for k, v := range otherArgs {
		output[k] = v
	}
	return base.MakeObjectOutput(output)
}

// Remap values from the input to the output
func (m *OpUtils) RemapKeys(ctx *base.Context, input *base.OperatorIO, args EchoArgumentsArgs, mapping map[string]string) *base.OperatorIO {
	oldArgs, err := input.GetArgsMap()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "input cannot be converted to map[string]string: %v", err)
	}
	output := map[string]interface{}{}
	for k, v := range oldArgs {
		if newKey, ok := mapping[k]; ok {
			output[newKey] = v
		} else {
			output[k] = v
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
func (m *OpUtils) StringReplaceMulti(ctx *base.Context, input *base.OperatorIO, args StringReplaceMultiArgs, otherArgs map[string]string) *base.OperatorIO {
	inputStr := input.GetString()
	if args.InputString != nil {
		inputStr = *args.InputString
	}
	for k, v := range otherArgs {
		searchStr := "%" + k + "%"
		inputStr = strings.Replace(inputStr, searchStr, v, -1)
	}
	return base.MakePlainOutput(inputStr)
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

package freepsgraph

import (
	"net/http"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/jeremywohl/flatten"
)

// OpUtils is a collection of utility operations
type OpUtils struct {
}

var _ base.FreepsOperator = &OpUtils{}

// Flatten flattens the input from a nested map to a flat map
func (m *OpUtils) Flatten(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
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
	Search  string
	Replace string
}

// StringReplace replaces the given search string with the given replace string
func (m *OpUtils) StringReplace(ctx *base.Context, input *base.OperatorIO, args StringReplaceArgs) *base.OperatorIO {
	return base.MakePlainOutput(strings.Replace(input.GetString(), args.Search, args.Replace, -1))
}

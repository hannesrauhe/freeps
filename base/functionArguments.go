package base

import (
	"github.com/hannesrauhe/freeps/utils"
)

// FunctionArguments is a struct that can be used to pass arguments to a function
type FunctionArguments = utils.CIMap[string]

func NewFunctionArguments(args map[string]string) FunctionArguments {
	return utils.NewStringCIMap(args)
}

func NewFunctionArgumentsFromURLQuery(args map[string][]string) FunctionArguments {
	return utils.NewStringCIMapFromValues(args)
}

func MakeEmptyFunctionArguments() FunctionArguments {
	return utils.NewStringCIMap(map[string]string{})
}

package base

import (
	"net/url"

	"github.com/hannesrauhe/freeps/utils"
)

// FunctionArguments is a struct that can be used to pass arguments to a function
type FunctionArguments = utils.CIMap[string]

func NewFunctionArguments(args map[string]string) FunctionArguments {
	return utils.NewStringCIMap(args)
}

// NewFunctionArgumentsFromObject flattens an object into function arguments
func NewFunctionArgumentsFromObject(obj interface{}) (FunctionArguments, error) {
	args, err := utils.ObjectToArgsMap(obj)
	return utils.NewStringCIMap(args), err
}

func NewFunctionArgumentsFromURLValues(args map[string][]string) FunctionArguments {
	return utils.NewStringCIMapFromValues(args)
}

func NewFunctionArgumentsFromURLQuery(query string) (FunctionArguments, error) {
	args, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}
	return utils.NewStringCIMapFromValues(args), nil
}

func NewSingleFunctionArgument(key string, value ...string) FunctionArguments {
	ret := utils.NewStringCIMap(map[string]string{})
	ret.Append(key, value...)
	return ret
}

func MakeEmptyFunctionArguments() FunctionArguments {
	return utils.NewStringCIMap(map[string]string{})
}

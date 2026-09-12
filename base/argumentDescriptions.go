package base

import (
	"reflect"

	"github.com/hannesrauhe/freeps/utils"
)

// ArgumentDescription describes a single argument of a FreepsFunction.
// For operators created with MakeFreepsOperators, the Description is taken from the "doc"
// struct tag of the corresponding field of the parameter struct. The tag is optional, the
// description is empty if it is not set.
type ArgumentDescription struct {
	Name        string // the name of the argument, as returned by GetPossibleArgs
	Type        string // the type of the argument: string, int, "int64 or duration", float, bool or "<type> list"
	Required    bool   // true for plain fields, false for pointers and slices
	Description string // the description from the "doc" struct tag, empty if the tag is not set
}

var _ FreepsBaseOperator = &FreepsOperatorWrapper{}

// DescribeArguments returns the descriptions of all arguments of the given function of the
// given operator. It is a nil-safe shortcut for op.GetArgumentDescriptions(fn).
func DescribeArguments(op FreepsBaseOperator, fn string) []ArgumentDescription {
	if op == nil {
		return []ArgumentDescription{}
	}
	return op.GetArgumentDescriptions(fn)
}

// NameOnlyArgumentDescriptions returns an ArgumentDescription with only the name set for every
// argument of the given function. It is the default implementation for operators that implement
// FreepsBaseOperator directly and therefore have no parameter structs to describe.
func NameOnlyArgumentDescriptions(op FreepsBaseOperator, fn string) []ArgumentDescription {
	list := []ArgumentDescription{}
	for _, name := range op.GetPossibleArgs(fn) {
		list = append(list, ArgumentDescription{Name: name})
	}
	return list
}

// GetArgumentDescriptions returns a description for every argument of the given function.
// The fields of the parameter struct describe the arguments: the "doc" struct tag provides
// the description and is optional. Dynamic functions have no parameter struct and are
// described by name only.
func (o *FreepsOperatorWrapper) GetArgumentDescriptions(fn string) []ArgumentDescription {
	list := []ArgumentDescription{}

	m := o.getFunctionMetaData(fn)
	if m == nil {
		// dynamic functions have no parameter struct, describe them by name only
		if dynamicOp, ok := o.opInstance.(FreepsOperatorWithDynamicFunctions); ok {
			for _, argName := range dynamicOp.GetDynamicPossibleArgs(utils.StringToLower(fn)) {
				list = append(list, ArgumentDescription{Name: argName})
			}
		}
		return list
	}

	switch m.FuncType {
	case FreepsFunctionTypeSimple, FreepsFunctionTypeContextOnly, FreepsFunctionTypeContextAndInput, FreepsFunctionTypeWithDynamicFunctionArguments:
		return list
	}

	// get the type of the third parameter of the FreepsFunction (the parameter struct) and iterate over all fields
	paramStructType := m.FuncValue.Type().In(2)
	paramStruct := reflect.New(paramStructType).Elem()
	for i := 0; i < paramStruct.NumField(); i++ {
		field := paramStruct.Field(i)
		required := isSupportedField(field, false)
		if !required && !isSupportedField(field, true) {
			continue
		}
		fieldType := paramStructType.Field(i)
		list = append(list, ArgumentDescription{
			Name:        fieldType.Name,
			Type:        argumentTypeName(fieldType.Type),
			Required:    required,
			Description: fieldType.Tag.Get("doc"),
		})
	}
	return list
}

// argumentTypeName returns a human readable name for the type of an argument.
// int64 fields accept a plain integer as well as a duration string and are often used for
// durations, but this cannot be detected from the type, so the name stays honest.
func argumentTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Ptr:
		return argumentTypeName(t.Elem())
	case reflect.Slice:
		return argumentTypeName(t.Elem()) + " list"
	case reflect.String:
		return "string"
	case reflect.Int:
		return "int"
	case reflect.Int64:
		return "int64 or duration"
	case reflect.Float64:
		return "float"
	case reflect.Bool:
		return "bool"
	}
	return t.String()
}

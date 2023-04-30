package base

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
)

// FreepsOperator is the interface structs need to implement so FreepsOperatorWrapper can create a FreepsOperator from them
type FreepsOperator interface {
	// every exported function that follows the rules given in FreepsFunctionType is a FreepsFunction
}

// FreepsOperatorWithConfig adds the GetConfig() method to FreepsOperator
type FreepsOperatorWithConfig interface {
	FreepsOperator
	// GetConfig returns the config struct of the operator that is filled wiht the values from the config file
	GetConfig() interface{}
	// Init is called after the config is read and the operator is created
	Init(ctx *Context) error
}

// FreepsOperatorWithShutdown adds the Shutdown() method to FreepsOperatorWithConfig
type FreepsOperatorWithShutdown interface {
	FreepsOperatorWithConfig
	Shutdown(ctx *Context)
}

// FreepsFunctionParameters is the interface for a paramter struct that can return ArgumentSuggestions
type FreepsFunctionParameters interface {
	// GetArgSuggestions returns a map of possible arguments for the given function and argument name
	GetArgSuggestions(fn string, argName string, otherArgs map[string]string) map[string]string
}

/* Implentation follows below this line */

// FreepsFunctionType is an enum that indicates how many function parameters a compatible FreepsFunction has
type FreepsFunctionType int

const (
	// FreepsFunctionTypeUnknown indicates that the function is not a FreepsFunction
	FreepsFunctionTypeUnknown FreepsFunctionType = iota
	// FreepsFunctionTypeSimple indicates that the function has no parameters
	FreepsFunctionTypeSimple
	// FreepsFunctionTypeContextOnly indicates that the function has a Context ptr as first parameter
	FreepsFunctionTypeContextOnly
	// FreepsFunctionTypeContextAndInput indicates that the function has a Context ptr as first parameter and a OperatorIO ptr as second parameter
	FreepsFunctionTypeContextAndInput
	// FreepsFunctionTypeWithArguments indicates that the function has a Context ptr as first parameter, a OperatorIO ptr as second parameter and a struct as third parameter
	FreepsFunctionTypeWithArguments
	// FreepsFunctionTypeFullSignature indicates that the function has a Context ptr as first parameter, a OperatorIO ptr as second parameter, a struct as third parameter and a map[string]string as fourth parameter
	FreepsFunctionTypeFullSignature
)

// FreepsFunctionMetaData contains the reflect Value to the Function itself, the case sensitive name and the FreepsFunctionType
type FreepsFunctionMetaData struct {
	FuncValue reflect.Value
	Name      string
	FuncType  FreepsFunctionType
}

// FreepsOperatorWrapper creates everything necessary to be a FreepsOperator from any struct that implements FreepsOperator
type FreepsOperatorWrapper struct {
	opInstance          FreepsOperator
	functionMetaDataMap map[string]FreepsFunctionMetaData
}

var _ FreepsBaseOperator = &FreepsOperatorWrapper{}

// MakeFreepsOperator creates a FreepsBaseOperator from any struct that implements FreepsOperator
func MakeFreepsOperator(anyClass FreepsOperator, cr *utils.ConfigReader, ctx *Context) FreepsBaseOperator {
	if anyClass == nil {
		return nil
	}

	op := &FreepsOperatorWrapper{opInstance: anyClass}
	enabled, err := op.initIfEnabled(cr, ctx)
	if err != nil {
		logrus.Errorf("Initializing operator \"%v\" failed: %v", op.GetName(), err)
		return nil
	}
	if !enabled {
		return nil
	}
	return op
}

func (o *FreepsOperatorWrapper) initIfEnabled(cr *utils.ConfigReader, ctx *Context) (bool, error) {
	o.functionMetaDataMap = o.createFunctionMap(ctx)

	var noFuncsError error // in case the operator is disabled in the config we do not want to return an error
	if len(o.functionMetaDataMap) == 0 {
		noFuncsError = fmt.Errorf("No functions found for operator \"%v\"", o.GetName())
	}

	if cr == nil {
		// no config reader might mean testing, just return
		return true, noFuncsError
	}

	confOp, ok := o.opInstance.(FreepsOperatorWithConfig)
	if !ok {
		return true, noFuncsError
	}

	conf := confOp.GetConfig()
	if conf == nil {
		return true, noFuncsError
	}
	err := cr.ReadSectionWithDefaults(o.GetName(), &conf)
	if err != nil {
		return true, fmt.Errorf("Reading config for operator \"%v\" failed: %v", o.GetName(), err)
	}

	err = cr.WriteBackConfigIfChanged()
	if err != nil {
		return true, fmt.Errorf("Writing back config for operator \"%v\" failed: %v", o.GetName(), err)
	}

	// check if the config object has a field called "enabled" and if it is set to false
	// if it is set to false, we do not want to initialize the operator and return nil
	enabledField := reflect.ValueOf(conf).Elem().FieldByName("Enabled")
	if enabledField.IsValid() && enabledField.Kind() == reflect.Bool && !enabledField.Bool() {
		return false, nil
	}
	confOp.Init(ctx)
	return true, noFuncsError
}

// getFunction returns the function with the given name (case insensitive)
func (o *FreepsOperatorWrapper) getFunctionMetaData(name string) *FreepsFunctionMetaData {
	name = utils.StringToLower(name)
	if f, ok := o.functionMetaDataMap[name]; ok {
		return &f
	}
	return nil
}

// getFunction returns the function with the given name (case insensitive)
func (o *FreepsOperatorWrapper) getFunction(name string) *reflect.Value {
	name = utils.StringToLower(name)
	if f, ok := o.functionMetaDataMap[name]; ok {
		return &f.FuncValue
	}
	return nil
}

// getFreepsFunctionType returns the FreepsFunctionType that describes the function for a given reflect.Type
// The given reflect.Type:
// 1. must be an exported function
// 2. returns exactly one value wich must be OperatorIO*
// 3. has between 1 and 5 parameters
// 4. the first parameter must the ptr to the Operator the function belongs to
// 5. optionally takes a Context ptr as first parameter
// 6. optionally takes a OperatorIO ptr as second parameters
// 7. optionally takes a struct as third parameter (the parameters struct)
// 8. optionally takes a map[string]string as fourth parameter
func getFreepsFunctionType(f reflect.Type) (FreepsFunctionType, error) {
	// describe function signature in a string to give developer a hint what is wrong
	// we do not want to use f.String() because it is not very readable
	var funcSignature string
	for i := 0; i < f.NumIn(); i++ {
		if i > 0 {
			funcSignature += ", "
		}
		funcSignature += f.In(i).String()
	}
	funcSignature += " -> "
	for i := 0; i < f.NumOut(); i++ {
		if i > 0 {
			funcSignature += ", "
		}
		funcSignature += f.Out(i).String()
	}

	if f.Kind() != reflect.Func || f.NumOut() != 1 || f.Out(0) != reflect.TypeOf(&OperatorIO{}) {
		return FreepsFunctionTypeUnknown, fmt.Errorf("Function \"%v\" does not return exactly one value of type \"*OperatorIO\"", funcSignature)
	}
	switch f.NumIn() {
	case 0:
		panic("Function has no parameters, it does not belong to an operator")
	case 1:
		return FreepsFunctionTypeSimple, nil
	case 2:
		if f.In(1) == reflect.TypeOf(&Context{}) {
			return FreepsFunctionTypeContextOnly, nil
		}
	case 3:
		if f.In(1) == reflect.TypeOf(&Context{}) && f.In(2) == reflect.TypeOf(&OperatorIO{}) {
			return FreepsFunctionTypeContextAndInput, nil
		}
	case 4:
		if f.In(1) == reflect.TypeOf(&Context{}) && f.In(2) == reflect.TypeOf(&OperatorIO{}) && f.In(3).Kind() == reflect.Struct {
			return FreepsFunctionTypeWithArguments, nil
		}
	case 5:
		if f.In(1) == reflect.TypeOf(&Context{}) && f.In(2) == reflect.TypeOf(&OperatorIO{}) && f.In(3).Kind() == reflect.Struct && f.In(4) == reflect.TypeOf(map[string]string{}) {
			return FreepsFunctionTypeFullSignature, nil
		}
	}
	return FreepsFunctionTypeUnknown, fmt.Errorf("Function \"%v\" has an invalid signature", funcSignature)
}

// createFunctionMap creates a map of all exported functions of the struct that return a struct that implements FreepsFunction
func (o *FreepsOperatorWrapper) createFunctionMap(ctx *Context) map[string]FreepsFunctionMetaData {
	funcMap := make(map[string]FreepsFunctionMetaData)
	t := reflect.TypeOf(o.opInstance)
	v := reflect.ValueOf(o.opInstance)
	for i := 0; i < t.NumMethod(); i++ {
		ffType, err := getFreepsFunctionType(t.Method(i).Type)
		if err != nil {
			ctx.logger.Debug("Function \"%v\" of operator \"%v\" is not a valid FreepsFunction: %v\n", t.Method(i).Name, o.GetName(), err)
			continue
		}
		funcMap[utils.StringToLower(t.Method(i).Name)] = FreepsFunctionMetaData{Name: t.Method(i).Name, FuncValue: v.Method(i), FuncType: ffType}
	}
	return funcMap
}

func isSupportedFieldType(field reflect.Type) bool {
	kind := field.Kind()
	return kind == reflect.Int || kind == reflect.Int64 || kind == reflect.String || kind == reflect.Float64 || kind == reflect.Bool
}

// isSupportedField returns true if the field is a primitive type or a pointer to a primitive type
func isSupportedField(field reflect.Value, mustBePtr bool) bool {
	if !field.CanSet() {
		return false
	}
	if field.Kind() == reflect.Ptr && mustBePtr {
		return isSupportedFieldType(field.Type().Elem())
	}
	if mustBePtr {
		return false
	}
	return isSupportedFieldType(field.Type())
}

// setSupportedField sets the value of the field and converts from string if necessary
func setSupportedField(field reflect.Value, value string) error {
	if field.Type().Kind() == reflect.Ptr {
		newField := reflect.New(field.Type().Elem())
		field.Set(newField)
		field = field.Elem()
	}

	// convert the value to the type of the field, return an error if the conversion fails
	switch field.Kind() {
	case reflect.Int:
		v, err := utils.StringToInt(value)
		if err != nil {
			return fmt.Errorf("\"%v\" is not convertible to int: %v", value, err)
		}
		field.SetInt(int64(v))
	case reflect.Int64: // this might actually be a time.Duration
		v, err := utils.StringToInt(value)
		if err == nil {
			field.SetInt(int64(v))
		} else {
			vTime, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("\"%v\" is not convertible to int and is not a time duration: %v", value, err)
			}
			field.SetInt(int64(vTime))
		}
	case reflect.String:
		field.SetString(value)
	case reflect.Float64:
		v, err := utils.StringToFloat64(value)
		if err != nil {
			return fmt.Errorf("\"%v\" is not convertible to float64: %v", value, err)
		}
		field.SetFloat(v)
	case reflect.Bool:
		v := utils.ParseBool(value)
		field.SetBool(v)
	default:
		// should never get here
		return fmt.Errorf("Unsupported field type: %v", field.Kind())
	}
	return nil
}

// SetRequiredFreepsFunctionParameters sets the parameters of the FreepsFunction based on the args map
func (o *FreepsOperatorWrapper) SetRequiredFreepsFunctionParameters(freepsFuncParams reflect.Value, args map[string]string, failOnErr bool) *OperatorIO {
	//make sure all non-pointer fields of the FreepsFunction are set to the values of the args map
	for i := 0; i < freepsFuncParams.Elem().NumField(); i++ {
		field := freepsFuncParams.Elem().Field(i)

		fieldName := utils.StringToLower(freepsFuncParams.Elem().Type().Field(i).Name)
		if !isSupportedField(field, false) {
			continue
		}

		//return an error if the field is not set in the args map
		v, ok := args[fieldName]
		if !ok {
			if failOnErr {
				return MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" not found", fieldName))
			} else {
				continue
			}
		}

		// set the value of the field
		err := setSupportedField(field, v)
		if err != nil {
			if failOnErr {
				return MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is invalid: %v", fieldName, err))
			}
			continue
		}

		delete(args, fieldName)
	}
	return nil
}

// SetOptionalFreepsFunctionParameters sets the parameters of the FreepsFunction based on the args map
func (o *FreepsOperatorWrapper) SetOptionalFreepsFunctionParameters(freepsfunc reflect.Value, args map[string]string, failOnErr bool) *OperatorIO {
	// set all pointer fields of the FreepsFunction to the values of the args map
	for i := 0; i < freepsfunc.Elem().NumField(); i++ {
		field := freepsfunc.Elem().Field(i)

		fieldName := utils.StringToLower(freepsfunc.Elem().Type().Field(i).Name)
		if !isSupportedField(field, true) {
			continue
		}

		v, ok := args[fieldName]
		if !ok {
			continue
		}
		err := setSupportedField(field, v)
		if err != nil {
			if failOnErr {
				return MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is invalid: %v", fieldName, err))
			}
			continue
		}
		delete(args, fieldName)
	}
	return nil
}

// GetName returns the name of the struct opClass
func (o *FreepsOperatorWrapper) GetName() string {
	t := reflect.TypeOf(o.opInstance)
	return t.Elem().Name()
}

// Execute gets the FreepsFunction by name, assignes all parameters based on the args map and calls the function
func (o *FreepsOperatorWrapper) Execute(ctx *Context, function string, args map[string]string, mainInput *OperatorIO) *OperatorIO {
	m := o.getFunctionMetaData(function)
	if m == nil {
		return MakeOutputError(http.StatusNotFound, fmt.Sprintf("Function \"%v\" not found", function))
	}

	// execute function immediately if the FreepsFunctionType indicates it needs no arguments
	switch m.FuncType {
	case FreepsFunctionTypeSimple:
		outValue := m.FuncValue.Call([]reflect.Value{})
		return outValue[0].Interface().(*OperatorIO)
	case FreepsFunctionTypeContextOnly:
		outValue := m.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx)})
		return outValue[0].Interface().(*OperatorIO)
	case FreepsFunctionTypeContextAndInput:
		outValue := m.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(mainInput)})
		return outValue[0].Interface().(*OperatorIO)
	}

	//TODO(HR): ensure that args are lowercase
	lowercaseArgs := map[string]string{}
	for k, v := range args {
		lowercaseArgs[utils.StringToLower(k)] = v
	}

	// get the type of the third parameter of the FreepsFunction (the parameter struct) and create a new instance of it
	paramStructType := m.FuncValue.Type().In(2)
	paramStruct := reflect.New(paramStructType)

	failOnError := true

	//set all required parameters of the FreepsFunction
	err := o.SetRequiredFreepsFunctionParameters(paramStruct, lowercaseArgs, failOnError)
	if err != nil && failOnError {
		return err
	}
	err = o.SetOptionalFreepsFunctionParameters(paramStruct, lowercaseArgs, failOnError)
	if err != nil && failOnError {
		return err
	}

	if m.FuncType == FreepsFunctionTypeWithArguments {
		outValue := m.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(mainInput), paramStruct.Elem()})
		return outValue[0].Interface().(*OperatorIO)
	}
	if m.FuncType == FreepsFunctionTypeFullSignature {
		outValue := m.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(mainInput), paramStruct.Elem(), reflect.ValueOf(lowercaseArgs)})
		return outValue[0].Interface().(*OperatorIO)
	}

	return MakeOutputError(http.StatusInternalServerError, "Function could not be executed")
}

// GetFunctions returns all methods of the opClass
func (o *FreepsOperatorWrapper) GetFunctions() []string {
	list := []string{}

	for _, v := range o.functionMetaDataMap {
		list = append(list, v.Name)
	}
	return list
}

// GetPossibleArgs returns all function parameters
func (o *FreepsOperatorWrapper) GetPossibleArgs(fn string) []string {
	list := []string{}

	m := o.getFunctionMetaData(fn)
	if m == nil {
		return list
	}
	if m.FuncType == FreepsFunctionTypeSimple || m.FuncType == FreepsFunctionTypeContextOnly || m.FuncType == FreepsFunctionTypeContextAndInput {
		return list
	}

	// get the type of the third parameter of the FreepsFunction (the parameter struct) and iterate over all fields
	paramStructType := m.FuncValue.Type().In(2)
	paramStruct := reflect.New(paramStructType).Elem()
	for i := 0; i < paramStruct.NumField(); i++ {
		arg := paramStruct.Field(i)
		if isSupportedField(arg, true) || isSupportedField(arg, false) {
			list = append(list, paramStructType.Field(i).Name)
		}
	}

	return list
}

// GetArgSuggestions creates a Freepsfunction by name and returns the suggestions for the argument argName
func (o *FreepsOperatorWrapper) GetArgSuggestions(function string, argName string, otherArgs map[string]string) map[string]string {
	res := map[string]string{}
	m := o.getFunctionMetaData(function)
	if m == nil {
		return res
	}

	switch m.FuncType {
	case FreepsFunctionTypeSimple, FreepsFunctionTypeContextOnly, FreepsFunctionTypeContextAndInput:
		return res
	}

	//TODO(HR): ensure that args are lowercase
	lowercaseArgs := map[string]string{}
	for k, v := range otherArgs {
		lowercaseArgs[utils.StringToLower(k)] = v
	}

	// get the type of the third parameter of the FreepsFunction (the parameter struct) and create a new instance of it
	paramStructType := m.FuncValue.Type().In(2)
	paramStruct := reflect.New(paramStructType)
	// check if paramStruct implements the FreepsFunctionParameters interface
	if !paramStruct.Type().Implements(reflect.TypeOf((*FreepsFunctionParameters)(nil)).Elem()) {
		return res
	}

	failOnError := false

	//set all required parameters of the FreepsFunction
	o.SetRequiredFreepsFunctionParameters(paramStruct, lowercaseArgs, failOnError)
	o.SetOptionalFreepsFunctionParameters(paramStruct, lowercaseArgs, failOnError)

	ps := paramStruct.Interface().(FreepsFunctionParameters)
	return ps.GetArgSuggestions(utils.StringToLower(function), utils.StringToLower(argName), lowercaseArgs)
}

// Shutdown calls the Shutdown method of the FreepsOperator if it exists
func (o *FreepsOperatorWrapper) Shutdown(ctx *Context) {
	opShutdown, ok := o.opInstance.(FreepsOperatorWithShutdown)
	if ok {
		opShutdown.Shutdown(ctx)
	}
}

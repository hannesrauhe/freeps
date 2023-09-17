package base

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/hannesrauhe/freeps/utils"
)

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

// FreepsOperatorWrapper creates everything necessary to be a FreepsBaseOperator from any struct that implements FreepsOperator
type FreepsOperatorWrapper struct {
	opInstance          FreepsOperator
	opName              string // the name of the operator, if empty, the name is extracted from the type of opInstance
	config              interface{}
	functionMetaDataMap map[string]FreepsFunctionMetaData
}

var _ FreepsBaseOperator = &FreepsOperatorWrapper{}

// MakeFreepsOperators creates FreepsBaseOperator variations from any struct that implements FreepsOperator
func MakeFreepsOperators(anyClass FreepsOperator, cr *utils.ConfigReader, ctx *Context) []FreepsBaseOperator {
	if anyClass == nil {
		return nil
	}

	o := FreepsOperatorWrapper{opInstance: anyClass}
	o.createFunctionMap(ctx)

	if len(o.functionMetaDataMap) == 0 {
		ctx.logger.Panicf("No compatible freeps functions found for operator \"%v\"", o.GetName())
	}

	if cr == nil {
		// no config reader might mean testing, just return
		return []FreepsBaseOperator{&o}
	}

	_, ok := o.opInstance.(FreepsOperatorWithConfig)
	if !ok {
		// operator does not implement FreepsOperatorWithConfig, so there cannot be any variations
		return []FreepsBaseOperator{&o}
	}

	ops := initOperatorVariations(o, cr, ctx)
	if len(ops) == 0 {
		return nil
	}

	return ops
}

func initOperatorVariations(opVariationWrapper0 FreepsOperatorWrapper, cr *utils.ConfigReader, ctx *Context) []FreepsBaseOperator {
	ops := []FreepsBaseOperator{}
	opVariation0 := opVariationWrapper0.opInstance.(FreepsOperatorWithConfig)
	opVariationSectionNames, err := cr.GetSectionNamesWithPrefix(opVariationWrapper0.GetName() + ".")
	if err != nil {
		ctx.logger.Errorf("Reading config for operator \"%v\" failed: %v", opVariationWrapper0.GetName(), err)
		return nil
	}
	opVariationSectionNames = append(opVariationSectionNames, opVariationWrapper0.GetName())
	for _, opVariationSectionName := range opVariationSectionNames {
		conf := opVariation0.GetDefaultConfig()
		if conf == nil {
			ops = append(ops, &FreepsOperatorWrapper{opInstance: opVariation0})
			continue
		}
		err := cr.ReadSectionWithDefaults(opVariationSectionName, conf)
		if err != nil {
			ctx.logger.Errorf("Reading config for operator \"%v\" failed: %v", opVariationSectionName, err)
			continue
		}

		err = cr.WriteBackConfigIfChanged()
		if err != nil {
			ctx.logger.Errorf("Writing back config for operator \"%v\" failed: %v", opVariationSectionName, err)
		}

		// check if the config object has a field called "enabled" and if it is set to false
		// if it is set to false, we do not want to initialize the operator and return nil
		enabledField := reflect.ValueOf(conf).Elem().FieldByName("Enabled")
		if enabledField.IsValid() && enabledField.Kind() == reflect.Bool && !enabledField.Bool() {
			ctx.GetLogger().Debugf("Operator \"%v\" disabled", opVariationSectionName)
			continue
		}
		opVariation, err := opVariation0.InitCopyOfOperator(conf, ctx)
		if err != nil {
			ctx.logger.Errorf("Initializing operator \"%v\" failed: %v", opVariationSectionName, err)
			continue
		}
		opVariationWrapper := FreepsOperatorWrapper{opInstance: opVariation, opName: opVariationSectionName, config: conf}
		opVariationWrapper.createFunctionMap(ctx)
		ops = append(ops, &opVariationWrapper)
	}
	return ops
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

// getInitializedParamStruct returns the struct that is the third parameter of the function,
// if the struct implements the FreepsFunctionParameters interface, InitOptionalParameters is called and the struct is returned
func (o *FreepsOperatorWrapper) getInitializedParamStruct(f reflect.Type) (reflect.Value, FreepsFunctionParameters) {
	paramStruct := f.In(2)

	paramStructInstance := reflect.New(paramStruct)
	if !paramStructInstance.Type().Implements(reflect.TypeOf((*FreepsFunctionParameters)(nil)).Elem()) {
		return paramStructInstance, nil
	}
	ps := paramStructInstance.Interface().(FreepsFunctionParameters)
	ps.InitOptionalParameters(o.opInstance, f.Name())
	return paramStructInstance, ps
}

// createFunctionMap creates a map of all exported functions of the struct that return a struct that implements FreepsFunction
func (o *FreepsOperatorWrapper) createFunctionMap(ctx *Context) {
	o.functionMetaDataMap = make(map[string]FreepsFunctionMetaData)
	t := reflect.TypeOf(o.opInstance)
	v := reflect.ValueOf(o.opInstance)
	for i := 0; i < t.NumMethod(); i++ {
		ffType, err := getFreepsFunctionType(t.Method(i).Type)
		if err != nil {
			ctx.logger.Debugf("Function \"%v\" of operator \"%v\" is not a valid FreepsFunction: %v\n", t.Method(i).Name, o.GetName(), err)
			continue
		}
		// check if the third paramter implements the FreepsFunctionParameters interface, if it does not but has methods, log a warning
		if ffType == FreepsFunctionTypeWithArguments || ffType == FreepsFunctionTypeFullSignature {
			paramStruct, ps := o.getInitializedParamStruct(t.Method(i).Type)
			if ps == nil && paramStruct.NumMethod() > 0 {
				ctx.logger.Warnf("Function \"%v\" of operator \"%v\" has a third parameter that does not implement the FreepsFunctionParameters interface but has methods", t.Method(i).Name, o.GetName())
			}
		}
		o.functionMetaDataMap[utils.StringToLower(t.Method(i).Name)] = FreepsFunctionMetaData{Name: t.Method(i).Name, FuncValue: v.Method(i), FuncType: ffType}
	}
}

// GetName returns the name of the struct opClass
func (o *FreepsOperatorWrapper) GetName() string {
	if o.opName != "" {
		return o.opName
	}

	t := reflect.TypeOf(o.opInstance)
	fullName := t.Elem().Name()
	if utils.StringStartsWith(fullName, "Operator") {
		return fullName[8:]
	}
	if utils.StringStartsWith(fullName, "Op") {
		return fullName[2:]
	}
	return fullName
}

// Execute gets the FreepsFunction by name, assignes all parameters based on the args map and calls the function
func (o *FreepsOperatorWrapper) Execute(ctx *Context, function string, args map[string]string, mainInput *OperatorIO) *OperatorIO {
	ffm := o.getFunctionMetaData(function)
	if ffm == nil {
		return MakeOutputError(http.StatusNotFound, fmt.Sprintf("Function \"%v\" not found", function))
	}

	// execute function immediately if the FreepsFunctionType indicates it needs no arguments
	switch ffm.FuncType {
	case FreepsFunctionTypeSimple:
		outValue := ffm.FuncValue.Call([]reflect.Value{})
		return outValue[0].Interface().(*OperatorIO)
	case FreepsFunctionTypeContextOnly:
		outValue := ffm.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx)})
		return outValue[0].Interface().(*OperatorIO)
	case FreepsFunctionTypeContextAndInput:
		outValue := ffm.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(mainInput)})
		return outValue[0].Interface().(*OperatorIO)
	}

	//TODO(HR): ensure that args are lowercase
	lowercaseArgs := map[string]string{}
	for k, v := range args {
		lowercaseArgs[utils.StringToLower(k)] = v
	}

	// create an initialized instance of the parameter struct
	paramStruct, ps := o.getInitializedParamStruct(ffm.FuncValue.Type())

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

	if ps != nil {
		// verify parameters before executing the function
		res := ps.VerifyParameters(o.opInstance)
		if res != nil && res.IsError() {
			return res
		}
	}

	if ffm.FuncType == FreepsFunctionTypeWithArguments {
		outValue := ffm.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(mainInput), paramStruct.Elem()})
		return outValue[0].Interface().(*OperatorIO)
	}
	if ffm.FuncType == FreepsFunctionTypeFullSignature {
		// pass on case sensitive arguments to function, but only the ones left in the lowercaseArgs map
		caseArgs := map[string]string{}
		for k, v := range args {
			if _, ok := lowercaseArgs[utils.StringToLower(k)]; ok {
				caseArgs[k] = v
			}
		}
		outValue := ffm.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(mainInput), paramStruct.Elem(), reflect.ValueOf(caseArgs)})
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
	ffm := o.getFunctionMetaData(function)
	if ffm == nil {
		return res
	}

	switch ffm.FuncType {
	case FreepsFunctionTypeSimple, FreepsFunctionTypeContextOnly, FreepsFunctionTypeContextAndInput:
		return res
	}

	//TODO(HR): ensure that args are lowercase
	lowercaseArgs := map[string]string{}
	for k, v := range otherArgs {
		lowercaseArgs[utils.StringToLower(k)] = v
	}

	// create an initialized instance of the parameter struct
	paramStruct, ps := o.getInitializedParamStruct(ffm.FuncValue.Type())
	if ps == nil {
		// common arg suggestions if the parameter struct does not implement the FreepsFunctionParameters interface
		return ParamListToParamMap(o.GetCommonParameterSuggestions(paramStruct, utils.StringToLower(argName)))
	}

	failOnError := false

	//set all required parameters of the FreepsFunction
	o.SetRequiredFreepsFunctionParameters(paramStruct, lowercaseArgs, failOnError)
	o.SetOptionalFreepsFunctionParameters(paramStruct, lowercaseArgs, failOnError)

	res = ps.GetArgSuggestions(o.opInstance, utils.StringToLower(function), utils.StringToLower(argName), lowercaseArgs)
	if res == nil || len(res) == 0 {
		return ParamListToParamMap(o.GetCommonParameterSuggestions(paramStruct, utils.StringToLower(argName)))
	}
	return res
}

// GetConfig returns the config of the operator
func (o *FreepsOperatorWrapper) GetConfig() interface{} {
	return o.config
}

// Shutdown calls the Shutdown method of the FreepsOperator if it exists
func (o *FreepsOperatorWrapper) Shutdown(ctx *Context) {
	opShutdown, ok := o.opInstance.(FreepsOperatorWithShutdown)
	if ok {
		opShutdown.Shutdown(ctx)
	}
}

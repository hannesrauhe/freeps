package operatorbuilder

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
)

// FreepsGenericOperator is the interface structs need to implement so GenericOperatorBuilder can create a FreepsOperator from them
type FreepsGenericOperator interface {
}

// FreepsGenericOperatorWithConfig adds the GetConfig() method to FreepsGenericOperator
type FreepsGenericOperatorWithConfig interface {
	FreepsGenericOperator
	//GetConfig returns the config struct of the operator that is filled wiht the values from the config file
	GetConfig() interface{}
	//Init is called after the config is read and the operator is created
	Init() error
}

// FreepsGenericOperatorWithShutdown adds the Shutdown() method to FreepsGenericOperatorWithConfig
type FreepsGenericOperatorWithShutdown interface {
	FreepsGenericOperatorWithConfig
	Shutdown(ctx *base.Context)
}

// FreepsGenericFunction is the interface that all functions that can be called by GenericOperatorBuilder.Execute must implement
type FreepsGenericFunction interface {
	// Run is called whenever a user requests the function to be executed
	Run(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO
}

// GenericOperatorBuilder creates everyt necessary to be a FreepsOperator function by reflection
type GenericOperatorBuilder struct {
	opInstance          FreepsGenericOperator
	functionMetaDataMap map[string]reflect.Value
}

var _ base.FreepsOperator = &GenericOperatorBuilder{}

// MakeGenericOperator creates a FreepsOperator from any struct that implements FreepsGenericOperator
func MakeGenericOperator(anyClass FreepsGenericOperator, cr *utils.ConfigReader) *GenericOperatorBuilder {
	if anyClass == nil {
		return nil
	}

	op := &GenericOperatorBuilder{opInstance: anyClass}
	err := op.init(cr)
	if err != nil {
		logrus.Errorf("Initializing operator \"%v\" failed: %v", op.GetName(), err)
		return nil
	}
	return op
}

func (o *GenericOperatorBuilder) init(cr *utils.ConfigReader) error {
	o.functionMetaDataMap = o.createFunctionMap()
	if len(o.functionMetaDataMap) == 0 {
		// this is a fatal error that should be fixed by the developer of the operator
		panic(fmt.Sprintf("No functions found for operator \"%v\"", o.GetName()))
	}

	if cr == nil {
		// no config reader might mean testing, just return
		return nil
	}

	confOp, ok := o.opInstance.(FreepsGenericOperatorWithConfig)
	if !ok {
		return nil
	}

	conf := confOp.GetConfig()
	if conf == nil {
		return nil
	}
	err := cr.ReadSectionWithDefaults(o.GetName(), &conf)
	if err != nil {
		return fmt.Errorf("Reading config for operator \"%v\" failed: %v", o.GetName(), err)
	}

	err = cr.WriteBackConfigIfChanged()
	if err != nil {
		return fmt.Errorf("Writing back config for operator \"%v\" failed: %v", o.GetName(), err)
	}

	initOp, ok := o.opInstance.(FreepsGenericOperatorWithShutdown)
	if !ok {
		return nil
	}
	initOp.Init()
	return nil
}

// getFunction returns the function with the given name (case insensitive)
func (o *GenericOperatorBuilder) getFunction(name string) *reflect.Value {
	name = utils.StringToLower(name)
	if f, ok := o.functionMetaDataMap[name]; ok {
		return &f
	}
	return nil
}

// createFunctionMap creates a map of all exported functions of the struct that return a struct that implements FreepsFunction
func (o *GenericOperatorBuilder) createFunctionMap() map[string]reflect.Value {
	funcMap := make(map[string]reflect.Value)
	t := reflect.TypeOf(o.opInstance)
	v := reflect.ValueOf(o.opInstance)
	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		if method.Type.NumOut() != 1 {
			continue
		}
		if method.Type.NumIn() != 1 {
			continue
		}

		ff := method.Type.Out(0)
		if ff.Kind() != reflect.Ptr {
			if ff.Kind() == reflect.Struct {
				fmt.Printf("Warning: Function \"%v\" of operator \"%v\" returns a struct instead of a pointer to a struct. This is ignored by freeps\n", method.Name, o.GetName())
			}
			continue
		}

		if !ff.Implements(reflect.TypeOf((*FreepsGenericFunction)(nil)).Elem()) {
			continue
		}
		funcMap[utils.StringToLower(method.Name)] = v.Method(i)
	}
	return funcMap
}

func isSupportedFieldType(field reflect.Value) bool {
	kind := field.Kind()
	return kind == reflect.Int || kind == reflect.String || kind == reflect.Float64 || kind == reflect.Bool
}

// isSupportedField returns true if the field is a primitive type or a pointer to a primitive type
func isSupportedField(field reflect.Value, mustBePtr bool) bool {
	if field.Kind() == reflect.Ptr && mustBePtr {
		return isSupportedFieldType(field.Elem())
	}
	if mustBePtr {
		return false
	}
	if !field.CanSet() {
		return false
	}
	return isSupportedFieldType(field)
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

// SetRequiredFreepsFunctionParameters sets the parameters of the FreepsFunction based on the vars map
func (o *GenericOperatorBuilder) SetRequiredFreepsFunctionParameters(freepsfunc *reflect.Value, vars map[string]string, failOnErr bool) *base.OperatorIO {
	//make sure all non-pointer fields of the FreepsFunction are set to the values of the vars map
	for i := 0; i < freepsfunc.Elem().NumField(); i++ {
		field := freepsfunc.Elem().Field(i)
		if !isSupportedField(field, false) {
			continue
		}

		fieldName := utils.StringToLower(freepsfunc.Elem().Type().Field(i).Name)

		//return an error if the field is not set in the vars map
		v, ok := vars[fieldName]
		if !ok {
			if failOnErr {
				return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" not found", fieldName))
			} else {
				continue
			}
		}

		// set the value of the field
		err := setSupportedField(field, v)
		if err != nil {
			if failOnErr {
				return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is invalid: %v", fieldName, err))
			}
			continue
		}

		delete(vars, fieldName)
	}
	return nil
}

// SetOptionalFreepsFunctionParameters sets the parameters of the FreepsFunction based on the vars map
func (o *GenericOperatorBuilder) SetOptionalFreepsFunctionParameters(freepsfunc *reflect.Value, vars map[string]string, failOnErr bool) *base.OperatorIO {
	// set all pointer fields of the FreepsFunction to the values of the vars map
	for i := 0; i < freepsfunc.Elem().NumField(); i++ {
		field := freepsfunc.Elem().Field(i)

		fieldName := utils.StringToLower(freepsfunc.Elem().Type().Field(i).Name)
		if !isSupportedField(field, true) {
			continue
		}

		v, ok := vars[fieldName]
		if !ok {
			continue
		}
		err := setSupportedField(field, v)
		if err != nil {
			if failOnErr {
				return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is invalid: %v", fieldName, err))
			}
			continue
		}
		delete(vars, fieldName)
	}
	return nil
}

// GetName returns the name of the struct opClass
func (o *GenericOperatorBuilder) GetName() string {
	t := reflect.TypeOf(o.opInstance)
	return utils.StringToLower(t.Elem().Name())
}

// Execute gets the FreepsFunction by name, assignes all parameters based on the vars map and calls the Run method of the FreepsFunction
func (o *GenericOperatorBuilder) Execute(ctx *base.Context, function string, vars map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	m := o.getFunction(function)
	if m == nil {
		return base.MakeOutputError(http.StatusNotFound, fmt.Sprintf("Function \"%v\" not found", function))
	}

	// call the method M to create a new instance of the requested FreepsFunction
	freepsfunc := m.Call([]reflect.Value{})[0]

	//TODO(HR): ensure that vars are lowercase
	lowercaseVars := map[string]string{}
	for k, v := range vars {
		lowercaseVars[utils.StringToLower(k)] = v
	}

	//set all required parameters of the FreepsFunction
	err := o.SetRequiredFreepsFunctionParameters(&freepsfunc, lowercaseVars, true)
	if err != nil {
		return err
	}
	err = o.SetOptionalFreepsFunctionParameters(&freepsfunc, lowercaseVars, true)
	if err != nil {
		return err
	}
	// set remaining vars to the fields vars of Freepsfunction if it exists
	VarsField := freepsfunc.Elem().FieldByName("Vars")
	if VarsField.IsValid() {
		VarsField.Set(reflect.ValueOf(lowercaseVars))
	}

	//call the Run method of the FreepsFunction
	actualFunc := freepsfunc.Interface().(FreepsGenericFunction)
	return actualFunc.Run(ctx, mainInput)
}

// GetFunctions returns all methods of the opClass
func (o *GenericOperatorBuilder) GetFunctions() []string {
	list := []string{}

	for n := range o.functionMetaDataMap {
		list = append(list, n)
	}
	return list
}

// GetPossibleArgs returns all members of the return type of the method called fn
func (o *GenericOperatorBuilder) GetPossibleArgs(fn string) []string {
	list := []string{}

	m := o.getFunction(fn)
	if m == nil {
		return list
	}
	mt := m.Type()

	ft := mt.Out(0).Elem()
	fmt.Printf(ft.Name())
	for j := 0; j < ft.NumField(); j++ {
		arg := ft.Field(j)
		if !arg.IsExported() {
			continue
		}
		list = append(list, arg.Name)
	}

	return list
}

func (o *GenericOperatorBuilder) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown calls the shutdown method of the FreepsGenericOperator if it exists
func (o *GenericOperatorBuilder) Shutdown(ctx *base.Context) {
	opShutdown, ok := o.opInstance.(FreepsGenericOperatorWithShutdown)
	if ok {
		opShutdown.Shutdown(ctx)
	}
}

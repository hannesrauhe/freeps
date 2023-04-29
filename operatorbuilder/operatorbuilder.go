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

// SetRequiredFreepsFunctionParameters sets the parameters of the FreepsFunction based on the vars map
func (o *GenericOperatorBuilder) SetRequiredFreepsFunctionParameters(freepsfunc *reflect.Value, vars map[string]string) *base.OperatorIO {
	//make sure all non-pointer fields of the FreepsFunction are set to the values of the vars map
	for i := 0; i < freepsfunc.Elem().NumField(); i++ {
		field := freepsfunc.Elem().Field(i)
		if field.Kind() == reflect.Ptr {
			continue
		}
		// continue if the field is not exported
		if !field.CanSet() {
			continue
		}
		// continue if the field is not a primitive type
		if field.Kind() != reflect.Int && field.Kind() != reflect.String && field.Kind() != reflect.Float64 && field.Kind() != reflect.Bool {
			continue
		}

		fieldName := utils.StringToLower(freepsfunc.Elem().Type().Field(i).Name)

		//return an error if the field is not set in the vars map
		if _, ok := vars[fieldName]; !ok {
			return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" not found", fieldName))
		}
		//set the value of the field to the value of the vars map
		if field.CanSet() {
			// convert the value to the type of the field, return an error if the conversion fails
			switch field.Kind() {
			case reflect.Int:
				v, err := utils.StringToInt(vars[fieldName])
				if err != nil {
					return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is not an int", fieldName))
				}
				field.SetInt(int64(v))
			case reflect.String:
				field.SetString(vars[fieldName])
			case reflect.Float64:
				v, err := utils.StringToFloat64(vars[fieldName])
				if err != nil {
					return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is not a float", fieldName))
				}
				field.SetFloat(v)
			case reflect.Bool:
				v := utils.ParseBool(vars[fieldName])
				field.SetBool(v)
			default:
				return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Parameter \"%v\" is not supported", fieldName))
			}
			delete(vars, fieldName)
		} else {
			return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Parameter \"%v\" of FreepsFunction is not settable", fieldName))
		}
	}
	return nil
}

// SetOptionalFreepsFunctionParameters sets the parameters of the FreepsFunction based on the vars map
func (o *GenericOperatorBuilder) SetOptionalFreepsFunctionParameters(freepsfunc *reflect.Value, vars map[string]string) *base.OperatorIO {
	// set all pointer fields of the FreepsFunction to the values of the vars map
	for i := 0; i < freepsfunc.Elem().NumField(); i++ {
		field := freepsfunc.Elem().Field(i)

		if field.Kind() != reflect.Ptr {
			continue
		}
		if !field.CanSet() {
			continue
		}

		fieldName := utils.StringToLower(freepsfunc.Elem().Type().Field(i).Name)
		v, ok := vars[fieldName]
		if !ok {
			continue
		}

		// since the field is a pointer, we need to create a new instance of the type of the field
		// and set the value of the field to the new instance
		newField := reflect.New(field.Type().Elem())
		field.Set(newField)
		// convert the value to the type of the field, return an error if the conversion fails
		switch field.Elem().Kind() {
		case reflect.Int:
			v, err := utils.StringToInt(v)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is not an int", fieldName))
			}
			field.Elem().SetInt(int64(v))
		case reflect.String:
			field.Elem().SetString(v)
		case reflect.Float64:
			v, err := utils.StringToFloat64(v)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is not a float", fieldName))
			}
			field.Elem().SetFloat(v)
		case reflect.Bool:
			v := utils.ParseBool(v)
			field.Elem().SetBool(v)
		default:
			return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Parameter Type \"%v\" of \"%v\" is not supported", field.Elem().Kind(), fieldName))
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
	err := o.SetRequiredFreepsFunctionParameters(&freepsfunc, lowercaseVars)
	if err != nil {
		return err
	}
	err = o.SetOptionalFreepsFunctionParameters(&freepsfunc, lowercaseVars)
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

package operatorbuilder

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

// GenericOperator creates all necessary to be a FreepsOperator function by reflection
type GenericOperator struct {
	opClass             interface{}
	functionMetaDataMap map[string]reflect.Value
}

var _ base.FreepsOperator = &GenericOperator{}

// FreepsFunction is the interface that all functions that can be called by GenericOperator.Execute must implement
type FreepsFunction interface {
	Run(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO
}

// MakeGenericOperator creates a GenericOperator from any struct pointer
func MakeGenericOperator(anyClass interface{}) *GenericOperator {
	t := reflect.TypeOf(anyClass)
	if t.Kind() != reflect.Ptr {
		return nil
	}
	if t.Elem().Kind() != reflect.Struct {
		return nil
	}
	op := &GenericOperator{opClass: anyClass}
	op.init()
	return op
}

func (o *GenericOperator) init() {
	o.functionMetaDataMap = o.createFunctionMap()
	if len(o.functionMetaDataMap) == 0 {
		panic(fmt.Sprintf("No functions found for operator \"%v\"", o.GetName()))
	}
}

// getFunction returns the function with the given name (case insensitive)
func (o *GenericOperator) getFunction(name string) *reflect.Value {
	name = utils.StringToLower(name)
	if f, ok := o.functionMetaDataMap[name]; ok {
		return &f
	}
	return nil
}

// createFunctionMap creates a map of all exported functions of the struct that return a struct that implements FreepsFunction
func (o *GenericOperator) createFunctionMap() map[string]reflect.Value {
	funcMap := make(map[string]reflect.Value)
	t := reflect.TypeOf(o.opClass)
	v := reflect.ValueOf(o.opClass)
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

		if !ff.Implements(reflect.TypeOf((*FreepsFunction)(nil)).Elem()) {
			continue
		}
		funcMap[utils.StringToLower(method.Name)] = v.Method(i)
	}
	return funcMap
}

// SetRequiredFreepsFunctionParameters sets the parameters of the FreepsFunction based on the vars map
func (o *GenericOperator) SetRequiredFreepsFunctionParameters(freepsfunc *reflect.Value, vars map[string]string) *base.OperatorIO {
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
func (o *GenericOperator) SetOptionalFreepsFunctionParameters(freepsfunc *reflect.Value, vars map[string]string) *base.OperatorIO {
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
func (o *GenericOperator) GetName() string {
	t := reflect.TypeOf(o.opClass)
	return utils.StringToLower(t.Elem().Name())
}

// Execute gets the FreepsFunction by name, assignes all parameters based on the vars map and calls the Run method of the FreepsFunction
func (o *GenericOperator) Execute(ctx *base.Context, function string, vars map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
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
	actualFunc := freepsfunc.Interface().(FreepsFunction)
	return actualFunc.Run(ctx, mainInput)
}

// GetFunctions returns all methods of the opClass
func (o *GenericOperator) GetFunctions() []string {
	list := []string{}

	for n := range o.functionMetaDataMap {
		list = append(list, n)
	}
	return list
}

// GetPossibleArgs returns all members of the return type of the method called fn
func (o *GenericOperator) GetPossibleArgs(fn string) []string {
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

func (o *GenericOperator) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o *GenericOperator) Shutdown(ctx *base.Context) {
}

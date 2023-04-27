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
	opClass interface{}
}

var _ base.FreepsOperator = &GenericOperator{}

// FreepsFunction is the interface that all functions that can be called by GenericOperator.Execute must implement
type FreepsFunction interface {
	Run(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO
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
			return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter %v not found", fieldName))
		}
		//set the value of the field to the value of the vars map
		if field.CanSet() {
			// convert the value to the type of the field, return an error if the conversion fails
			switch field.Kind() {
			case reflect.Int:
				v, err := utils.StringToInt(vars[fieldName])
				if err != nil {
					return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter %v is not an int", fieldName))
				}
				field.SetInt(int64(v))
			case reflect.String:
				field.SetString(vars[fieldName])
			case reflect.Float64:
				v, err := utils.StringToFloat64(vars[fieldName])
				if err != nil {
					return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter %v is not a float", fieldName))
				}
				field.SetFloat(v)
			case reflect.Bool:
				v := utils.ParseBool(vars[fieldName])
				field.SetBool(v)
			default:
				return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Parameter %v is not supported", fieldName))
			}
			delete(vars, fieldName)
		} else {
			return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Parameter %v of FreepsFunction is not settable", fieldName))
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
		// continue if the field is not a pointer to a primitive type
		if field.Elem().Kind() != reflect.Int && field.Elem().Kind() != reflect.String && field.Elem().Kind() != reflect.Float64 && field.Elem().Kind() != reflect.Bool {
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
				return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter %v is not an int", fieldName))
			}
			field.Elem().SetInt(int64(v))
		case reflect.String:
			field.Elem().SetString(v)
		case reflect.Float64:
			v, err := utils.StringToFloat64(v)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter %v is not a float", fieldName))
			}
			field.Elem().SetFloat(v)
		case reflect.Bool:
			v := utils.ParseBool(v)
			field.Elem().SetBool(v)
		default:
			return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Parameter Type %v of %v is not supported", field.Elem().Kind(), fieldName))
		}
		delete(vars, fieldName)
	}
	return nil
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
	return &GenericOperator{opClass: anyClass}
}

// GetName returns the name of the struct opClass
func (o *GenericOperator) GetName() string {
	t := reflect.TypeOf(o.opClass)
	return t.Elem().Name()
}

// Execute gets the FreepsFunction by name, assignes all parameters based on the vars map and calls the Run method of the FreepsFunction
func (o *GenericOperator) Execute(ctx *base.Context, function string, vars map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	m := o.getFreepsFunctionByName(function)
	if m == nil {
		return base.MakeOutputError(http.StatusNotFound, fmt.Sprintf("Function %v not found", function))
	}
	freepsfunc := reflect.New(m.Type.Out(0))

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
	if freepsfunc.Elem().FieldByName("Vars").IsValid() {
		freepsfunc.Elem().FieldByName("Vars").Set(reflect.ValueOf(lowercaseVars))
	}

	//call the Run method of the FreepsFunction
	out := freepsfunc.MethodByName("Run").Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(mainInput)})
	//return the result of the Run method
	return out[0].Interface().(*base.OperatorIO)
}

// GetFunctions returns all methods of the opClass
func (o *GenericOperator) GetFunctions() []string {
	list := []string{}

	for _, m := range o.getAllFreepsFunctions() {
		list = append(list, utils.StringToLower(m.Name))
	}
	return list
}

// getFreepsFunctionByName returns the method of the opClass by name (case insensitive) if the method has no parameters and the returned struct is a FreepsFunction
func (o *GenericOperator) getFreepsFunctionByName(fn string) *reflect.Method {
	t := reflect.TypeOf(o.opClass)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		if m.Type.NumIn() != 1 {
			continue
		}
		if m.Type.NumOut() != 1 {
			continue
		}
		if m.Type.Out(0).Kind() != reflect.Struct {
			continue
		}
		// if !m.Type.Out(0).Implements(reflect.TypeOf((*FreepsFunction)(nil)).Elem()) {
		// 	continue
		// }
		if utils.StringToLower(m.Name) == utils.StringToLower(fn) {
			return &m
		}
	}
	return nil
}

// getAllFreepsFunctions returns all methods of the opClass that have no parameters and the returned struct is a FreepsFunction
func (o *GenericOperator) getAllFreepsFunctions() []*reflect.Method {
	list := []*reflect.Method{}

	t := reflect.TypeOf(o.opClass)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		if m.Type.NumIn() != 1 {
			continue
		}
		if m.Type.NumOut() != 1 {
			continue
		}
		ff := m.Type.Out(0)
		if ff.Kind() != reflect.Struct {
			continue
		}
		// if !ff.Implements(reflect.TypeOf((*FreepsFunction)(nil)).Elem()) {
		// 	continue
		// }
		list = append(list, &m)
	}
	return list
}

// GetPossibleArgs returns all members of the return type of the method called fn
func (o *GenericOperator) GetPossibleArgs(fn string) []string {
	list := []string{}

	m := o.getFreepsFunctionByName(fn)
	if m == nil {
		return list
	}

	ft := m.Type.Out(0)
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

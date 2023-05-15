package base

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/utils"
)

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

		fieldNameCase := field.Name
		fieldName := utils.StringToLower(fieldNameCase)
		if !isSupportedField(field, false) {
			continue
		}

		//return an error if the field is not set in the args map
		v, ok := args[fieldName]
		if !ok {
			jsonFieldName := ""
			// check if the caller used the JSON represantation of the field:
			switch jsonTag := field.Tag.Get("json"); jsonTag {
			case "-":
			case "":
				jsonFieldName = ""
			default:
				parts := strings.Split(jsonTag, ",")
				jsonFieldName := parts[0]
			}

			if jsonFieldName!="" {
				v = args[jsonFieldName]
			}

			if v=="" {
				if failOnErr {
					return MakeOutputError(http.StatusBadRequest, fmt.Sprintf("required Parameter \"%v\" is missing", fieldNameCase))
				} else {
					continue
				}
			}
		}

		// set the value of the field
		err := setSupportedField(field, v)
		if err != nil {
			if failOnErr {
				return MakeOutputError(http.StatusBadRequest, fmt.Sprintf("Parameter \"%v\" is invalid: %v", fieldNameCase, err))
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

// ParamListToParamMap is a helper function that converts a list of strings to a map of strings (key==value)
func ParamListToParamMap(args []string) map[string]string {
	argMap := map[string]string{}
	for _, arg := range args {
		argMap[arg] = arg
	}
	return argMap
}

// GetCommonParameterSuggestions returns the default suggestions for the argument argName of type argType
func (o *FreepsOperatorWrapper) GetCommonParameterSuggestions(parmStruct reflect.Value, paramName string) []string {
	paramKind := reflect.Invalid
	for i := 0; i < parmStruct.Elem().NumField(); i++ {
		field := parmStruct.Elem().Field(i)
		fieldName := utils.StringToLower(parmStruct.Elem().Type().Field(i).Name)
		if fieldName != paramName {
			continue
		}
		if isSupportedField(field, false) {
			paramKind = field.Kind()
			break
		}
		if isSupportedField(field, true) {
			paramKind = field.Type().Elem().Kind()
			break
		}
	}

	switch paramKind {
	case reflect.Bool:
		return []string{"true", "false"}
	case reflect.Float32, reflect.Float64:
		return []string{"0.0", "0.5", "1.0", "1.5", "2.0", "2.5", "3.0", "3.5", "4.0", "4.5", "5.0"}
	case reflect.Int64:
		if strings.Contains(paramName, "duration") || strings.Contains(paramName, "time") || strings.Contains(paramName, "age") {
			return []string{"100ms", "200ms", "500ms", "1s", "2s", "5s", "10s", "20s", "50s", "100s", "1m", "10m", "1h"}
		}
		return []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "100", "1000"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "100", "1000"}
	default:
		return []string{}
	}
}

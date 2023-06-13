package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseBool returns the bool value represented by string
func ParseBool(str string) bool {
	v, err := strconv.ParseBool(str)
	if err != nil {
		str = strings.ToLower(str)
		switch str {
		case "on", "yes":
			return true
		default:
			return false
		}
	}
	return v
}

// ConvertToInt64 converts a value to an int
func ConvertToInt64(v interface{}) (int64, error) {
	switch v.(type) {
	case int:
		return int64(v.(int)), nil
	case int64:
		return v.(int64), nil
	case int32:
		return int64(v.(int32)), nil
	case float64:
		return int64(math.Round(v.(float64))), nil
	case []byte:
		b := v.([]byte)
		if len(b) == 0 {
			return 0, fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as Int, array is empty", v, v)
		}
		return int64(b[0]), nil
	case string:
		vInt, err := strconv.ParseInt(v.(string), 10, 64)
		if err != nil {
			return 0, err
		}
		return vInt, nil
	}
	return 0, fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as Int", v, v)
}

// ConvertToFloat converts a value to a float64
func ConvertToFloat(v interface{}) (float64, error) {
	switch v.(type) {
	case int:
		return float64(v.(int)), nil
	case int64:
		return float64(v.(int64)), nil
	case int32:
		return float64(v.(int32)), nil
	case float64:
		return v.(float64), nil
	case string:
		vF, err := strconv.ParseFloat(v.(string), 64)
		if err != nil {
			return 0, err
		}
		return vF, nil
	}
	return 0, fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as Float64", v, v)
}

// ConvertToBool converts a value to a bool
func ConvertToBool(v interface{}) (bool, error) {
	switch v.(type) {
	case bool:
		return v.(bool), nil
	case []byte:
		b := v.([]byte)
		if len(b) == 0 {
			return false, fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as bool, array is empty", v, v)
		}
		return b[0] != 0, nil
	case string:
		vB, err := strconv.ParseBool(v.(string))
		if err != nil {
			return false, err
		}
		return vB, nil
	}
	return false, fmt.Errorf("Cannot parse \"%v\" of type \"%T\"  as Bool", v, v)
}

// ConvertToString converts a value to a string
func ConvertToString(v interface{}) (string, error) {
	switch v.(type) {
	case string:
		return v.(string), nil
	case bool:
		return strconv.FormatBool(v.(bool)), nil
	case int:
		return strconv.Itoa(v.(int)), nil
	case int64:
		return strconv.FormatInt(v.(int64), 10), nil
	case int32:
		return strconv.FormatInt(int64(v.(int32)), 10), nil
	case float64:
		return strconv.FormatFloat(v.(float64), 'f', -1, 64), nil
	}
	return "", fmt.Errorf("Cannot parse \"%v\" of type \"%T\" as String", v, v)
}

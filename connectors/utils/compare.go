package freepsutils

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

// IntParams contains the parameters for the Int comparisons
type IntParams struct {
	Key   *string
	Value int64
}

func (m *OpUtils) intGetValue(ctx *base.Context, input *base.OperatorIO, args IntParams) (int64, *base.OperatorIO) {
	var err error
	var v int64
	if args.Key != nil {
		argsmap, ferr := m.flatten(input)
		if ferr != nil {
			return 0, ferr
		}
		vi, ok := argsmap[*args.Key]
		if !ok {
			return 0, base.MakeOutputError(http.StatusExpectationFailed, "key %s not found", *args.Key)
		}
		v, err = utils.ConvertToInt64(vi)
	} else {
		v, err = utils.ConvertToInt64(input.Output)
	}
	if err != nil {
		return 0, base.MakeOutputError(http.StatusBadRequest, "input cannot be converted to int: %v", err)
	}
	return v, nil
}

// IntEqual compares the input with the given value and returns the input if it is equal to the given value
func (m *OpUtils) IntEqual(ctx *base.Context, input *base.OperatorIO, args IntParams) *base.OperatorIO {
	v, ferr := m.intGetValue(ctx, input, args)
	if ferr != nil {
		return ferr
	}

	if v == args.Value {
		return input
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "input is not equal to %d", args.Value)
}

// IntLessThan compares the input with the given value and returns the input if it is less than the given value
func (m *OpUtils) IntLessThan(ctx *base.Context, input *base.OperatorIO, args IntParams) *base.OperatorIO {
	v, ferr := m.intGetValue(ctx, input, args)
	if ferr != nil {
		return ferr
	}
	if v < args.Value {
		return input
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "input is not less than %d", args.Value)
}

// IntGreaterThan compares the input with the given value and returns the input if it is greater than the given value
func (m *OpUtils) IntGreaterThan(ctx *base.Context, input *base.OperatorIO, args IntParams) *base.OperatorIO {
	v, ferr := m.intGetValue(ctx, input, args)
	if ferr != nil {
		return ferr
	}
	if v > args.Value {
		return input
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "input is not greater than %d", args.Value)
}

// FloatParams contains the parameters for the Float comparisons
type FloatParams struct {
	Key   *string
	Value float64
}

func (m *OpUtils) floatGetValue(ctx *base.Context, input *base.OperatorIO, args FloatParams) (float64, *base.OperatorIO) {
	var err error
	var v float64
	if args.Key != nil {
		argsmap, ferr := m.flatten(input)
		if ferr != nil {
			return 0, ferr
		}
		vi, ok := argsmap[*args.Key]
		if !ok {
			return 0, base.MakeOutputError(http.StatusExpectationFailed, "key %s not found", *args.Key)
		}
		v, err = utils.ConvertToFloat(vi)
	} else {
		v, err = utils.ConvertToFloat(input.Output)
	}
	if err != nil {
		return 0, base.MakeOutputError(http.StatusBadRequest, "input cannot be converted to float: %v", err)
	}
	return v, nil
}

// FloatGreaterThan compares the input with the given value and returns the input if it is greater than the given value
func (m *OpUtils) FloatGreaterThan(ctx *base.Context, input *base.OperatorIO, args FloatParams) *base.OperatorIO {
	v, ferr := m.floatGetValue(ctx, input, args)
	if ferr != nil {
		return ferr
	}

	if v > args.Value {
		return input
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "input is not greater than %f", args.Value)
}

// FloatLessThan compares the input with the given value and returns the input if it is less than the given value
func (m *OpUtils) FloatLessThan(ctx *base.Context, input *base.OperatorIO, args FloatParams) *base.OperatorIO {
	v, ferr := m.floatGetValue(ctx, input, args)
	if ferr != nil {
		return ferr
	}
	if v < args.Value {
		return input
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "input is not less than %f", args.Value)
}

// StringParams contains the parameters for the String comparisons
type StringParams struct {
	Key   *string
	Value string
}

// StringEqual compares the input with the given value and returns the input if it is equal to the given value
func (m *OpUtils) StringEqual(ctx *base.Context, input *base.OperatorIO, args StringParams) *base.OperatorIO {
	var err error
	var v string
	if args.Key != nil {
		argsmap, ferr := m.flatten(input)
		if ferr != nil {
			return ferr
		}
		vi, ok := argsmap[*args.Key]
		if !ok {
			return base.MakeOutputError(http.StatusExpectationFailed, "key %s not found", *args.Key)
		}
		v, err = utils.ConvertToString(vi)
	} else {
		v, err = utils.ConvertToString(input.Output)
	}
	if err != nil {
		v = input.GetString()
	}

	if v == args.Value {
		return input
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "input is not equal to %s", args.Value)
}

// BoolParams contains the parameters for the Bool comparisons
type BoolParams struct {
	Key *string
}

func (m *OpUtils) boolConv(ctx *base.Context, input *base.OperatorIO, args BoolParams) (bool, error) {
	if args.Key != nil {
		argsmap, ferr := m.flatten(input)
		if ferr != nil {
			return false, ferr.GetError()
		}
		vi, ok := argsmap[*args.Key]
		if !ok {
			return false, fmt.Errorf("key %s not found", *args.Key)
		}
		return utils.ConvertToBool(vi)
	}

	return utils.ConvertToBool(input.Output)
}

// IsTrue returns the input if it is true
func (m *OpUtils) IsTrue(ctx *base.Context, input *base.OperatorIO, args BoolParams) *base.OperatorIO {
	v, err := m.boolConv(ctx, input, args)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	if v {
		return input
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "input is not true")
}

// IsFalse returns the input if the input is false
func (m *OpUtils) IsFalse(ctx *base.Context, input *base.OperatorIO, args BoolParams) *base.OperatorIO {
	v, err := m.boolConv(ctx, input, args)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	if !v {
		return input
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "input is not false")
}

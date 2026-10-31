package device

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

// ActionParam is the single value a caller passes to an action, with optional
// readable-to-raw translation. Actions either have no parameter or exactly one.
type ActionParam struct {
	Name   string            `doc:"argument name shown to the user, e.g. temperature"`
	Raw    string            `json:",omitempty" doc:"argument name the operator expects, defaults to Name"`
	Type   string            `doc:"human readable type: string, int, float or bool"`
	Doc    string            `json:",omitempty"`
	Scale  float64           `json:",omitempty" doc:"raw = value * Scale, 0 and 1 mean no conversion"`
	Values map[string]string `json:",omitempty" doc:"readable label to raw value, unlisted values pass through"`
}

// rawName returns the argument name the target operator expects.
func (p ActionParam) rawName() string {
	if p.Raw == "" {
		return p.Name
	}
	return p.Raw
}

// Action is a command that can be executed on a device, as plain data so any source can produce it.
type Action struct {
	Name     string
	Operator string            `doc:"target operator of the action"`
	Function string            `doc:"target function in that operator"`
	Fixed    map[string]string `json:",omitempty" doc:"always-sent args, they win over the caller value"`
	Param    *ActionParam      `json:",omitempty" doc:"the value the caller passes as the value argument, nil for actions without one"`
}

// ActionInfo is the user facing projection of an action, without the raw call details.
type ActionInfo struct {
	Param *ActionParamInfo `json:",omitempty"`
}

// ActionParamInfo is the user facing description of the action parameter.
type ActionParamInfo struct {
	Name   string
	Type   string
	Doc    string            `json:",omitempty"`
	Values map[string]string `json:",omitempty"`
}

// DeviceActions is one entry of the per category action listing.
type DeviceActions struct {
	ID      string
	Alias   string // echoed so a caller can map the id to a name without a second request
	Actions []string
}

// info returns the user facing projection of an action.
func (a *Action) info() ActionInfo {
	if a.Param == nil {
		return ActionInfo{}
	}
	p := a.Param
	return ActionInfo{Param: &ActionParamInfo{Name: p.Name, Type: p.Type, Doc: p.Doc, Values: p.Values}}
}

// argumentDescriptions returns the parameter in the shape the flowbuilder metadata API uses.
func (a *Action) argumentDescriptions() []base.ArgumentDescription {
	if a.Param == nil {
		return []base.ArgumentDescription{}
	}
	return []base.ArgumentDescription{{Name: a.Param.Name, Type: a.Param.Type, Required: true, Description: a.Param.Doc}}
}

// buildArgs translates the caller value into the raw operator arguments; Fixed args are never overwritten.
func (a *Action) buildArgs(value string) (base.FunctionArguments, error) {
	args := base.MakeEmptyFunctionArguments()
	for k, v := range a.Fixed {
		args.Set(k, []string{v})
	}
	if a.Param == nil {
		return args, nil
	}
	if value == "" {
		return nil, fmt.Errorf("action \"%s\" needs a %s", a.Name, a.Param.Name)
	}
	raw, err := a.Param.toRaw(value)
	if err != nil {
		return nil, fmt.Errorf("value for action \"%s\" is invalid: %v", a.Name, err)
	}
	args.Set(a.Param.rawName(), []string{raw})
	return args, nil
}

// toRaw converts one readable value into the raw value the operator expects.
func (p ActionParam) toRaw(value string) (string, error) {
	for label, raw := range p.Values {
		if utils.StringEqualsIgnoreCase(label, value) {
			return raw, nil
		}
	}
	if p.Scale == 0 || p.Scale == 1 {
		return value, nil
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return "", fmt.Errorf("\"%s\" is not a number and not one of %s", value, strings.Join(p.valueNames(), ", "))
	}
	scaled := number * p.Scale
	if strings.HasPrefix(p.Type, "int") {
		// the target argument is an int, so a scaled value must not come out with decimals
		return strconv.Itoa(int(math.Round(scaled))), nil
	}
	return strconv.FormatFloat(scaled, 'f', -1, 64), nil
}

func (p ActionParam) valueNames() []string {
	names := make([]string, 0, len(p.Values))
	for k := range p.Values {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// valueSuggestions returns the readable labels as value suggestions, the label itself is what
// a caller sends and toRaw translates back to the raw value.
func (p ActionParam) valueSuggestions() map[string]string {
	sug := make(map[string]string, len(p.Values))
	for label := range p.Values {
		sug[label] = label
	}
	return sug
}

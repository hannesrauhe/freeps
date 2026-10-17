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

// ActionParam is one parameter of an action, with optional readable-to-raw translation.
type ActionParam struct {
	Name     string            `doc:"argument name the caller uses, e.g. temperature"`
	Raw      string            `json:",omitempty" doc:"argument name the operator expects, defaults to Name"`
	Type     string            `doc:"human readable type: string, int, float or bool"`
	Doc      string            `json:",omitempty"`
	Optional bool              `json:",omitempty" doc:"the caller may leave it out"`
	Scale    float64           `json:",omitempty" doc:"raw = value * Scale, 0 and 1 mean no conversion"`
	Values   map[string]string `json:",omitempty" doc:"readable label to raw value, unlisted values pass through"`
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
	Fixed    map[string]string `json:",omitempty" doc:"always-sent args, they win over caller args"`
	Params   []ActionParam     `json:",omitempty" doc:"readable parameters the caller may pass"`
}

// ActionInfo is the user facing projection of an action, without the raw call details.
type ActionInfo struct {
	Params []ActionParamInfo `json:",omitempty"`
}

// ActionParamInfo is the user facing description of one action parameter.
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
	var info ActionInfo
	for _, p := range a.Params {
		info.Params = append(info.Params, ActionParamInfo{Name: p.Name, Type: p.Type, Doc: p.Doc, Values: p.Values})
	}
	return info
}

// argumentDescriptions returns the parameters in the shape the flowbuilder metadata API uses.
func (a *Action) argumentDescriptions() []base.ArgumentDescription {
	list := make([]base.ArgumentDescription, 0, len(a.Params))
	for _, p := range a.Params {
		list = append(list, base.ArgumentDescription{Name: p.Name, Type: p.Type, Required: !p.Optional, Description: p.Doc})
	}
	return list
}

// buildArgs translates caller arguments into the raw operator arguments; Fixed args are never overwritten.
func (a *Action) buildArgs(callerArgs base.FunctionArguments) (base.FunctionArguments, error) {
	args := base.MakeEmptyFunctionArguments()
	for k, v := range a.Fixed {
		args.Set(k, []string{v})
	}

	for _, p := range a.Params {
		if !callerArgs.Has(p.Name) {
			if p.Optional {
				continue
			}
			return nil, fmt.Errorf("parameter \"%s\" is missing, %s expects %s", p.Name, a.Name, strings.Join(a.paramNames(), ", "))
		}
		raw, err := p.toRaw(callerArgs.Get(p.Name))
		if err != nil {
			return nil, fmt.Errorf("parameter \"%s\" of action \"%s\" is invalid: %v", p.Name, a.Name, err)
		}
		args.Set(p.rawName(), []string{raw})
	}

	for _, key := range callerArgs.GetOriginalKeys() {
		if a.hasParam(key) || a.isFixed(key) {
			continue
		}
		return nil, fmt.Errorf("unknown parameter \"%s\" for action \"%s\", expected one of: %s", key, a.Name, strings.Join(a.paramNames(), ", "))
	}

	return args, nil
}

// toRaw converts one readable parameter value into the raw value the operator expects.
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

func (a *Action) hasParam(name string) bool {
	for _, p := range a.Params {
		if utils.StringEqualsIgnoreCase(p.Name, name) {
			return true
		}
	}
	return false
}

func (a *Action) isFixed(name string) bool {
	for k := range a.Fixed {
		if utils.StringEqualsIgnoreCase(k, name) {
			return true
		}
	}
	return false
}

func (a *Action) paramNames() []string {
	names := make([]string, 0, len(a.Params))
	for _, p := range a.Params {
		names = append(names, p.Name)
	}
	if len(names) == 0 {
		return []string{"no parameters"}
	}
	sort.Strings(names)
	return names
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

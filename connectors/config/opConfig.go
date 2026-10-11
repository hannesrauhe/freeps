package opconfig

import (
	"encoding/json"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
)

// OpConfig is a FreepsOperator that can be used to retrieve and modify the config
type OpConfig struct {
	CR *utils.ConfigReader
	GE *freepsflow.FlowEngine
}

var _ base.FreepsOperator = &OpConfig{}

// SectionParams contains the parameters for the GetSection function
type SectionParams struct {
	SectionName string
}

// SectionNameSuggestions returns all section names, used for every function with a SectionName argument
func (oc *OpConfig) SectionNameSuggestions(otherArgs base.FunctionArguments) map[string]string {
	retMap := make(map[string]string)

	sections, err := oc.CR.GetSectionNames()
	if err != nil {
		return retMap
	}

	for _, section := range sections {
		retMap[section] = utils.StringToLower(section)
	}
	return retMap
}

// GetSection returns the section with the given name
func (oc *OpConfig) GetSection(ctx *base.Context, mainInput *base.OperatorIO, args SectionParams) *base.OperatorIO {
	b, err := oc.CR.GetSectionBytes(args.SectionName)
	if err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	return base.MakeByteOutput(b)
}

// RemoveSection removes the given section from the config file
func (oc *OpConfig) RemoveSection(ctx *base.Context, mainInput *base.OperatorIO, args SectionParams) *base.OperatorIO {
	err := oc.CR.RemoveSection(args.SectionName)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error removing section \"%v\": %v", args.SectionName, err)
	}
	err = oc.CR.WriteBackConfigIfChanged()
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error writing config: %v", err)
	}
	return base.MakeEmptyOutput()
}

// KeyParams contains the parameters for the GetKey function
type KeyParams struct {
	SectionName string
	Key         string
}

// KeySuggestions returns the keys contained in the selected section, used for every function with a Key argument
func (oc *OpConfig) KeySuggestions(otherArgs base.FunctionArguments) map[string]string {
	retMap := make(map[string]string)

	b, err := oc.CR.GetSectionBytes(otherArgs.Get("sectionname"))
	if err != nil || len(b) == 0 {
		return retMap
	}
	var section map[string]interface{}
	if err := json.Unmarshal(b, &section); err != nil {
		return retMap
	}
	for k := range section {
		retMap[k] = k
	}
	return retMap
}

// SetKeyParams contains the parameters for the SetKey function
type SetKeyParams struct {
	SectionName string
	Key         string
	Value       string
}

// GetKey returns a single key from a section as plain text
func (oc *OpConfig) GetKey(ctx *base.Context, mainInput *base.OperatorIO, args KeyParams) *base.OperatorIO {
	b, err := oc.CR.GetSectionBytes(args.SectionName)
	if err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	if len(b) == 0 {
		return base.MakeOutputError(http.StatusNotFound, "Section \"%v\" not found", args.SectionName)
	}
	var section map[string]interface{}
	if err := json.Unmarshal(b, &section); err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	// keys are matched case-insensitively, like most of the config handling
	var value interface{}
	for k, v := range section {
		if utils.StringToLower(k) == utils.StringToLower(args.Key) {
			value = v
			break
		}
	}
	if value == nil {
		return base.MakeOutputError(http.StatusNotFound, "Key \"%v\" not found in section \"%v\"", args.Key, args.SectionName)
	}
	if s, ok := value.(string); ok {
		return base.MakePlainOutput(s)
	}
	return base.MakeOutputInferType(value)
}

// SetKey writes a single key into a section, creating the section if it does not exist
func (oc *OpConfig) SetKey(ctx *base.Context, mainInput *base.OperatorIO, args SetKeyParams) *base.OperatorIO {
	section := map[string]interface{}{}
	b, err := oc.CR.GetSectionBytes(args.SectionName)
	if err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &section); err != nil {
			return base.MakeInternalServerErrorOutput(err)
		}
		// replace an existing key case-insensitively, like GetKey matches
		for k := range section {
			if utils.StringToLower(k) == utils.StringToLower(args.Key) {
				if k != args.Key {
					delete(section, k)
				}
				break
			}
		}
	}
	section[args.Key] = args.Value
	if err := oc.CR.WriteSection(args.SectionName, section, true); err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error writing key \"%v\": %v", args.Key, err)
	}
	return base.MakeEmptyOutput()
}

// WriteSection writes the given section to the config file
func (oc *OpConfig) WriteSection(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO {
	args, err := mainInput.ParseFormData()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err.Error())
	}

	if !args.Has("sectionName") {
		return base.MakeOutputError(http.StatusBadRequest, "Missing sectionName")
	}
	if !args.Has("sectionBytes") {
		return base.MakeOutputError(http.StatusBadRequest, "Missing sectionBytes")
	}

	err = oc.CR.WriteSectionBytes(args.Get("sectionName"), []byte(args.Get("sectionBytes")))
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err.Error())
	}
	err = oc.CR.WriteBackConfigIfChanged()
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error writing config: %v", err)
	}
	return base.MakeEmptyOutput()
}

// GetOperatorConfigParams contains the parameters for the GetSection function
type GetOperatorConfigParams struct {
	OperatorName string
}

// OperatorNameSuggestions returns a map of possible arguments for the given function and argument name
func (p *GetOperatorConfigParams) OperatorNameSuggestions(otherArgs base.FunctionArguments, oc *OpConfig) map[string]string {
	retMap := make(map[string]string)
	operators := oc.GE.GetOperators()
	for _, op := range operators {
		retMap[op] = utils.StringToLower(op)
	}
	return retMap
}

// GetOperatorConfig returns the default config for a given section
func (oc *OpConfig) GetOperatorConfig(ctx *base.Context, mainInput *base.OperatorIO, args GetOperatorConfigParams) *base.OperatorIO {
	op := oc.GE.GetOperator(args.OperatorName)
	if op == nil {
		return base.MakeOutputError(404, "Unknown operator %v", args.OperatorName)
	}
	fop, ok := op.(*base.FreepsOperatorWrapper)
	if !ok {
		return base.MakeOutputError(500, "Operator %v does not support config", args.OperatorName)
	}
	return base.MakeObjectOutput(fop.GetConfig())
}

package opconfig

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

// OpConfig is a FreepsOperator that can be used to retrieve and modify the config
type OpConfig struct {
	CR *utils.ConfigReader
	GE *freepsgraph.GraphEngine
}

var _ base.FreepsOperator = &OpConfig{}

// SectionParams contains the parameters for the GetSection function
type SectionParams struct {
	SectionName string
}

var _ base.FreepsFunctionParameters = &SectionParams{}

// InitOptionalParameters initializes the optional (pointer) arguments of the parameters struct with default values
func (p *SectionParams) InitOptionalParameters(operator base.FreepsOperator, fn string) {
}

// GetArgSuggestions returns a map of possible arguments for the given function and argument name
func (p *SectionParams) GetArgSuggestions(operator base.FreepsOperator, fn string, argName string, otherArgs map[string]string) map[string]string {
	retMap := make(map[string]string)

	sections, err := operator.(*OpConfig).CR.GetSectionNames()
	if err != nil {
		return retMap
	}

	for _, section := range sections {
		retMap[utils.StringToLower(section)] = section
	}
	return retMap
}

// VerifyParameters checks if the given parameters are valid
func (p *SectionParams) VerifyParameters(operator base.FreepsOperator) *base.OperatorIO {
	return nil
}

// GetSection returns the section with the given name
func (oc *OpConfig) GetSection(ctx *base.Context, mainInput *base.OperatorIO, args SectionParams) *base.OperatorIO {
	b, err := oc.CR.GetSectionBytes(args.SectionName)
	if err != nil {
		return base.MakeOutputError(500, err.Error())
	}
	return base.MakeByteOutput(b)
}

// RemoveSection removes the given section from the config file
func (oc *OpConfig) RemoveSection(ctx *base.Context, mainInput *base.OperatorIO, args SectionParams) *base.OperatorIO {
	err := oc.CR.RemoveSection(args.SectionName)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error removing section \"%v\": %v", args.SectionName, err))
	}
	err = oc.CR.WriteBackConfigIfChanged()
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error writing config: %v", err))
	}
	return base.MakeEmptyOutput()
}

// WriteSection writes the given section to the config file
func (oc *OpConfig) WriteSection(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO {
	args, err := mainInput.ParseFormData()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	if !args.Has("sectionName") {
		return base.MakeOutputError(http.StatusBadRequest, "Missing sectionName")
	}
	if !args.Has("sectionBytes") {
		return base.MakeOutputError(http.StatusBadRequest, "Missing sectionBytes")
	}

	err = oc.CR.WriteSectionBytes(args.Get("sectionName"), []byte(args.Get("sectionBytes")))
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	err = oc.CR.WriteBackConfigIfChanged()
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error writing config: %v", err))
	}
	return base.MakeEmptyOutput()
}

// GetOperatorConfigParams contains the parameters for the GetSection function
type GetOperatorConfigParams struct {
	OperatorName string
}

var _ base.FreepsFunctionParameters = &GetOperatorConfigParams{}

// InitOptionalParameters initializes the optional (pointer) arguments of the parameters struct with default values
func (p *GetOperatorConfigParams) InitOptionalParameters(operator base.FreepsOperator, fn string) {
}

// GetArgSuggestions returns a map of possible arguments for the given function and argument name
func (p *GetOperatorConfigParams) GetArgSuggestions(operator base.FreepsOperator, fn string, argName string, otherArgs map[string]string) map[string]string {
	retMap := make(map[string]string)
	operators := operator.(*OpConfig).GE.GetOperators()
	for _, op := range operators {
		retMap[utils.StringToLower(op)] = op
	}
	return retMap
}

// VerifyParameters checks if the given parameters are valid
func (p *GetOperatorConfigParams) VerifyParameters(operator base.FreepsOperator) *base.OperatorIO {
	return nil
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

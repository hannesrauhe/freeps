package opconfig

import (
	"fmt"
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

// SectionNameSuggestions returns a map of possible arguments for the given function and argument name
func (p *SectionParams) SectionNameSuggestions(oc *OpConfig) map[string]string {
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

// OperatorNameSuggestions returns a map of possible arguments for the given function and argument name
func (p *GetOperatorConfigParams) OperatorNameSuggestions(oc *OpConfig) map[string]string {
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

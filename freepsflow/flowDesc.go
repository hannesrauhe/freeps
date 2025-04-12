package freepsflow

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

// FlowOperationDesc defines which operator to execute with Arguments and where to take the input from
type FlowOperationDesc struct {
	Name               string `json:",omitempty"`
	Operator           string
	Function           string
	Arguments          base.FunctionArguments `json:",omitempty"`
	InputFrom          string                 `json:",omitempty"`
	ExecuteOnSuccessOf string                 `json:",omitempty"`
	ExecuteOnFailOf    string                 `json:",omitempty"`
	ArgumentsFrom      string                 `json:",omitempty"`
	UseMainArgs        bool                   `json:",omitempty"`
}

// ToQuicklink returns the URL to call a standalone-operation outside of a Flow
func (gop *FlowOperationDesc) ToQuicklink() string {
	var s strings.Builder
	s.WriteString("/" + gop.Operator)
	if gop.Function != "" {
		s.WriteString("/" + gop.Function)
	}
	if !gop.Arguments.IsEmpty() {
		s.WriteString("?")
	}
	for k, values := range gop.Arguments.GetOriginalCaseMap() {
		for _, v := range values {
			s.WriteString(url.QueryEscape(k) + "=" + url.QueryEscape(v) + "&")
		}
	}
	return s.String()
}

// FlowDesc contains a number of operations and defines which output to use
type FlowDesc struct {
	FlowID      string `json:",omitempty"` // is only assigned when the flow is added to the engine and will be overwritten
	DisplayName string
	Tags        []string
	Source      string
	OutputFrom  string
	Operations  []FlowOperationDesc
}

// HasAllTags return true if the FlowDesc contains all given tags
func (gd *FlowDesc) HasAllTags(expectedTags []string) bool {
	if expectedTags == nil || len(expectedTags) == 0 {
		return true
	}

	for _, expectedTag := range expectedTags {
		expectedTagKey, expectedTagValue := SplitTag(expectedTag)
		found := false
		for _, tag := range gd.Tags {
			tagKey, tagValue := SplitTag(tag)
			if utils.StringEqualsIgnoreCase(tagKey, expectedTagKey) && (tagValue == expectedTagValue || expectedTagValue == "") {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// HasAtLeastOneTag returns true if the FlowDesc contains at least one of the given tags
func (gd *FlowDesc) HasAtLeastOneTag(expectedTags []string) bool {
	if expectedTags == nil || len(expectedTags) == 0 {
		return true
	}

	for _, expectedTag := range expectedTags {
		expectedTagKey, expectedTagValue := SplitTag(expectedTag)
		for _, tag := range gd.Tags {
			tagKey, tagValue := SplitTag(tag)
			if utils.StringEqualsIgnoreCase(tagKey, expectedTagKey) && (tagValue == expectedTagValue || expectedTagValue == "") {
				return true
			}
		}
	}
	return false
}

// HasAtLeastOneTagPerGroup returns true if the FlowDesc contains at least one tag of each array
func (gd *FlowDesc) HasAtLeastOneTagPerGroup(tagGroups ...[]string) bool {
	if tagGroups == nil || len(tagGroups) == 0 {
		return true
	}

	for _, tags := range tagGroups {
		if !gd.HasAtLeastOneTag(tags) {
			return false
		}
	}
	return true
}

// AddTags adds a Tag to the description and removes duplicates
func (gd *FlowDesc) AddTags(tags ...string) {
	fakeSet := map[string]bool{}
	if gd.Tags != nil || len(gd.Tags) == 0 {
		for _, t := range gd.Tags {
			fakeSet[t] = true
		}
		for _, t := range tags {
			fakeSet[t] = true
		}
	}

	gd.Tags = make([]string, 0, len(fakeSet))
	for k := range fakeSet {
		gd.Tags = append(gd.Tags, k)
	}
}

// RemoveTag removes a Tag and duplicates in general from the description
func (gd *FlowDesc) RemoveTag(tag string) {
	if gd.Tags == nil || len(gd.Tags) == 0 {
		gd.Tags = []string{}
		return
	}
	fakeSet := map[string]bool{}
	for _, t := range gd.Tags {
		fakeSet[t] = true
	}
	delete(fakeSet, tag)
	gd.Tags = make([]string, 0, len(fakeSet))
	for k := range fakeSet {
		gd.Tags = append(gd.Tags, k)
	}
}

// GetTagValue returns the value of a given tag if that tag is set, or "" if tag is not set or doesn't have a value
func (gd *FlowDesc) GetTagValue(tagKey string) string {
	tm := utils.NewStringCIMap(map[string]string{})
	for _, t := range gd.Tags {
		k, v := SplitTag(t)
		tm.Append(k, v)
	}
	return tm.Get(tagKey)
}

// RenameOperation renames an operation oldName to newName everywhere in the Flow
func (gd *FlowDesc) RenameOperation(oldName string, newName string) {
	rename := func(ref *string) {
		if *ref == oldName {
			*ref = newName
		}
	}
	for i := range gd.Operations {
		rename(&gd.Operations[i].Name)
		rename(&gd.Operations[i].ArgumentsFrom)
		rename(&gd.Operations[i].InputFrom)
		rename(&gd.Operations[i].ExecuteOnSuccessOf)
		rename(&gd.Operations[i].ExecuteOnFailOf)
	}
	rename(&gd.OutputFrom)
}

// GetCompleteDesc initializes and validates the FlowDescription and returns a copy in order to create a Flow
func (gd *FlowDesc) GetCompleteDesc(flowID string, ge *FlowEngine) (*FlowDesc, error) {
	completeFlowDesc := *gd
	completeFlowDesc.FlowID = flowID
	if completeFlowDesc.DisplayName == "" && len(flowID) > 1 {
		completeFlowDesc.DisplayName = strings.ToUpper(flowID[0:1]) + flowID[1:]
	}
	completeFlowDesc.Operations = make([]FlowOperationDesc, len(gd.Operations))

	outputNames := make(map[string]bool)
	outputNames[ROOT_SYMBOL] = true

	if len(gd.Operations) == 0 {
		return &completeFlowDesc, errors.New("No operations defined")
	}

	if ge == nil {
		return &completeFlowDesc, errors.New("FlowEngine not set")
	}

	// create a copy of each operation and add it to the flow
	for i, op := range gd.Operations {
		if op.Name == ROOT_SYMBOL {
			return &completeFlowDesc, errors.New("Operation name cannot be " + ROOT_SYMBOL)
		}
		if outputNames[op.Name] {
			return &completeFlowDesc, errors.New("Operation name " + op.Name + " is used multiple times")
		}
		if op.Name == "" {
			op.Name = fmt.Sprintf("#%d", i)
		}
		if !ge.HasOperator(op.Operator) {
			return &completeFlowDesc, fmt.Errorf("Operation \"%v\" references unknown operator \"%v\"", op.Name, op.Operator)
		}
		if op.ArgumentsFrom != "" && outputNames[op.ArgumentsFrom] != true {
			return &completeFlowDesc, fmt.Errorf("Operation \"%v\" references unknown argumentsFrom \"%v\"", op.Name, op.ArgumentsFrom)
		}
		if op.InputFrom != "" && outputNames[op.InputFrom] != true {
			return &completeFlowDesc, fmt.Errorf("Operation \"%v\" references unknown inputFrom \"%v\"", op.Name, op.InputFrom)
		}
		if op.ExecuteOnSuccessOf != "" {
			if outputNames[op.ExecuteOnSuccessOf] != true {
				return &completeFlowDesc, fmt.Errorf("Operation \"%v\" references unknown ExecuteOnSuccessOf \"%v\"", op.Name, op.ExecuteOnSuccessOf)
			}
		}
		if op.ExecuteOnFailOf != "" {
			if outputNames[op.ExecuteOnFailOf] != true {
				return &completeFlowDesc, fmt.Errorf("Operation \"%v\" references unknown ExecuteOnFailOf \"%v\"", op.Name, op.ExecuteOnFailOf)
			}
			if op.ExecuteOnFailOf == op.InputFrom {
				return &completeFlowDesc, fmt.Errorf("Operation \"%v\" references the same InputFrom and ExecuteOnFailOf \"%v\"", op.Name, op.ExecuteOnFailOf)
			}
			if op.ExecuteOnFailOf == op.ExecuteOnSuccessOf {
				return &completeFlowDesc, fmt.Errorf("Operation \"%v\" references the same ExecuteOnSuccessOf and ExecuteOnFailOf \"%v\"", op.Name, op.ExecuteOnFailOf)
			}
		}
		outputNames[op.Name] = true
		completeFlowDesc.Operations[i] = op

		// op.args are not copied, because they aren't modified during execution
	}
	if gd.OutputFrom == "" {
		if len(gd.Operations) == 1 {
			completeFlowDesc.OutputFrom = completeFlowDesc.Operations[0].Name
		}
	} else if outputNames[gd.OutputFrom] != true {
		return &completeFlowDesc, fmt.Errorf("Flow Description references unknown outputFrom \"%v\"", gd.OutputFrom)
	}
	return &completeFlowDesc, nil
}

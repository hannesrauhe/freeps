package freepsgraph

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hannesrauhe/freeps/utils"
)

// GraphOperationDesc defines which operator to execute with Arguments and where to take the input from
type GraphOperationDesc struct {
	Name            string `json:",omitempty"`
	Operator        string
	Function        string
	Arguments       map[string]string `json:",omitempty"`
	InputFrom       string            `json:",omitempty"`
	ExecuteOnFailOf string            `json:",omitempty"`
	ArgumentsFrom   string            `json:",omitempty"`
	IgnoreMainArgs  bool              `json:",omitempty"` // deprecate
	UseMainArgs     bool              `json:",omitempty"`
}

// GraphDesc contains a number of operations and defines which output to use
type GraphDesc struct {
	GraphID     string `json:",omitempty"` // is only assigned when the graph is added to the engine and will be overwritten
	DisplayName string
	Tags        []string
	Source      string
	OutputFrom  string
	Operations  []GraphOperationDesc
}

// HasAllTags return true if the GraphDesc contains all given tags
func (gd *GraphDesc) HasAllTags(expectedTags []string) bool {
	if expectedTags == nil || len(expectedTags) == 0 {
		return true
	}

	for _, expectedTag := range expectedTags {
		expectedTagKey, expectedTagValue := SplitTag(expectedTag)
		found := false
		for _, tag := range gd.Tags {
			tagKey, tagValue := SplitTag(tag)
			if utils.StringCmpIgnoreCase(tagKey, expectedTagKey) && (tagValue == expectedTagValue || expectedTagValue == "") {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// HasAtLeastOneTag returns true if the GraphDesc contains at least one of the given tags
func (gd *GraphDesc) HasAtLeastOneTag(expectedTags []string) bool {
	if expectedTags == nil || len(expectedTags) == 0 {
		return true
	}

	for _, expectedTag := range expectedTags {
		expectedTagKey, expectedTagValue := SplitTag(expectedTag)
		for _, tag := range gd.Tags {
			tagKey, tagValue := SplitTag(tag)
			if utils.StringCmpIgnoreCase(tagKey, expectedTagKey) && (tagValue == expectedTagValue || expectedTagValue == "") {
				return true
			}
		}
	}
	return false
}

// HasAtLeastOneTagPerGroup returns true if the GraphDesc contains at least one tag of each array
func (gd *GraphDesc) HasAtLeastOneTagPerGroup(tagGroups ...[]string) bool {
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
func (gd *GraphDesc) AddTags(tags ...string) {
	fakeSet := map[string]bool{}
	for _, t := range gd.Tags {
		fakeSet[t] = true
	}
	for _, t := range tags {
		fakeSet[t] = true
	}

	gd.Tags = make([]string, 0, len(fakeSet))
	for k := range fakeSet {
		gd.Tags = append(gd.Tags, k)
	}
}

// RemoveTag removes a Tag and duplicates in general from the description
func (gd *GraphDesc) RemoveTag(tag string) {
	if gd.Tags == nil || len(gd.Tags) == 0 {
		gd.Tags = []string{tag}
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

// RenameOperation renames an operation oldName to newName everywhere in the Graph
func (gd *GraphDesc) RenameOperation(oldName string, newName string) {
	rename := func(ref *string) {
		if *ref == oldName {
			*ref = newName
		}
	}
	for i := range gd.Operations {
		rename(&gd.Operations[i].Name)
		rename(&gd.Operations[i].ArgumentsFrom)
		rename(&gd.Operations[i].InputFrom)
		rename(&gd.Operations[i].ExecuteOnFailOf)
	}
	rename(&gd.OutputFrom)
}

// GetCompleteDesc initializes and validates the GraphDescription and returns a copy in order to create a Graph
func (gd *GraphDesc) GetCompleteDesc(graphID string, ge *GraphEngine) (*GraphDesc, error) {
	completeGraphDesc := *gd
	completeGraphDesc.GraphID = graphID
	if completeGraphDesc.DisplayName == "" && len(graphID) > 1 {
		completeGraphDesc.DisplayName = strings.ToUpper(graphID[0:1]) + graphID[1:]
	}
	completeGraphDesc.Operations = make([]GraphOperationDesc, len(gd.Operations))

	outputNames := make(map[string]bool)
	outputNames[ROOT_SYMBOL] = true

	if len(gd.Operations) == 0 {
		return &completeGraphDesc, errors.New("No operations defined")
	}

	if ge == nil {
		return &completeGraphDesc, errors.New("GraphEngine not set")
	}

	// create a copy of each operation and add it to the graph
	for i, op := range gd.Operations {
		if op.Name == ROOT_SYMBOL {
			return &completeGraphDesc, errors.New("Operation name cannot be " + ROOT_SYMBOL)
		}
		if outputNames[op.Name] {
			return &completeGraphDesc, errors.New("Operation name " + op.Name + " is used multiple times")
		}
		if op.Name == "" {
			op.Name = fmt.Sprintf("#%d", i)
		}
		if !ge.HasOperator(op.Operator) {
			return &completeGraphDesc, fmt.Errorf("Operation \"%v\" references unknown operator \"%v\"", op.Name, op.Operator)
		}
		if op.ArgumentsFrom != "" && outputNames[op.ArgumentsFrom] != true {
			return &completeGraphDesc, fmt.Errorf("Operation \"%v\" references unknown argumentsFrom \"%v\"", op.Name, op.ArgumentsFrom)
		}
		if op.InputFrom == "" && i == 0 {
			op.InputFrom = ROOT_SYMBOL
		}
		if i == 0 || !op.IgnoreMainArgs { // deprecate automatically consuming main arguments with this
			op.UseMainArgs = true
		}
		if op.InputFrom != "" && outputNames[op.InputFrom] != true {
			return &completeGraphDesc, fmt.Errorf("Operation \"%v\" references unknown inputFrom \"%v\"", op.Name, op.InputFrom)
		}
		if op.ExecuteOnFailOf != "" {
			if outputNames[op.ExecuteOnFailOf] != true {
				return &completeGraphDesc, fmt.Errorf("Operation \"%v\" references unknown ExecuteOnFailOf \"%v\"", op.Name, op.ExecuteOnFailOf)
			}
			if op.ExecuteOnFailOf == op.InputFrom {
				return &completeGraphDesc, fmt.Errorf("Operation \"%v\" references the same InputFrom and ExecuteOnFailOf \"%v\"", op.Name, op.ExecuteOnFailOf)
			}
		}
		outputNames[op.Name] = true
		completeGraphDesc.Operations[i] = op

		// op.args are not copied, because they aren't modified during execution
	}
	if gd.OutputFrom == "" {
		if len(gd.Operations) == 1 {
			completeGraphDesc.OutputFrom = completeGraphDesc.Operations[0].Name
		}
	} else if outputNames[gd.OutputFrom] != true {
		return &completeGraphDesc, fmt.Errorf("Graph Description references unknown outputFrom \"%v\"", gd.OutputFrom)
	}
	return &completeGraphDesc, nil
}

package freepsgraph

import "time"

// GraphInfo holds the GraphDesc and some runtime info about the graph execution
type GraphInfo struct {
	Desc              GraphDesc
	LastExecutionTime time.Time
	ExecutionCounter  int64
}

// GraphOperationDesc defines which operator to execute with Arguments and where to take the input from
type GraphOperationDesc struct {
	Name            string `json:",omitempty"`
	Operator        string
	Function        string
	Arguments       map[string]string `json:",omitempty"`
	InputFrom       string            `json:",omitempty"`
	ExecuteOnFailOf string            `json:",omitempty"`
	ArgumentsFrom   string            `json:",omitempty"`
	IgnoreMainArgs  bool              `json:",omitempty"`
}

// GraphDesc contains a number of operations and defines which output to use
type GraphDesc struct {
	Tags       []string
	Source     string
	OutputFrom string
	Operations []GraphOperationDesc
}

// HasTags return true if the GraphDesc contains all given tags
func (gd *GraphDesc) HasTags(expectedTags []string) bool {
	if expectedTags == nil && len(expectedTags) == 0 {
		return true
	}

	for _, exexpectedTag := range expectedTags {
		found := false
		for _, tag := range gd.Tags {
			if tag == exexpectedTag {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// AddTag adds a Tag to the description and removes duplicates
func (gd *GraphDesc) AddTag(tag string) {
	if gd.Tags == nil || len(gd.Tags) == 0 {
		gd.Tags = []string{tag}
	}
	if tag == "" {
		return
	}
	fakeSet := map[string]bool{}
	for _, t := range gd.Tags {
		fakeSet[t] = true
	}
	fakeSet[tag] = true
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

package freepsgraph

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
			if tagKey == expectedTagKey && (tagValue == expectedTagValue || expectedTagValue == "") {
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
			if tagKey == expectedTagKey && (tagValue == expectedTagValue || expectedTagValue == "") {
				return true
			}
		}
	}
	return false
}

// HasAtLeastOneTagPerGroup returns true if the GraphDesc contains at least one tag of each array
func (gd *GraphDesc) HasAtLeastOneTagPerGroup(tagGroups [][]string) bool {
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

package utils

import (
	"sort"
	"time"
)

// TopKList is a top-k-list of strings with timestamps sorted by timestamp
type TopKList struct {
	entries []*TopKEntry
	maxLen  int
}

// TopKEntry is an entry in a TopKList
type TopKEntry struct {
	Value     string
	Timestamp time.Time
}

// NewTopKList creates a new TopKList
func NewTopKList(maxLen int) *TopKList {
	return &TopKList{maxLen: maxLen}
}

// Add adds a value to the TopKList and returns the value that was removed, nil if the list wasn't full
func (tkl *TopKList) Add(value string, timestamp time.Time) *string {
	tkl.entries = append(tkl.entries, &TopKEntry{Value: value, Timestamp: timestamp})
	if len(tkl.entries) == tkl.maxLen {
		// we need to sort the list now to make sure we remove the oldest entry at the next insert
		sort.Slice(tkl.entries, func(i, j int) bool {
			return tkl.entries[i].Timestamp.Before(tkl.entries[j].Timestamp)
		})
		return nil
	}

	if len(tkl.entries) > tkl.maxLen {
		// if timestamp is older than the oldest entry, we can skip the insert and return the value
		if timestamp.Before(tkl.entries[0].Timestamp) {
			return &value
		}

		sort.Slice(tkl.entries, func(i, j int) bool {
			return tkl.entries[i].Timestamp.Before(tkl.entries[j].Timestamp)
		})
		removed := tkl.entries[0].Value
		tkl.entries = tkl.entries[1:]
		return &removed
	}
	return nil
}

package freepsgraph

import (
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

// CollectedError store an error, the timestamp and the name of the graph and operation
type CollectedError struct {
	Input     *base.OperatorIO
	Error     string
	Time      time.Time
	GraphName string
	Operation *GraphOperationDesc
}

// CollectedErrors is a top-k-list of CollectedError
type CollectedErrors struct {
	errors []*CollectedError
	maxLen int
	mutex  sync.Mutex
}

// NewCollectedErrors creates a new CollectedErrors
func NewCollectedErrors(maxLen int) *CollectedErrors {
	return &CollectedErrors{maxLen: maxLen}
}

// AddError adds an error to the CollectedErrors
func (ce *CollectedErrors) AddError(input *base.OperatorIO, err *base.OperatorIO, graphName string, od *GraphOperationDesc) {
	ce.mutex.Lock()
	defer ce.mutex.Unlock()
	ce.errors = append(ce.errors, &CollectedError{Input: input, Error: err.GetString(), Time: time.Now(), GraphName: graphName, Operation: od})
	if len(ce.errors) > ce.maxLen {
		ce.errors = ce.errors[1:]
	}
}

// GetErrorsSince returns the error that occured in the given duration
func (ce *CollectedErrors) GetErrorsSince(d time.Duration) []*CollectedError {
	ce.mutex.Lock()
	defer ce.mutex.Unlock()
	var ret []*CollectedError
	for _, e := range ce.errors {
		if time.Since(e.Time) < d {
			ret = append(ret, e)
		}
	}
	return ret
}

// GetErrorsForGraph returns the error that occured in the given duration for a gien graph
func (ce *CollectedErrors) GetErrorsForGraph(since time.Time, graphName string) []*CollectedError {
	ce.mutex.Lock()
	defer ce.mutex.Unlock()
	var res []*CollectedError
	for _, e := range ce.errors {
		if e.Time.After(since) && e.GraphName == graphName {
			res = append(res, e)
		}
	}
	return res
}

package freepsstore

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/sirupsen/logrus"
)

// CollectedError keeps information about an error that occurred during graph execution
type CollectedError struct {
	Input     *base.OperatorIO
	Error     string
	GraphName string
	Operation *freepsgraph.GraphOperationDesc
}

// CollectedErrors keeps track of errors that occurred during graph execution
type CollectedErrors struct {
	ns           StoreNamespace
	maxLen       int
	errorCounter atomic.Uint64
}

// NewCollectedErrors creates a new CollectedErrors
func NewCollectedErrors(config *StoreConfig) *CollectedErrors {
	return &CollectedErrors{maxLen: config.MaxErrorLogSize, ns: store.GetNamespace(config.ErrorLogName)}
}

// AddError adds an error to the CollectedErrors
func (ce *CollectedErrors) AddError(input *base.OperatorIO, err *base.OperatorIO, ctx *base.Context, graphName string, od *freepsgraph.GraphOperationDesc) error {
	e := &CollectedError{Input: input, Error: err.GetString(), GraphName: graphName, Operation: od}
	id := ce.errorCounter.Add(1)
	storeErr := ce.ns.SetValue(fmt.Sprint(id), base.MakeObjectOutput(e), ctx.GetID())
	if storeErr != nil {
		return storeErr
	}
	storeErr = ce.ns.SetValue(fmt.Sprintf("%d-input", id), input, ctx.GetID())
	if storeErr != nil {
		return storeErr
	}

	if ce.ns.Len() > ce.maxLen*2 {
		ce.ns.DeleteValue(fmt.Sprint(id - uint64(ce.maxLen)))
	}
	return storeErr
}

// GetErrorsSince returns the error that occured in the given duration
func (ce *CollectedErrors) GetErrorsSince(d time.Duration) []*CollectedError {
	errors := ce.ns.GetAllFiltered("", "", "", 0, d)
	ret := make([]*CollectedError, 0, len(errors))
	for _, sob := range errors {
		var e CollectedError
		err := sob.ParseJSON(&e)
		if err != nil {
			logrus.Errorf("Error while parsing error log: %v", err)
			continue
		}
		ret = append(ret, &e)
	}

	return ret
}

// GetErrorsForGraph returns the error that occured ion the given duration for the given graph
func (ce *CollectedErrors) GetErrorsForGraph(d time.Duration, graphName string) []*CollectedError {
	errors := ce.ns.GetAllFiltered("", graphName, "", 0, d)
	ret := make([]*CollectedError, 0)
	for _, sob := range errors {
		var e CollectedError
		err := sob.ParseJSON(&e)
		if err != nil {
			logrus.Errorf("Error while parsing error log: %v", err)
			continue
		}
		if e.GraphName == graphName {
			ret = append(ret, &e)
		}
	}

	return ret
}

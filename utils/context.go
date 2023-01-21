package utils

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type OperationLog struct {
	GraphName         string
	OpDesc            string
	StartTime         time.Time
	ExecutionDuration time.Duration
	HTTPResponseCode  int
	NestingLevel      int
}

// MarshalJSON provides a custom marshaller with better readable time formats
func (o *OperationLog) MarshalJSON() ([]byte, error) {
	readable := struct {
		GraphName         string
		OpDesc            string
		StartTime         string
		ExecutionDuration string
		HTTPResponseCode  int
		NestingLevel      int
	}{
		GraphName:         o.GraphName,
		OpDesc:            o.OpDesc,
		StartTime:         o.StartTime.Format(time.RFC1123),
		ExecutionDuration: o.ExecutionDuration.String(),
		HTTPResponseCode:  o.HTTPResponseCode,
		NestingLevel:      o.NestingLevel,
	}

	return json.Marshal(readable)
}

// Context keeps the runtime data of a graph execution tree
type Context struct {
	UUID         uuid.UUID
	logger       log.FieldLogger
	Created      time.Time
	Finished     time.Time
	Operations   []OperationLog
	currentLevel int
}

// MarshalJSON provides a custom marshaller with better readable time formats
func (c *Context) MarshalJSON() ([]byte, error) {
	readable := struct {
		UUID       uuid.UUID
		Created    string
		Finished   string
		Operations []OperationLog
	}{
		UUID:       c.UUID,
		Created:    c.Created.Format(time.RFC1123),
		Finished:   c.Finished.Format(time.RFC1123),
		Operations: c.Operations,
	}

	return json.Marshal(readable)
}

// NewContext creates a Context with a given logger
func NewContext(logger log.FieldLogger) *Context {
	u := uuid.New()
	return &Context{UUID: u, logger: logger.WithField("uuid", u.String()), Created: time.Now(), Operations: make([]OperationLog, 0)}
}

// GetID returns the string represantation of the ID for this execution tree
func (c *Context) GetID() string {
	return c.UUID.String()
}

// GetLogger returns a Logger with the proper fields added to identify the context
func (c *Context) GetLogger() log.FieldLogger {
	return c.logger
}

func (c *Context) IncreaseNesting() {
	c.currentLevel++
}

func (c *Context) DecreaseNesting() {
	c.currentLevel--
	c.Finished = time.Now()
}

func (c *Context) IsRootContext() bool {
	return c.currentLevel == 0
}

// RecordFinisheOperation records a new entry in the execution log of this context
func (c *Context) RecordFinisheOperation(graphName string, opDesc string, startTime time.Time, responseCode int) {
	op := OperationLog{GraphName: graphName, OpDesc: opDesc, StartTime: startTime, HTTPResponseCode: responseCode, ExecutionDuration: time.Now().Sub(startTime), NestingLevel: c.currentLevel}
	c.Operations = append(c.Operations, op)
}

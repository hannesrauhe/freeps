package base

import (
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// Context keeps the runtime data of a graph execution tree
type Context struct {
	UUID   uuid.UUID
	logger log.FieldLogger
}

// NewContext creates a Context with a given logger
func NewContext(logger log.FieldLogger) *Context {
	u := uuid.New()
	return &Context{UUID: u, logger: logger.WithField("uuid", u.String())}
}

// GetID returns the string represantation of the ID for this execution tree
func (c *Context) GetID() string {
	return c.UUID.String()
}

// GetLogger returns a Logger with the proper fields added to identify the context
func (c *Context) GetLogger() log.FieldLogger {
	return c.logger
}

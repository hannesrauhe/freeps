package base

import (
	"context"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// Context keeps the runtime data of a graph execution tree
type Context struct {
	UUID      uuid.UUID
	Reason    string
	GoContext context.Context
	logger    log.FieldLogger
}

// NewContext creates a Context with a given logger
func NewContext(logger log.FieldLogger, reason string) *Context {
	u := uuid.New()
	return &Context{UUID: u, logger: logger.WithField("uuid", u.String()), Reason: reason, GoContext: context.TODO()}
}

// NewBaseContext creates a Context with a given logger
func NewBaseContext(logger log.FieldLogger) (*Context, context.CancelFunc) {
	u := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	return &Context{UUID: u, logger: logger, Reason: "base", GoContext: ctx}, cancel
}

func WithTimeout(parentContext *Context, timeout time.Duration) (*Context, context.CancelFunc) {
	goCtx, cancel := context.WithTimeout(parentContext.GoContext, timeout)
	ctx := &Context{UUID: parentContext.UUID, logger: parentContext.logger, Reason: parentContext.Reason, GoContext: goCtx}
	return ctx, cancel
}

func WithField(parentContext *Context, key string, value string) *Context {
	reason := key + ":" + value
	u := uuid.New()
	return &Context{UUID: u, logger: parentContext.logger.WithField(key, value).WithField("uuid", u.String()), Reason: reason, GoContext: parentContext.GoContext}
}

// GetID returns the string represantation of the ID for this execution tree
func (c *Context) GetID() string {
	return c.UUID.String()
}

// GetReason returns the reason for the creation of this context
func (c *Context) GetReason() string {
	return c.Reason
}

// GetLogger returns a Logger with the proper fields added to identify the context
func (c *Context) GetLogger() log.FieldLogger {
	return c.logger
}

func (c *Context) Done() <-chan struct{} {
	return c.GoContext.Done()
}

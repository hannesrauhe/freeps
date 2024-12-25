package base

import (
	"context"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// Context keeps the runtime data of a flow execution tree
type Context struct {
	UUID       uuid.UUID
	Reason     string
	GoContext  context.Context
	logger     log.FieldLogger
	baseLogger *log.Logger
}

// NewBaseContextWithReason creates a Context with a given logger
// Deprecated: use NewBaseContext instead
func NewBaseContextWithReason(logger *log.Logger, reason string) *Context {
	u := uuid.New()
	return &Context{UUID: u, logger: logger.WithField("uuid", u.String()), Reason: reason, GoContext: context.TODO(), baseLogger: logger}
}

// NewBaseContext creates a Context with a given logger
func NewBaseContext(logger *log.Logger) (*Context, context.CancelFunc) {
	u := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	return &Context{UUID: u, logger: logger, Reason: "base", GoContext: ctx, baseLogger: logger}, cancel
}

func CreateContextWithField(baseContext *Context, key string, value string, reason string) *Context {
	u := uuid.New()
	logger := baseContext.logger.WithField(key, value).WithField("uuid", u.String())
	logger.Debugf("Creating new context with reason: %s", reason)
	return &Context{UUID: u, logger: logger, Reason: reason, GoContext: baseContext.GoContext, baseLogger: baseContext.baseLogger}
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

func (c *Context) ChildContextWithField(key string, value string) *Context {
	return &Context{UUID: c.UUID, logger: c.logger.WithField(key, value), Reason: c.Reason, GoContext: c.GoContext, baseLogger: c.baseLogger}
}

func (c *Context) ChildContextWithTimeout(timeout time.Duration) (*Context, context.CancelFunc) {
	goCtx, cancel := context.WithTimeout(c.GoContext, timeout)
	ctx := &Context{UUID: c.UUID, logger: c.logger, Reason: c.Reason, GoContext: goCtx, baseLogger: c.baseLogger}
	return ctx, cancel
}

func (c *Context) EnableDebugLogging() log.Level {
	prevLevel := c.baseLogger.GetLevel()
	if prevLevel != log.DebugLevel {
		c.baseLogger.SetLevel(log.DebugLevel)
		c.logger.Debug("Enabling debug logging")
	}

	return prevLevel
}

func (c *Context) DisableDebugLogging(prevLevel log.Level) {
	if prevLevel != log.DebugLevel {
		// ensure that we do not accidentally enable debug logging which might happen if a second context calls EnableDebugLogging
		c.logger.Debug("Disabling debug logging")
		c.baseLogger.SetLevel(prevLevel)
	}
}
